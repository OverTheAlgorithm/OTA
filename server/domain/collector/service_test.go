package collector

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// --- mocks ---

type mockAIClient struct {
	resp AIResponse
	err  error
}

func (m *mockAIClient) SearchAndAnalyze(_ context.Context, _ string) (AIResponse, error) {
	return m.resp, m.err
}

type mockRepo struct {
	createRunErr     error
	completeRunErr   error
	saveItemsErr     error
	completedStatus  RunStatus
	completedErrMsg  *string
	completedRawResp *string
	savedItems       []ContextItem
}

func (m *mockRepo) CreateRun(_ context.Context, _ CollectionRun) error {
	return m.createRunErr
}

func (m *mockRepo) CompleteRun(_ context.Context, _ uuid.UUID, status RunStatus, errMsg *string, rawResp *string) error {
	m.completedStatus = status
	m.completedErrMsg = errMsg
	m.completedRawResp = rawResp
	return m.completeRunErr
}

func (m *mockRepo) SaveContextItems(_ context.Context, items []ContextItem) error {
	m.savedItems = items
	return m.saveItemsErr
}

// --- tests ---

func TestCollect_Success(t *testing.T) {
	repo := &mockRepo{}
	aiClient := &mockAIClient{
		resp: AIResponse{
			OutputText: validCollectionJSON,
			RawJSON:    `{"raw":"data"}`,
		},
	}

	svc := NewService(aiClient, repo)
	result, err := svc.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Run.Status != RunStatusSuccess {
		t.Errorf("expected status success, got %s", result.Run.Status)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if result.Items[0].Category != "top" {
		t.Errorf("expected category top, got %s", result.Items[0].Category)
	}
	if result.Items[1].Category != "entertainment" {
		t.Errorf("expected category entertainment, got %s", result.Items[1].Category)
	}
	if repo.completedStatus != RunStatusSuccess {
		t.Errorf("expected repo completed with success, got %s", repo.completedStatus)
	}
	if len(repo.savedItems) != 2 {
		t.Errorf("expected 2 saved items, got %d", len(repo.savedItems))
	}
}

func TestCollect_AIFailure(t *testing.T) {
	repo := &mockRepo{}
	aiClient := &mockAIClient{err: errors.New("openai down")}

	svc := NewService(aiClient, repo)
	_, err := svc.Collect(context.Background())
	if err == nil {
		t.Fatal("expected error when AI fails")
	}
	if repo.completedStatus != RunStatusFailed {
		t.Errorf("expected repo completed with failed, got %s", repo.completedStatus)
	}
}

func TestCollect_MalformedAIResponse(t *testing.T) {
	repo := &mockRepo{}
	aiClient := &mockAIClient{
		resp: AIResponse{OutputText: "not json at all", RawJSON: `{"raw":"bad"}`},
	}

	svc := NewService(aiClient, repo)
	_, err := svc.Collect(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed AI response")
	}
	if repo.completedStatus != RunStatusFailed {
		t.Errorf("expected repo completed with failed, got %s", repo.completedStatus)
	}
	if repo.completedRawResp == nil {
		t.Error("expected raw response to be saved on parse failure")
	}
}

func TestCollect_CreateRunFailure(t *testing.T) {
	repo := &mockRepo{createRunErr: errors.New("db down")}
	aiClient := &mockAIClient{}

	svc := NewService(aiClient, repo)
	_, err := svc.Collect(context.Background())
	if err == nil {
		t.Fatal("expected error when CreateRun fails")
	}
}

func TestCollect_SkipsInvalidItems(t *testing.T) {
	repo := &mockRepo{}
	aiClient := &mockAIClient{
		resp: AIResponse{
			OutputText: `{"items":[
				{"category":"top","rank":1,"topic":"유효","summary":"유효한 항목","sources":[]},
				{"category":"","rank":2,"topic":"빈 카테고리","summary":"필터됨","sources":[]},
				{"category":"top","rank":3,"topic":"","summary":"빈 토픽","sources":[]}
			]}`,
			RawJSON: "{}",
		},
	}

	svc := NewService(aiClient, repo)
	result, err := svc.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 valid item after filtering, got %d", len(result.Items))
	}
}

func TestCollect_AllItemsInvalid(t *testing.T) {
	repo := &mockRepo{}
	aiClient := &mockAIClient{
		resp: AIResponse{
			OutputText: `{"items":[{"category":"","rank":1,"topic":"","summary":"","sources":[]}]}`,
			RawJSON:    "{}",
		},
	}

	svc := NewService(aiClient, repo)
	_, err := svc.Collect(context.Background())
	if err == nil {
		t.Fatal("expected error when all items are invalid")
	}
}

const validCollectionJSON = `{
	"items": [
		{
			"category": "top",
			"rank": 1,
			"topic": "환승연애 시즌3",
			"summary": "출연자가 전 남자친구를 두 명이나 데리고 출연하며 화제를 모으고 있다.",
			"sources": ["https://example.com/news1"]
		},
		{
			"category": "entertainment",
			"rank": 1,
			"topic": "뉴진스 컴백",
			"summary": "뉴진스가 새 앨범으로 컴백을 발표했다.",
			"sources": ["https://example.com/news2"]
		}
	]
}`
