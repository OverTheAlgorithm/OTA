package collector

import "context"

type AIResponse struct {
	OutputText  string
	Annotations []AIAnnotation
	RawJSON     string
}

type AIAnnotation struct {
	URL   string
	Title string
}

type AIClient interface {
	SearchAndAnalyze(ctx context.Context, prompt string) (AIResponse, error)
}
