package auth

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
	kakaoAuthURL  = "https://kauth.kakao.com/oauth/authorize"
	kakaoTokenURL = "https://kauth.kakao.com/oauth/token"
	kakaoUserURL  = "https://kapi.kakao.com/v2/user/me"
)

type KakaoClient struct {
	clientID     string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client
}

type KakaoTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type KakaoUserResponse struct {
	ID      int64            `json:"id"`
	Account KakaoUserAccount `json:"kakao_account"`
}

type KakaoUserAccount struct {
	Email   string           `json:"email"`
	Profile KakaoUserProfile `json:"profile"`
}

type KakaoUserProfile struct {
	Nickname        string `json:"nickname"`
	ProfileImageURL string `json:"profile_image_url"`
}

func NewKakaoClient(clientID, clientSecret, redirectURI string) *KakaoClient {
	return &KakaoClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		httpClient:   &http.Client{},
	}
}

func (k *KakaoClient) AuthorizationURL(state string) string {
	params := url.Values{
		"client_id":     {k.clientID},
		"redirect_uri":  {k.redirectURI},
		"response_type": {"code"},
		"state":         {state},
	}
	return kakaoAuthURL + "?" + params.Encode()
}

func (k *KakaoClient) ExchangeCode(ctx context.Context, code string) (KakaoTokenResponse, error) {
	data := url.Values{
		"grant_type":   {"authorization_code"},
		"client_id":    {k.clientID},
		"redirect_uri": {k.redirectURI},
		"code":         {code},
	}
	if k.clientSecret != "" {
		data.Set("client_secret", k.clientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, kakaoTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return KakaoTokenResponse{}, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return KakaoTokenResponse{}, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return KakaoTokenResponse{}, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return KakaoTokenResponse{}, fmt.Errorf("token request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp KakaoTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return KakaoTokenResponse{}, fmt.Errorf("parse token response: %w", err)
	}

	return tokenResp, nil
}

func (k *KakaoClient) FetchUser(ctx context.Context, accessToken string) (KakaoUserResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, kakaoUserURL, nil)
	if err != nil {
		return KakaoUserResponse{}, fmt.Errorf("create user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return KakaoUserResponse{}, fmt.Errorf("user request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return KakaoUserResponse{}, fmt.Errorf("read user response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return KakaoUserResponse{}, fmt.Errorf("user request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var userResp KakaoUserResponse
	if err := json.Unmarshal(body, &userResp); err != nil {
		return KakaoUserResponse{}, fmt.Errorf("parse user response: %w", err)
	}

	return userResp, nil
}
