package handler

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ota/auth"
	"ota/cache"
	"ota/domain/delivery"
	"ota/domain/terms"
	"ota/domain/user"
	"ota/platform/kakao"
)

const cookieName = "ota_token"

// signupCacheTTL is the duration a pending signup entry lives in cache.
const signupCacheTTL = 3 * time.Minute

// PendingSignup holds Kakao profile data while the user reviews terms.
type PendingSignup struct {
	KakaoID         int64
	Email           string
	Nickname        string
	ProfileImageURL string
}

// SignupBonusGranter grants bonus coins to newly registered users.
type SignupBonusGranter interface {
	SetCoins(ctx context.Context, userID string, coins int) error
	InsertCoinEvent(ctx context.Context, userID string, amount int, eventType, memo, actorID string) error
}

// WithdrawalChecker checks for pending withdrawals before account deletion.
type WithdrawalChecker interface {
	HasPendingWithdrawals(ctx context.Context, userID string) (bool, error)
}

type AuthHandler struct {
	kakao              *kakao.Client
	jwt                *auth.JWTManager
	states             *auth.StateStore
	userRepo           user.Repository
	welcomeDeliverer   delivery.WelcomeDeliverer
	bonusGranter       SignupBonusGranter
	signupBonus        int
	frontendURL        string
	signupCache        cache.Cache
	termsService       *terms.Service
	withdrawalChecker  WithdrawalChecker
}

func NewAuthHandler(
	kakao *kakao.Client,
	jwt *auth.JWTManager,
	states *auth.StateStore,
	userRepo user.Repository,
	welcomeDeliverer delivery.WelcomeDeliverer,
	bonusGranter SignupBonusGranter,
	signupBonus int,
	frontendURL string,
	signupCache cache.Cache,
	termsService *terms.Service,
) *AuthHandler {
	return &AuthHandler{
		kakao:            kakao,
		jwt:              jwt,
		states:           states,
		userRepo:         userRepo,
		welcomeDeliverer: welcomeDeliverer,
		bonusGranter:     bonusGranter,
		signupBonus:      signupBonus,
		frontendURL:      frontendURL,
		signupCache:      signupCache,
		termsService:     termsService,
	}
}

func (h *AuthHandler) WithWithdrawalChecker(wc WithdrawalChecker) *AuthHandler {
	h.withdrawalChecker = wc
	return h
}

func (h *AuthHandler) KakaoLogin(c *gin.Context) {
	state, err := h.states.Generate()
	if err != nil {
		log.Printf("failed to generate state: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	authURL := h.kakao.AuthorizationURL(state)
	c.Redirect(http.StatusFound, authURL)
}

func (h *AuthHandler) KakaoCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		c.Redirect(http.StatusFound, h.frontendURL+"/login?error=invalid_request")
		return
	}

	if !h.states.Validate(state) {
		c.Redirect(http.StatusFound, h.frontendURL+"/login?error=invalid_state")
		return
	}

	tokenResp, err := h.kakao.ExchangeCode(c.Request.Context(), code)
	if err != nil {
		log.Printf("failed to exchange code: %v", err)
		c.Redirect(http.StatusFound, h.frontendURL+"/login?error=token_exchange_failed")
		return
	}

	kakaoUser, err := h.kakao.FetchUser(c.Request.Context(), tokenResp.AccessToken)
	if err != nil {
		log.Printf("failed to fetch kakao user: %v", err)
		c.Redirect(http.StatusFound, h.frontendURL+"/login?error=user_fetch_failed")
		return
	}

	// Check if user already exists
	_, found, err := h.userRepo.FindByKakaoID(c.Request.Context(), kakaoUser.ID)
	if err != nil {
		log.Printf("failed to check existing user: %v", err)
		c.Redirect(http.StatusFound, h.frontendURL+"/login?error=db_error")
		return
	}

	if found {
		// Returning user — update profile and login directly
		u, err := h.userRepo.UpsertByKakaoID(
			c.Request.Context(),
			kakaoUser.ID,
			kakaoUser.Account.Email,
			kakaoUser.Account.Profile.Nickname,
			kakaoUser.Account.Profile.ProfileImageURL,
		)
		if err != nil {
			log.Printf("failed to upsert user: %v", err)
			c.Redirect(http.StatusFound, h.frontendURL+"/login?error=db_error")
			return
		}

		jwtToken, err := h.jwt.Generate(u.ID, u.Role)
		if err != nil {
			log.Printf("failed to generate JWT: %v", err)
			c.Redirect(http.StatusFound, h.frontendURL+"/login?error=jwt_error")
			return
		}

		http.SetCookie(c.Writer, &http.Cookie{
			Name:     cookieName,
			Value:    jwtToken,
			MaxAge:   7 * 24 * 3600,
			Path:     "/",
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
		})

		c.Redirect(http.StatusFound, h.frontendURL+"/home")
		return
	}

	// New user — cache Kakao data and redirect to terms consent
	signupKey := uuid.New().String()
	h.signupCache.Set(signupKey, PendingSignup{
		KakaoID:         kakaoUser.ID,
		Email:           kakaoUser.Account.Email,
		Nickname:        kakaoUser.Account.Profile.Nickname,
		ProfileImageURL: kakaoUser.Account.Profile.ProfileImageURL,
	}, signupCacheTTL)

	log.Printf("new user signup pending — kakao_id=%d, signup_key=%s", kakaoUser.ID, signupKey)
	c.Redirect(http.StatusFound, h.frontendURL+"/terms-consent?signup_key="+signupKey)
}

type completeSignupRequest struct {
	SignupKey      string   `json:"signup_key"`
	AgreedTermIDs []string `json:"agreed_term_ids"`
}

// CompleteSignup finishes the two-phase signup after terms consent.
func (h *AuthHandler) CompleteSignup(c *gin.Context) {
	var req completeSignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 요청 형식입니다"})
		return
	}

	if req.SignupKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "signup_key는 필수입니다"})
		return
	}

	// Retrieve and immediately bust the cache entry
	cached, found := h.signupCache.Get(req.SignupKey)
	h.signupCache.Delete(req.SignupKey)

	if !found {
		c.JSON(http.StatusBadRequest, gin.H{"error": "세션이 만료되었습니다. 다시 로그인해주세요."})
		return
	}

	pending, ok := cached.(PendingSignup)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "내부 오류가 발생했습니다"})
		return
	}

	// Validate terms consent
	if err := h.termsService.ValidateConsents(c.Request.Context(), req.AgreedTermIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create user
	u, err := h.userRepo.UpsertByKakaoID(
		c.Request.Context(),
		pending.KakaoID,
		pending.Email,
		pending.Nickname,
		pending.ProfileImageURL,
	)
	if err != nil {
		log.Printf("failed to create user during signup: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "회원가입에 실패했습니다"})
		return
	}

	// Save term consents
	if err := h.termsService.SaveConsents(c.Request.Context(), u.ID, req.AgreedTermIDs); err != nil {
		log.Printf("failed to save consents for user %s: %v", u.ID, err)
	}

	// Grant signup bonus
	if h.signupBonus > 0 && h.bonusGranter != nil {
		if err := h.bonusGranter.SetCoins(c.Request.Context(), u.ID, h.signupBonus); err != nil {
			log.Printf("signup bonus grant failed for user %s: %v", u.ID, err)
		} else {
			_ = h.bonusGranter.InsertCoinEvent(c.Request.Context(), u.ID, h.signupBonus, "signup_bonus", "회원가입 보너스", "")
			log.Printf("granted %d signup bonus coins to new user %s", h.signupBonus, u.ID)
		}
	}

	// Send welcome delivery
	if u.Email != "" && h.welcomeDeliverer != nil {
		go func() {
			if err := h.welcomeDeliverer.DeliverToNewUser(context.Background(), u.ID, u.Email); err != nil {
				log.Printf("welcome delivery failed for user %s: %v", u.ID, err)
			} else {
				log.Printf("welcome delivery sent to new user %s (%s)", u.ID, u.Email)
			}
		}()
	}

	// Generate JWT and set cookie
	jwtToken, err := h.jwt.Generate(u.ID, u.Role)
	if err != nil {
		log.Printf("failed to generate JWT: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "로그인 처리에 실패했습니다"})
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cookieName,
		Value:    jwtToken,
		MaxAge:   7 * 24 * 3600,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
	})

	log.Printf("new user signup completed — user_id=%s, kakao_id=%d, consents=%d", u.ID, pending.KakaoID, len(req.AgreedTermIDs))
	c.JSON(http.StatusOK, gin.H{"data": u})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	u, err := h.userRepo.FindByID(c.Request.Context(), userID.(string))
	if err != nil {
		log.Printf("failed to find user: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": u})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
	})
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	uid := userID.(string)

	// Block deletion if there are pending withdrawals
	if h.withdrawalChecker != nil {
		hasPending, err := h.withdrawalChecker.HasPendingWithdrawals(c.Request.Context(), uid)
		if err != nil {
			log.Printf("failed to check pending withdrawals for user %s: %v", uid, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "계정 삭제 중 오류가 발생했습니다"})
			return
		}
		if hasPending {
			c.JSON(http.StatusConflict, gin.H{"error": "처리 대기 중인 출금 신청이 있습니다. 출금 완료 또는 취소 후 탈퇴해주세요."})
			return
		}
	}

	if err := h.userRepo.DeleteByID(c.Request.Context(), uid); err != nil {
		log.Printf("failed to delete user %s: %v", uid, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "계정 삭제에 실패했습니다"})
		return
	}

	// Clear auth cookie
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
	})

	log.Printf("user account deleted — user_id=%s", uid)
	c.JSON(http.StatusOK, gin.H{"message": "계정이 삭제되었습니다"})
}

func (h *AuthHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/kakao/login", h.KakaoLogin)
	group.GET("/kakao/callback", h.KakaoCallback)
	group.POST("/complete-signup", h.CompleteSignup)
	group.POST("/logout", h.Logout)

	protected := group.Group("")
	protected.Use(h.authMiddleware())
	{
		protected.GET("/me", h.Me)
		protected.DELETE("/delete-account", h.DeleteAccount)
	}
}

func (h *AuthHandler) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := c.Cookie(cookieName)
		if err != nil || tokenStr == "" {
			header := c.GetHeader("Authorization")
			if len(header) > 7 && header[:7] == "Bearer " {
				tokenStr = header[7:]
			}
		}

		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		claims, err := h.jwt.Validate(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("role", claims.Role)
		c.Next()
	}
}
