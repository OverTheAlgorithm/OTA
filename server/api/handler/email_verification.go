package handler

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	emailPlatform "ota/platform/email"
	"ota/domain/user"
)

type EmailVerificationHandler struct {
	service     *user.EmailVerificationService
	emailSender emailPlatform.Sender
}

func NewEmailVerificationHandler(
	service *user.EmailVerificationService,
	emailSender emailPlatform.Sender,
) *EmailVerificationHandler {
	return &EmailVerificationHandler{
		service:     service,
		emailSender: emailSender,
	}
}

func (h *EmailVerificationHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/send-code", h.SendCode)
	group.POST("/verify-code", h.VerifyCode)
}

type sendCodeRequest struct {
	Email string `json:"email" binding:"required"`
}

// POST /api/v1/email-verification/send-code
func (h *EmailVerificationHandler) SendCode(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req sendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "이메일 주소를 입력해주세요"})
		return
	}

	result, err := h.service.SendCode(c.Request.Context(), userID, req.Email)
	if err != nil {
		// Distinguish user-facing errors
		switch {
		case isRateLimitError(err):
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "인증 코드 요청이 너무 많습니다. 잠시 후 다시 시도해주세요.",
			})
		case isValidationError(err):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "올바른 이메일 형식이 아닙니다.",
			})
		default:
			log.Printf("send verification code failed for user %s: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "인증 코드 전송에 실패했습니다. 다시 시도해주세요.",
			})
		}
		return
	}

	// Send the verification email
	subject := "[OTA] 이메일 인증 코드"
	textBody := fmt.Sprintf("OTA 이메일 인증 코드: %s\n\n이 코드는 5분간 유효합니다.", result.Code)
	htmlBody := fmt.Sprintf(`
		<div style="font-family: sans-serif; max-width: 480px; margin: 0 auto; padding: 24px;">
			<h2 style="color: #333;">OTA 이메일 인증</h2>
			<p>아래 인증 코드를 입력해주세요:</p>
			<div style="background: #f5f5f5; padding: 16px; text-align: center; border-radius: 8px; margin: 16px 0;">
				<span style="font-size: 32px; font-weight: bold; letter-spacing: 8px; color: #333;">%s</span>
			</div>
			<p style="color: #666; font-size: 14px;">이 코드는 5분간 유효합니다.</p>
			<p style="color: #999; font-size: 12px;">본인이 요청하지 않은 경우 이 메일을 무시하셔도 됩니다.</p>
		</div>
	`, result.Code)

	if err := h.emailSender.Send(result.Email, subject, textBody, htmlBody); err != nil {
		log.Printf("failed to send verification email to %s: %v", result.Email, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "이메일 전송에 실패했습니다. 이메일 주소를 확인해주세요.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "인증 코드가 전송되었습니다.",
	})
}

type verifyCodeRequest struct {
	Code string `json:"code" binding:"required"`
}

// POST /api/v1/email-verification/verify-code
func (h *EmailVerificationHandler) VerifyCode(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req verifyCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "인증 코드를 입력해주세요"})
		return
	}

	// Validate code format (must be exactly 6 digits)
	if len(req.Code) != 6 || !isNumeric(req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "올바른 인증 코드 형식이 아닙니다."})
		return
	}

	if err := h.service.VerifyCode(c.Request.Context(), userID, req.Code); err != nil {
		log.Printf("verify code failed for user %s: %v", userID, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "인증 코드가 올바르지 않거나 만료되었습니다.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "이메일이 성공적으로 인증되었습니다.",
	})
}

// Helper functions for error classification
func isRateLimitError(err error) bool {
	return strings.Contains(err.Error(), "rate limit exceeded")
}

func isValidationError(err error) bool {
	return strings.Contains(err.Error(), "invalid email format")
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
