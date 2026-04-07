package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const expoAPIURL = "https://exp.host/--/api/v2/push/send"

// Service handles push token management and notification delivery.
type Service struct {
	repo       Repository
	httpClient *http.Client
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// RegisterToken saves a push token for a user.
func (s *Service) RegisterToken(ctx context.Context, userID, token, platform string) error {
	if platform == "" {
		platform = "expo"
	}
	t := PushToken{
		ID:       uuid.New(),
		UserID:   userID,
		Token:    token,
		Platform: platform,
	}
	if err := s.repo.Save(ctx, t); err != nil {
		return fmt.Errorf("register token: %w", err)
	}
	return nil
}

// UnregisterToken removes a push token for a user.
func (s *Service) UnregisterToken(ctx context.Context, userID, token string) error {
	if err := s.repo.Delete(ctx, userID, token); err != nil {
		return fmt.Errorf("unregister token: %w", err)
	}
	return nil
}

// SendToUser sends a push notification to all devices of a specific user.
func (s *Service) SendToUser(ctx context.Context, userID, title, body string, data map[string]any) error {
	tokens, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get tokens for user: %w", err)
	}
	if len(tokens) == 0 {
		return nil
	}

	messages := make([]expoMessage, 0, len(tokens))
	for _, t := range tokens {
		messages = append(messages, expoMessage{
			To:    t.Token,
			Title: title,
			Body:  body,
			Data:  data,
		})
	}
	return s.sendMessages(ctx, messages)
}

// SendToAll sends a push notification to all registered devices.
func (s *Service) SendToAll(ctx context.Context, title, body string, data map[string]any) error {
	tokens, err := s.repo.GetAllActive(ctx)
	if err != nil {
		return fmt.Errorf("get all active tokens: %w", err)
	}
	if len(tokens) == 0 {
		return nil
	}

	messages := make([]expoMessage, 0, len(tokens))
	for _, t := range tokens {
		messages = append(messages, expoMessage{
			To:    t.Token,
			Title: title,
			Body:  body,
			Data:  data,
		})
	}
	return s.sendMessages(ctx, messages)
}

type expoMessage struct {
	To    string         `json:"to"`
	Title string         `json:"title"`
	Body  string         `json:"body"`
	Data  map[string]any `json:"data,omitempty"`
}

func (s *Service) sendMessages(ctx context.Context, messages []expoMessage) error {
	payload, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("marshal expo messages: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, expoAPIURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create expo request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send expo push: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("expo push API error: status %d", resp.StatusCode)
	}
	return nil
}
