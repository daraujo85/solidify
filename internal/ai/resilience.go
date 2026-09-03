package ai

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// ErrorCategory classifica falha operacional de provider.
type ErrorCategory string

const (
	ErrorRateLimited          ErrorCategory = "RATE_LIMITED"
	ErrorQuotaExhausted       ErrorCategory = "QUOTA_EXHAUSTED"
	ErrorCreditsDepleted      ErrorCategory = "CREDITS_DEPLETED"
	ErrorModelUnavailable     ErrorCategory = "MODEL_UNAVAILABLE"
	ErrorModelNotFound        ErrorCategory = "MODEL_NOT_FOUND"
	ErrorAuth                 ErrorCategory = "AUTH_ERROR"
	ErrorPermissionDenied     ErrorCategory = "PERMISSION_DENIED"
	ErrorProviderUnavailable  ErrorCategory = "PROVIDER_UNAVAILABLE"
	ErrorNetwork              ErrorCategory = "NETWORK_ERROR"
	ErrorTimeout              ErrorCategory = "TIMEOUT"
	ErrorSchema               ErrorCategory = "SCHEMA_ERROR"
	ErrorInvalidResponse      ErrorCategory = "INVALID_RESPONSE"
	ErrorContextLimitExceeded ErrorCategory = "CONTEXT_LIMIT_EXCEEDED"
	ErrorUnknownProvider      ErrorCategory = "UNKNOWN_PROVIDER_ERROR"
)

// ProviderError é diagnóstico seguro para decidir retry/failover.
type ProviderError struct {
	Category           ErrorCategory `json:"category"`
	HTTPStatus         int           `json:"http_status,omitempty"`
	Provider           string        `json:"provider,omitempty"`
	Model              string        `json:"model,omitempty"`
	RetryableSameModel bool          `json:"retryable_same_model"`
	FallbackAllowed    bool          `json:"fallback_allowed"`
	RetryAfterSeconds  *int          `json:"retry_after_seconds,omitempty"`
	RawReason          string        `json:"raw_reason,omitempty"`
}

func (e ProviderError) Error() string { return string(e.Category) + ": " + e.RawReason }

// ClassifyProviderError centraliza semântica sem expor credenciais.
func ClassifyProviderError(err error) ProviderError {
	if err == nil {
		return ProviderError{Category: ErrorUnknownProvider}
	}
	s := strings.ToLower(err.Error())
	status := statusFromError(s)
	p := ProviderError{HTTPStatus: status, RawReason: sanitizeProviderReason(err.Error())}
	switch {
	case strings.Contains(s, "prepayment credits") || strings.Contains(s, "credits depleted"):
		p.Category = ErrorCreditsDepleted
	case strings.Contains(s, "quota exceeded") || strings.Contains(s, "quota exhausted"):
		p.Category = ErrorQuotaExhausted
	case status == http.StatusTooManyRequests || strings.Contains(s, "rate limit"):
		p.Category = ErrorRateLimited
	case status == http.StatusUnauthorized || strings.Contains(s, "invalid token"):
		p.Category = ErrorAuth
	case status == http.StatusForbidden:
		p.Category = ErrorPermissionDenied
	case status == http.StatusNotFound || strings.Contains(s, "model not found"):
		p.Category = ErrorModelNotFound
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(s, "deadline exceeded") || strings.Contains(s, "timeout"):
		p.Category = ErrorTimeout
	case strings.Contains(s, "connection refused") || strings.Contains(s, "network") || strings.Contains(s, "no such host"):
		p.Category = ErrorNetwork
	case status >= 500:
		p.Category = ErrorProviderUnavailable
	case strings.Contains(s, "context limit") || strings.Contains(s, "context length"):
		p.Category = ErrorContextLimitExceeded
	default:
		p.Category = ErrorUnknownProvider
	}
	p.RetryableSameModel = p.Category == ErrorRateLimited || p.Category == ErrorNetwork || p.Category == ErrorTimeout
	p.FallbackAllowed = p.Category != ErrorAuth && p.Category != ErrorPermissionDenied && p.Category != ErrorSchema && p.Category != ErrorInvalidResponse
	return p
}

func statusFromError(s string) int {
	for _, marker := range []string{"status ", "http ", "http status "} {
		if i := strings.Index(s, marker); i >= 0 {
			v := s[i+len(marker):]
			n := 0
			for n < len(v) && v[n] >= '0' && v[n] <= '9' { n++ }
			if n > 0 { code, _ := strconv.Atoi(v[:n]); return code }
		}
	}
	return 0
}

func sanitizeProviderReason(s string) string {
	lower := strings.ToLower(s)
	for _, marker := range []string{"authorization: bearer ", "api_key=", "apikey=", "token="} {
		if i := strings.Index(lower, marker); i >= 0 {
			return s[:i] + "[REDACTED]"
		}
	}
	return s
}
