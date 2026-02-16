package server

import (
	"github.com/gin-gonic/gin"
	"ota/internal/auth"
)

func New(authHandler *auth.Handler, jwtManager *auth.JWTManager, frontendURL string) *gin.Engine {
	r := gin.Default()

	r.Use(CORSMiddleware(frontendURL))

	api := r.Group("/api/v1")
	{
		authGroup := api.Group("/auth")
		{
			authGroup.GET("/kakao/login", authHandler.KakaoLogin)
			authGroup.GET("/kakao/callback", authHandler.KakaoCallback)
			authGroup.POST("/logout", authHandler.Logout)

			protected := authGroup.Group("")
			protected.Use(AuthMiddleware(jwtManager))
			{
				protected.GET("/me", authHandler.Me)
			}
		}
	}

	return r
}
