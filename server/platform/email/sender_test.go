package email

import (
	"strings"
	"testing"
)

func TestBuildMIMEMessage(t *testing.T) {
	from := "sender@example.com"
	to := "recipient@example.com"
	subject := "Test Subject"
	textBody := "Plain text content"
	htmlBody := "<p>HTML content</p>"

	result := buildMIMEMessage(from, to, subject, textBody, htmlBody)

	// Verify headers
	if !strings.Contains(result, "From: "+from) {
		t.Error("expected From header")
	}

	if !strings.Contains(result, "To: "+to) {
		t.Error("expected To header")
	}

	if !strings.Contains(result, "Subject: "+subject) {
		t.Error("expected Subject header")
	}

	// Verify MIME structure
	if !strings.Contains(result, "MIME-Version: 1.0") {
		t.Error("expected MIME-Version header")
	}

	if !strings.Contains(result, "Content-Type: multipart/alternative") {
		t.Error("expected multipart/alternative content type")
	}

	// Verify both parts exist
	if !strings.Contains(result, "Content-Type: text/plain") {
		t.Error("expected text/plain part")
	}

	if !strings.Contains(result, "Content-Type: text/html") {
		t.Error("expected text/html part")
	}

	if !strings.Contains(result, textBody) {
		t.Error("expected text body content")
	}

	if !strings.Contains(result, htmlBody) {
		t.Error("expected HTML body content")
	}
}

func TestSMTPSender_Configuration(t *testing.T) {
	config := SMTPConfig{
		Host:     "smtp.example.com",
		Port:     587,
		Username: "user@example.com",
		Password: "password",
		From:     "sender@example.com",
	}

	sender := NewSMTPSender(config)

	if sender.config.Host != "smtp.example.com" {
		t.Errorf("expected Host 'smtp.example.com', got '%s'", sender.config.Host)
	}

	if sender.config.Port != 587 {
		t.Errorf("expected Port 587, got %d", sender.config.Port)
	}
}

// Note: We don't test actual SMTP sending in unit tests
// Integration tests with real SMTP will be in integration package
