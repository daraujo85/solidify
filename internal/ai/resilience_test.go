package ai

import (
	"errors"
	"testing"
)

func TestClassifyProviderError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ErrorCategory
	}{
		{"credits", errors.New("openai: status 429: Your prepayment credits are depleted."), ErrorCreditsDepleted},
		{"quota", errors.New("openai: status 429: quota exceeded; reset in 1 hour"), ErrorQuotaExhausted},
		{"rate", errors.New("openai: status 429: rate limit exceeded; retry-after: 2"), ErrorRateLimited},
		{"auth", errors.New("openai: status 401: invalid token"), ErrorAuth},
		{"not found", errors.New("openai: status 404: model not found"), ErrorModelNotFound},
		{"timeout", errors.New("openai: POST: context deadline exceeded"), ErrorTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyProviderError(tt.err)
			if got.Category != tt.want {
				t.Fatalf("category=%s, want %s", got.Category, tt.want)
			}
		})
	}
}
