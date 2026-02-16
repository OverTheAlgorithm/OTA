package kakao

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	authURL  = "https://kauth.kakao.com/oauth/authorize"
	tokenURL = "https://kauth.kakao.com/oauth/token"
	userURL  = "https://kapi.kakao.com/v2/user/me"
)

type Client struct {
	clientID     string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type UserResponse struct {
	ID      int64       `json:"id"`
	Account UserAccount `json:"kakao_account"`
}

type UserAccount struct {
	Email   string      `json:"email"`
	Profile UserProfile `json:"profile"`
}

type UserProfile struct {
	Nickname        string `json:"nickname"`
	ProfileImageURL string `json:"profile_image_url"`
}

func NewClient(clientID, clientSecret, redirectURI string) *Client {
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		httpClient:   &http.Client{},
	}
}

func (k *Client) AuthorizationURL(state string) string {
	params := url.Values{
		"client_id":     {k.clientID},
		"redirect_uri":  {k.redirectURI},
		"response_type": {"code"},
		"state":         {state},
	}
	return authURL + "?" + params.Encode()
}

func (k *Client) ExchangeCode(ctx context.Context, code string) (TokenResponse, error) {
	data := url.Values{
		"grant_type":   {"authorization_code"},
		"client_id":    {k.clientID},
		"redirect_uri": {k.redirectURI},
		"code":         {code},
	}
	if k.clientSecret != "" {
		data.Set("client_secret", k.clientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return TokenResponse{}, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return TokenResponse{}, fmt.Errorf("token request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return TokenResponse{}, fmt.Errorf("parse token response: %w", err)
	}

	return tokenResp, nil
}

func (k *Client) FetchUser(ctx context.Context, accessToken string) (UserResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userURL, nil)
	if err != nil {
		return UserResponse{}, fmt.Errorf("create user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return UserResponse{}, fmt.Errorf("user request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return UserResponse{}, fmt.Errorf("read user response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return UserResponse{}, fmt.Errorf("user request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var userResp UserResponse
	if err := json.Unmarshal(body, &userResp); err != nil {
		return UserResponse{}, fmt.Errorf("parse user response: %w", err)
	}

	return userResp, nil
}
