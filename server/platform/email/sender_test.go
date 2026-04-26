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

func TestBuildMIMEMessage_EncodesNonASCIISubject(t *testing.T) {
	// RFC 2047: non-ASCII bytes in headers must be encoded.
	// Samsung Email on Galaxy interprets unencoded 8-bit Subject bytes with
	// a default charset (not UTF-8), producing mojibake. Gmail/iPhone are
	// lenient and decode raw UTF-8, which is why the bug only appeared on
	// Galaxy Samsung Email.
	subject := "[🔥오늘의 필수 소식 5가지]: 테스트"
	result := buildMIMEMessage("sender@example.com", "to@example.com", subject, "text", "<p>html</p>")

	if strings.Contains(result, "Subject: "+subject) {
		t.Error("Subject header contains raw non-ASCII bytes; must be RFC 2047 encoded")
	}
	if !strings.Contains(result, "=?utf-8?") && !strings.Contains(result, "=?UTF-8?") {
		t.Errorf("Subject header missing RFC 2047 encoded-word marker. Got:\n%s", result)
	}
}

func TestBuildMIMEMessage_LeavesASCIISubjectReadable(t *testing.T) {
	// ASCII-only subjects should stay unencoded for readability in raw MIME.
	subject := "Plain ASCII Subject"
	result := buildMIMEMessage("sender@example.com", "to@example.com", subject, "text", "<p>html</p>")

	if !strings.Contains(result, "Subject: "+subject+"\r\n") {
		t.Errorf("ASCII subject should pass through unchanged. Got:\n%s", result)
	}
}

// Note: We don't test actual SMTP sending in unit tests
// Integration tests with real SMTP will be in integration package
