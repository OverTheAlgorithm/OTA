# E2E Tests

End-to-end tests with **real external APIs** (Gemini, OpenAI).

## Requirements
- Docker Desktop running
- Valid API keys

## Setup
```bash
export GEMINI_API_KEY="your-key"
export GEMINI_MODEL="gemini-2.5-flash-lite"
```

## Run
```bash
go test ./e2e/... -v -timeout 180s
```

Tests are automatically skipped if API keys are not set.
