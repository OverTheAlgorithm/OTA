package handler

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"ota/auth"
	"ota/domain/delivery"
	"ota/domain/user"
	"ota/platform/kakao"
)

const cookieName = "ota_token"

type AuthHandler struct {
	kakao            *kakao.Client
	jwt              *auth.JWTManager
	states           *auth.StateStore
	userRepo         user.Repository
	welcomeDeliverer delivery.WelcomeDeliverer
	frontendURL      string
}

func NewAuthHandler(kakao *kakao.Client, jwt *auth.JWTManager, states *auth.StateStore, userRepo user.Repository, welcomeDeliverer delivery.WelcomeDeliverer, frontendURL string) *AuthHandler {
	return &AuthHandler{
		kakao:            kakao,
		jwt:              jwt,
		states:           states,
		userRepo:         userRepo,
		welcomeDeliverer: welcomeDeliverer,
		frontendURL:      frontendURL,
	}
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

	jwtToken, err := h.jwt.Generate(u.ID)
	if err != nil {
		log.Printf("failed to generate JWT: %v", err)
		c.Redirect(http.StatusFound, h.frontendURL+"/login?error=jwt_error")
		return
	}

	c.SetCookie(cookieName, jwtToken, 7*24*3600, "/", "", false, true)

	// Send welcome delivery to newly registered users
	isNewUser := u.CreatedAt.Equal(u.UpdatedAt)
	if isNewUser && u.Email != "" && h.welcomeDeliverer != nil {
		go func() {
			if err := h.welcomeDeliverer.DeliverToNewUser(context.Background(), u.ID, u.Email); err != nil {
				log.Printf("welcome delivery failed for user %s: %v", u.ID, err)
			} else {
				log.Printf("welcome delivery sent to new user %s (%s)", u.ID, u.Email)
			}
		}()
	}

	c.Redirect(http.StatusFound, h.frontendURL+"/home")
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
	c.SetCookie(cookieName, "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (h *AuthHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/kakao/login", h.KakaoLogin)
	group.GET("/kakao/callback", h.KakaoCallback)
	group.POST("/logout", h.Logout)

	protected := group.Group("")
	protected.Use(h.authMiddleware())
	{
		protected.GET("/me", h.Me)
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

		userID, err := h.jwt.Validate(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}
