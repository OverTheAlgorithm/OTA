package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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

// RegisterToken saves a push token, optionally linked to a user.
// If userID is empty, the token is registered anonymously (user_id = NULL).
func (s *Service) RegisterToken(ctx context.Context, userID, token, platform string) error {
	if platform == "" {
		platform = "expo"
	}
	t := PushToken{
		ID:       uuid.New(),
		Token:    token,
		Platform: platform,
	}
	if userID != "" {
		t.UserID = &userID
	}
	if err := s.repo.Save(ctx, t); err != nil {
		return fmt.Errorf("register token: %w", err)
	}
	return nil
}

// UnlinkToken removes the user association from a token without deleting it.
// The token stays for anonymous push delivery.
func (s *Service) UnlinkToken(ctx context.Context, userID, token string) error {
	if err := s.repo.UnlinkUser(ctx, userID, token); err != nil {
		return fmt.Errorf("unlink token: %w", err)
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

// expoResponse / expoTicket model the Expo Push API response.
type expoTicket struct {
	Status  string          `json:"status"`
	Details json.RawMessage `json:"details,omitempty"`
}

type expoTicketDetails struct {
	Error string `json:"error"`
}

type expoResponse struct {
	Data []expoTicket `json:"data"`
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

	// Parse response to detect stale tokens (DeviceNotRegistered).
	var expoResp expoResponse
	if err := json.NewDecoder(resp.Body).Decode(&expoResp); err != nil {
		// Non-fatal: push was sent, just can't parse cleanup info.
		slog.Warn("failed to parse expo push response", "error", err)
		return nil
	}

	var staleTokens []string
	for i, ticket := range expoResp.Data {
		if ticket.Status != "error" || i >= len(messages) {
			continue
		}
		var details expoTicketDetails
		if err := json.Unmarshal(ticket.Details, &details); err != nil {
			continue
		}
		if details.Error == "DeviceNotRegistered" {
			staleTokens = append(staleTokens, messages[i].To)
		}
	}

	if len(staleTokens) > 0 {
		slog.Info("cleaning up stale push tokens", "count", len(staleTokens))
		if err := s.repo.DeleteByTokens(ctx, staleTokens); err != nil {
			slog.Error("failed to clean up stale push tokens", "error", err)
		}
	}

	return nil
}
