package email

import "fmt"

// MockSender is a mock implementation for testing
type MockSender struct {
	SentEmails []SentEmail
	ShouldFail bool
	FailError  error
}

// SentEmail records a sent email for verification
type SentEmail struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

// NewMockSender creates a new mock sender
func NewMockSender() *MockSender {
	return &MockSender{
		SentEmails: []SentEmail{},
	}
}

// Send records the email and optionally fails
func (m *MockSender) Send(to string, subject string, textBody string, htmlBody string) error {
	if m.ShouldFail {
		if m.FailError != nil {
			return m.FailError
		}
		return fmt.Errorf("mock send failure")
	}

	m.SentEmails = append(m.SentEmails, SentEmail{
		To:       to,
		Subject:  subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
	})

	return nil
}

// Reset clears all recorded emails
func (m *MockSender) Reset() {
	m.SentEmails = []SentEmail{}
	m.ShouldFail = false
	m.FailError = nil
}

// GetSentCount returns the number of emails sent
func (m *MockSender) GetSentCount() int {
	return len(m.SentEmails)
}

// GetLastSent returns the last sent email
func (m *MockSender) GetLastSent() *SentEmail {
	if len(m.SentEmails) == 0 {
		return nil
	}
	return &m.SentEmails[len(m.SentEmails)-1]
}
