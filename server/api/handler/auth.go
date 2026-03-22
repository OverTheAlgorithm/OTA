package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
	"ota/storage"
)

const cookieName = "ota_token"
const refreshCookieName = "ota_refresh"

// accessTokenMaxAge is the MaxAge for the access token cookie (15 min).
const accessTokenMaxAge = 15 * 60

// refreshTokenMaxAge is the MaxAge for the refresh token cookie (7 days).
const refreshTokenMaxAge = 7 * 24 * 3600

// signupCacheTTL is the duration a pending signup entry lives in cache.
const signupCacheTTL = 3 * time.Minute

// PendingSignup holds Kakao profile data while the user reviews terms.
type PendingSignup struct {
	KakaoID         int64  `json:"kakao_id"`
	Email           string `json:"email"`
	Nickname        string `json:"nickname"`
	ProfileImageURL string `json:"profile_image_url"`
}

// decodePendingSignup extracts a PendingSignup from a cache value.
// Supports both direct struct (in-process OtterCache) and map[string]any
// (Redis JSON-decoded). Uses a JSON round-trip for the latter case.
func decodePendingSignup(raw any) (PendingSignup, bool) {
	if p, ok := raw.(PendingSignup); ok {
		return p, true
	}
	var b []byte
	switch data := raw.(type) {
	case []byte:
		b = data
	case string:
		b = []byte(data)
	default:
		var err error
		b, err = json.Marshal(raw)
		if err != nil {
			return PendingSignup{}, false
		}
	}
	var p PendingSignup
	if err := json.Unmarshal(b, &p); err != nil {
		return PendingSignup{}, false
	}
	return p, true
}

// SignupBonusGranter grants bonus coins to newly registered users.
type SignupBonusGranter interface {
	AddPoints(ctx context.Context, userID string, amount int) error
	InsertCoinEvent(ctx context.Context, userID string, amount int, eventType, memo, actorID string) error
}

// WithdrawalChecker checks for pending withdrawals before account deletion.
type WithdrawalChecker interface {
	HasPendingWithdrawals(ctx context.Context, userID string) (bool, error)
}

// RefreshTokenStore persists and validates opaque refresh tokens.
type RefreshTokenStore interface {
	Insert(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	FindByHash(ctx context.Context, tokenHash string) (*storage.RefreshToken, bool, error)
	DeleteByHash(ctx context.Context, tokenHash string) error
	DeleteAllForUser(ctx context.Context, userID string) error
}

type AuthHandler struct {
	kakao             *kakao.Client
	jwt               *auth.JWTManager
	states            auth.StateStorer
	userRepo          user.Repository
	welcomeDeliverer  delivery.WelcomeDeliverer
	bonusGranter      SignupBonusGranter
	signupBonus       int
	frontendURL       string
	signupCache       cache.Cache
	termsService      *terms.Service
	withdrawalChecker WithdrawalChecker
	refreshTokenStore RefreshTokenStore
}

func NewAuthHandler(
	kakao *kakao.Client,
	jwt *auth.JWTManager,
	states auth.StateStorer,
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

func (h *AuthHandler) WithRefreshTokenStore(store RefreshTokenStore) *AuthHandler {
	h.refreshTokenStore = store
	return h
}

// setAuthCookies issues both the short-lived access token cookie and the
// long-lived refresh token cookie. The refresh cookie is scoped to
// /api/v1/auth to limit its exposure.
func (h *AuthHandler) setAuthCookies(w http.ResponseWriter, userID, role string) error {
	accessToken, err := h.jwt.Generate(userID, role)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    accessToken,
		MaxAge:   accessTokenMaxAge,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
	})

	if h.refreshTokenStore == nil {
		return nil
	}

	rawRefresh, hashRefresh, err := auth.GenerateRefreshToken()
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(auth.RefreshTokenExpiry)
	if err := h.refreshTokenStore.Insert(context.Background(), userID, hashRefresh, expiresAt); err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    rawRefresh,
		MaxAge:   refreshTokenMaxAge,
		Path:     "/api/v1/auth",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
	})

	return nil
}

// clearAuthCookies expires both auth cookies.
func clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		MaxAge:   -1,
		Path:     "/api/v1/auth",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
	})
}

func (h *AuthHandler) KakaoLogin(c *gin.Context) {
	state, err := h.states.Generate()
	if err != nil {
		slog.Error("failed to generate state", "error", err)
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
		slog.Error("failed to exchange code", "error", err)
		c.Redirect(http.StatusFound, h.frontendURL+"/login?error=token_exchange_failed")
		return
	}

	kakaoUser, err := h.kakao.FetchUser(c.Request.Context(), tokenResp.AccessToken)
	if err != nil {
		slog.Error("failed to fetch kakao user", "error", err)
		c.Redirect(http.StatusFound, h.frontendURL+"/login?error=user_fetch_failed")
		return
	}

	_, found, err := h.userRepo.FindByKakaoID(c.Request.Context(), kakaoUser.ID)
	if err != nil {
		slog.Error("failed to check existing user", "error", err)
		c.Redirect(http.StatusFound, h.frontendURL+"/login?error=db_error")
		return
	}

	if found {
		u, err := h.userRepo.UpsertByKakaoID(
			c.Request.Context(),
			kakaoUser.ID,
			kakaoUser.Account.Email,
			kakaoUser.Account.Profile.Nickname,
			kakaoUser.Account.Profile.ProfileImageURL,
		)
		if err != nil {
			slog.Error("failed to upsert user", "error", err)
			c.Redirect(http.StatusFound, h.frontendURL+"/login?error=db_error")
			return
		}

		if err := h.setAuthCookies(c.Writer, u.ID, u.Role); err != nil {
			slog.Error("failed to set auth cookies", "error", err)
			c.Redirect(http.StatusFound, h.frontendURL+"/login?error=jwt_error")
			return
		}

		c.Redirect(http.StatusFound, h.frontendURL+"/")
		return
	}

	signupKey := uuid.New().String()
	h.signupCache.Set(signupKey, PendingSignup{
		KakaoID:         kakaoUser.ID,
		Email:           kakaoUser.Account.Email,
		Nickname:        kakaoUser.Account.Profile.Nickname,
		ProfileImageURL: kakaoUser.Account.Profile.ProfileImageURL,
	}, signupCacheTTL)

	slog.Info("new user signup pending", "kakao_id", kakaoUser.ID)
	c.Redirect(http.StatusFound, h.frontendURL+"/terms-consent?signup_key="+signupKey)
}

type completeSignupRequest struct {
	SignupKey      string   `json:"signup_key"`
	AgreedTermIDs []string `json:"agreed_term_ids"`
}

func (h *AuthHandler) CompleteSignup(c *gin.Context) {
	var req completeSignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.SignupKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "signup_key required"})
		return
	}

	cached, found := h.signupCache.Get(req.SignupKey)
	h.signupCache.Delete(req.SignupKey)

	if !found {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session expired"})
		return
	}

	pending, ok := decodePendingSignup(cached)
	if !ok {
		slog.Error("[CompleteSignup] failed to decode pending signup from cache",
			"signup_key", req.SignupKey,
			"cached_type", fmt.Sprintf("%T", cached),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.termsService.ValidateConsents(c.Request.Context(), req.AgreedTermIDs); err != nil {
		slog.Warn("[CompleteSignup] terms validation failed",
			"kakao_id", pending.KakaoID,
			"agreed_term_ids", req.AgreedTermIDs,
			"error", err,
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, err := h.userRepo.UpsertByKakaoID(
		c.Request.Context(),
		pending.KakaoID,
		pending.Email,
		pending.Nickname,
		pending.ProfileImageURL,
	)
	if err != nil {
		slog.Error("[CompleteSignup] failed to create user",
			"kakao_id", pending.KakaoID,
			"email", pending.Email,
			"nickname", pending.Nickname,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "signup failed"})
		return
	}

	if err := h.termsService.SaveConsents(c.Request.Context(), u.ID, req.AgreedTermIDs); err != nil {
		slog.Error("[CompleteSignup] failed to save consents — aborting signup",
			"user_id", u.ID,
			"agreed_term_ids", req.AgreedTermIDs,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "signup failed"})
		return
	}

	if h.signupBonus > 0 && h.bonusGranter != nil {
		if err := h.bonusGranter.AddPoints(c.Request.Context(), u.ID, h.signupBonus); err != nil {
			slog.Warn("[CompleteSignup] signup bonus grant failed", "user_id", u.ID, "error", err)
		} else {
			_ = h.bonusGranter.InsertCoinEvent(c.Request.Context(), u.ID, h.signupBonus, "signup_bonus", "가입 보너스", "")
			slog.Info("[CompleteSignup] signup bonus granted", "coins", h.signupBonus, "user_id", u.ID)
		}
	}

	if u.Email != "" && h.welcomeDeliverer != nil {
		go func() {
			if err := h.welcomeDeliverer.DeliverToNewUser(context.Background(), u.ID, u.Email); err != nil {
				slog.Warn("[CompleteSignup] welcome delivery failed", "user_id", u.ID, "error", err)
			} else {
				slog.Info("[CompleteSignup] welcome delivery sent", "user_id", u.ID)
			}
		}()
	}

	if err := h.setAuthCookies(c.Writer, u.ID, u.Role); err != nil {
		slog.Error("[CompleteSignup] failed to set auth cookies",
			"user_id", u.ID,
			"role", u.Role,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}

	slog.Info("[CompleteSignup] new user signup completed", "user_id", u.ID, "kakao_id", pending.KakaoID, "consents", len(req.AgreedTermIDs))
	c.JSON(http.StatusOK, gin.H{"data": u})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	u, err := h.userRepo.FindByID(c.Request.Context(), userID)
	if err != nil {
		slog.Error("failed to find user", "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": u})
}

// Refresh validates the refresh token cookie, rotates it, and issues a new pair.
func (h *AuthHandler) Refresh(c *gin.Context) {
	if h.refreshTokenStore == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "refresh not configured"})
		return
	}

	rawToken, err := c.Cookie(refreshCookieName)
	if err != nil || rawToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing refresh token"})
		return
	}

	tokenHash := auth.HashRefreshToken(rawToken)
	record, found, err := h.refreshTokenStore.FindByHash(c.Request.Context(), tokenHash)
	if err != nil {
		slog.Error("refresh token lookup error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if !found {
		clearAuthCookies(c.Writer)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
		return
	}

	if err := h.refreshTokenStore.DeleteByHash(c.Request.Context(), tokenHash); err != nil {
		slog.Error("failed to delete old refresh token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	u, err := h.userRepo.FindByID(c.Request.Context(), record.UserID)
	if err != nil {
		slog.Error("refresh: user not found", "user_id", record.UserID, "error", err)
		clearAuthCookies(c.Writer)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	if err := h.setAuthCookies(c.Writer, u.ID, u.Role); err != nil {
		slog.Error("failed to set auth cookies during refresh", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "token refreshed"})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if h.refreshTokenStore != nil {
		if rawToken, err := c.Cookie(refreshCookieName); err == nil && rawToken != "" {
			tokenHash := auth.HashRefreshToken(rawToken)
			if err := h.refreshTokenStore.DeleteByHash(c.Request.Context(), tokenHash); err != nil {
				slog.Warn("failed to revoke refresh token on logout", "error", err)
			}
		}
	}

	clearAuthCookies(c.Writer)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	uid := c.GetString("userID")
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if h.withdrawalChecker != nil {
		hasPending, err := h.withdrawalChecker.HasPendingWithdrawals(c.Request.Context(), uid)
		if err != nil {
			slog.Error("failed to check pending withdrawals", "user_id", uid, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error during account deletion"})
			return
		}
		if hasPending {
			c.JSON(http.StatusConflict, gin.H{"error": "pending withdrawal exists"})
			return
		}
	}

	if h.refreshTokenStore != nil {
		if err := h.refreshTokenStore.DeleteAllForUser(c.Request.Context(), uid); err != nil {
			slog.Warn("failed to revoke refresh tokens", "user_id", uid, "error", err)
		}
	}

	if err := h.userRepo.DeleteByID(c.Request.Context(), uid); err != nil {
		slog.Error("failed to delete user", "user_id", uid, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "account deletion failed"})
		return
	}

	clearAuthCookies(c.Writer)
	slog.Info("user account deleted", "user_id", uid)
	c.JSON(http.StatusOK, gin.H{"message": "account deleted"})
}

func (h *AuthHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/kakao/login", h.KakaoLogin)
	group.GET("/kakao/callback", h.KakaoCallback)
	group.POST("/complete-signup", h.CompleteSignup)
	group.POST("/logout", h.Logout)
	group.POST("/refresh", h.Refresh)

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
