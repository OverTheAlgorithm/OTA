package email

import (
	"fmt"
	"mime"
	"net/mail"
	"net/smtp"
	"strings"
)

// Sender defines the interface for sending emails
// This abstraction allows easy mocking and testing
type Sender interface {
	Send(to string, subject string, textBody string, htmlBody string) error
}

// SMTPConfig holds SMTP server configuration
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// SMTPSender implements Sender using SMTP
type SMTPSender struct {
	config SMTPConfig
}

// NewSMTPSender creates a new SMTP-based email sender
func NewSMTPSender(config SMTPConfig) *SMTPSender {
	return &SMTPSender{
		config: config,
	}
}

// Send sends an email via SMTP
func (s *SMTPSender) Send(to string, subject string, textBody string, htmlBody string) error {
	// Parse From field: supports both "Name <email>" and plain "email" formats.
	// The display name is RFC 2047-encoded to handle non-ASCII characters (e.g. Korean).
	// The bare email address is used as the SMTP envelope sender (MAIL FROM),
	// which Gmail requires to match the authenticated account.
	fromHeader, envelopeFrom := parseFrom(s.config.From)

	// Build MIME message
	message := buildMIMEMessage(fromHeader, to, subject, textBody, htmlBody)

	// SMTP server address
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	// Authentication
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)

	// Send email
	err := smtp.SendMail(
		addr,
		auth,
		envelopeFrom,
		[]string{to},
		[]byte(message),
	)

	if err != nil {
		return fmt.Errorf("failed to send email to %s: %w", to, err)
	}

	return nil
}

// parseFrom parses a From value like "Name <email>" or plain "email".
// Returns (RFC2047-encoded From header value, bare email for SMTP envelope).
func parseFrom(from string) (header string, envelope string) {
	addr, err := mail.ParseAddress(from)
	if err != nil {
		// Fallback: treat the whole value as a plain email address
		return from, from
	}
	if addr.Name == "" {
		return addr.Address, addr.Address
	}
	encoded := mime.QEncoding.Encode("utf-8", addr.Name)
	return encoded + " <" + addr.Address + ">", addr.Address
}

// buildMIMEMessage constructs a multipart MIME email message
func buildMIMEMessage(from string, to string, subject string, textBody string, htmlBody string) string {
	var msg strings.Builder

	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: multipart/alternative; boundary=\"boundary123\"\r\n")
	msg.WriteString("\r\n")

	// Plain text part
	msg.WriteString("--boundary123\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(textBody)
	msg.WriteString("\r\n\r\n")

	// HTML part
	msg.WriteString("--boundary123\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)
	msg.WriteString("\r\n\r\n")

	msg.WriteString("--boundary123--\r\n")

	return msg.String()
}
