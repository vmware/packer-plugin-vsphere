// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// RetryConfig configures retry behavior for tag operations.
type RetryConfig struct {
	// MaxAttempts is the maximum number of attempts (including the initial attempt).
	// Default: 3
	MaxAttempts int

	// InitialDelay is the delay before the first retry.
	// Default: 1s
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries.
	// Default: 30s
	MaxDelay time.Duration

	// BackoffFactor is the multiplier for exponential backoff.
	// Default: 2.0
	BackoffFactor float64
}

// DefaultRetryConfig returns a RetryConfig with default values.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
	}
}

// retryableOperation wraps a function that can be retried.
type retryableOperation func() error

// withRetry executes an operation with retry logic and exponential backoff.
func withRetry(ctx context.Context, config *RetryConfig, operation retryableOperation) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Execute the operation
		err := operation()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}

		// Don't sleep after the last attempt
		if attempt >= config.MaxAttempts {
			break
		}

		// Check if context is canceled
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation canceled: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}

		// Calculate next delay with exponential backoff
		delay = min(time.Duration(float64(delay)*config.BackoffFactor), config.MaxDelay)
	}

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

// isRetryableError determines if an error should trigger a retry.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())

	// Non-retryable errors
	nonRetryablePatterns := []string{
		"permission denied",
		"unauthorized",
		"forbidden",
		"invalid",
		"not found",
		"does not exist",
		"already exists",
		"not associable",
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return false
		}
	}

	// Retryable errors (transient issues)
	retryablePatterns := []string{
		"timeout",
		"connection",
		"network",
		"temporary",
		"unavailable",
		"rate limit",
		"too many requests",
		"service unavailable",
		"gateway timeout",
		"bad gateway",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	// Default: don't retry unknown errors to avoid infinite loops
	return false
}

// calculateBackoff calculates the delay for a given attempt number.
func calculateBackoff(attempt int, config *RetryConfig) time.Duration {
	if config == nil {
		config = DefaultRetryConfig()
	}

	// Calculate exponential backoff: initialDelay * (backoffFactor ^ (attempt - 1))
	delay := float64(config.InitialDelay) * math.Pow(config.BackoffFactor, float64(attempt-1))

	// Cap at max delay
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	return time.Duration(delay)
}
