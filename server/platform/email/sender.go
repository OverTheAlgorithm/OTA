package email

import (
	"fmt"
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
	// Build MIME message
	message := buildMIMEMessage(s.config.From, to, subject, textBody, htmlBody)

	// SMTP server address
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	// Authentication
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)

	// Send email
	err := smtp.SendMail(
		addr,
		auth,
		s.config.From,
		[]string{to},
		[]byte(message),
	)

	if err != nil {
		return fmt.Errorf("failed to send email to %s: %w", to, err)
	}

	return nil
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
