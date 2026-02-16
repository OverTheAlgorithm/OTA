package auth

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"ota/internal/user"
)

const cookieName = "ota_token"

type Handler struct {
	kakao      *KakaoClient
	jwt        *JWTManager
	states     *StateStore
	userRepo   user.Repository
	frontendURL string
}

func NewHandler(kakao *KakaoClient, jwt *JWTManager, states *StateStore, userRepo user.Repository, frontendURL string) *Handler {
	return &Handler{
		kakao:       kakao,
		jwt:         jwt,
		states:      states,
		userRepo:    userRepo,
		frontendURL: frontendURL,
	}
}

func (h *Handler) KakaoLogin(c *gin.Context) {
	state, err := h.states.Generate()
	if err != nil {
		log.Printf("failed to generate state: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	authURL := h.kakao.AuthorizationURL(state)
	c.Redirect(http.StatusFound, authURL)
}

func (h *Handler) KakaoCallback(c *gin.Context) {
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
	c.Redirect(http.StatusFound, h.frontendURL)
}

func (h *Handler) Me(c *gin.Context) {
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

func (h *Handler) Logout(c *gin.Context) {
	c.SetCookie(cookieName, "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}
