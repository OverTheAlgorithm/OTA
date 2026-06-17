package collector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// scriptedImageClient returns the queued result on each call, recording how many
// times it was invoked.
type scriptedImageClient struct {
	errs  []error // err for attempt i; nil means success
	calls int
}

func (c *scriptedImageClient) Generate(_ context.Context, _ string) ([]byte, string, error) {
	i := c.calls
	c.calls++
	if i < len(c.errs) && c.errs[i] != nil {
		return nil, "", c.errs[i]
	}
	return []byte{0x89, 0x50, 0x4e, 0x47}, "image/png", nil
}

func TestImageGenerator_RetriesOnRetryableError(t *testing.T) {
	quota := errors.New("vertex image generation: Error 429, Status: RESOURCE_EXHAUSTED")
	client := &scriptedImageClient{errs: []error{quota, quota, nil}} // fail twice, then succeed

	gen := NewImageGenerator(client, t.TempDir()).WithRetry(3, time.Millisecond)

	result := gen.GenerateForItems(context.Background(), []ContextItem{
		{ID: uuid.New(), Topic: "테스트 토픽"},
	})

	if len(result) != 1 {
		t.Fatalf("expected 1 generated image, got %d", len(result))
	}
	if client.calls != 3 {
		t.Errorf("expected 3 attempts (2 retries), got %d", client.calls)
	}
}

func TestImageGenerator_StopsAtMaxAttempts(t *testing.T) {
	quota := errors.New("Error 429 resource has been exhausted")
	client := &scriptedImageClient{errs: []error{quota, quota, quota}}

	gen := NewImageGenerator(client, t.TempDir()).WithRetry(2, time.Millisecond)

	result := gen.GenerateForItems(context.Background(), []ContextItem{
		{ID: uuid.New(), Topic: "테스트 토픽"},
	})

	if len(result) != 0 {
		t.Fatalf("expected 0 generated images, got %d", len(result))
	}
	if client.calls != 2 {
		t.Errorf("expected exactly 2 attempts (maxAttempts), got %d", client.calls)
	}
}

func TestImageGenerator_NoRetryByDefault(t *testing.T) {
	quota := errors.New("Error 429 RESOURCE_EXHAUSTED")
	client := &scriptedImageClient{errs: []error{quota}}

	gen := NewImageGenerator(client, t.TempDir()) // default: maxAttempts=1

	gen.GenerateForItems(context.Background(), []ContextItem{
		{ID: uuid.New(), Topic: "테스트 토픽"},
	})

	if client.calls != 1 {
		t.Errorf("expected 1 attempt with default (no retry), got %d", client.calls)
	}
}

func TestImageGenerator_NoRetryOnNonRetryableError(t *testing.T) {
	// "no candidates" classifies as a format error -> not retryable.
	formatErr := errors.New("vertex: no candidates in response")
	client := &scriptedImageClient{errs: []error{formatErr, nil}}

	gen := NewImageGenerator(client, t.TempDir()).WithRetry(3, time.Millisecond)

	gen.GenerateForItems(context.Background(), []ContextItem{
		{ID: uuid.New(), Topic: "테스트 토픽"},
	})

	if client.calls != 1 {
		t.Errorf("expected 1 attempt (non-retryable error), got %d", client.calls)
	}
}
