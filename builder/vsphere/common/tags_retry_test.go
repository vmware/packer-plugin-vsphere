// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts to be 3, got %d", config.MaxAttempts)
	}

	if config.InitialDelay != 1*time.Second {
		t.Errorf("Expected InitialDelay to be 1s, got %v", config.InitialDelay)
	}

	if config.MaxDelay != 30*time.Second {
		t.Errorf("Expected MaxDelay to be 30s, got %v", config.MaxDelay)
	}

	if config.BackoffFactor != 2.0 {
		t.Errorf("Expected BackoffFactor to be 2.0, got %f", config.BackoffFactor)
	}
}

func TestWithRetry_Success(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
	}

	callCount := 0
	operation := func() error {
		callCount++
		return nil
	}

	err := withRetry(ctx, config, operation)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected operation to be called once, got %d", callCount)
	}
}

func TestWithRetry_SuccessAfterRetries(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
	}

	callCount := 0
	operation := func() error {
		callCount++
		if callCount < 3 {
			return errors.New("timeout error")
		}
		return nil
	}

	err := withRetry(ctx, config, operation)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if callCount != 3 {
		t.Errorf("Expected operation to be called 3 times, got %d", callCount)
	}
}

func TestWithRetry_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
	}

	callCount := 0
	operation := func() error {
		callCount++
		return errors.New("permission denied")
	}

	err := withRetry(ctx, config, operation)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "non-retryable error") {
		t.Errorf("Expected non-retryable error message, got %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected operation to be called once, got %d", callCount)
	}
}

func TestWithRetry_MaxAttemptsExceeded(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
	}

	callCount := 0
	operation := func() error {
		callCount++
		return errors.New("timeout error")
	}

	err := withRetry(ctx, config, operation)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "operation failed after 3 attempts") {
		t.Errorf("Expected max attempts error message, got %v", err)
	}

	if callCount != 3 {
		t.Errorf("Expected operation to be called 3 times, got %d", callCount)
	}
}

func TestWithRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := &RetryConfig{
		MaxAttempts:   5,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
	}

	callCount := 0
	operation := func() error {
		callCount++
		if callCount == 2 {
			cancel() // Cancel context after second attempt
		}
		return errors.New("timeout error")
	}

	err := withRetry(ctx, config, operation)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "operation cancelled") {
		t.Errorf("Expected context cancelled error, got %v", err)
	}

	// Should be called at least twice (once before cancel, once after)
	if callCount < 2 {
		t.Errorf("Expected operation to be called at least 2 times, got %d", callCount)
	}
}

func TestIsRetryableError_RetryableErrors(t *testing.T) {
	retryableErrors := []string{
		"connection timeout",
		"network error",
		"temporary failure",
		"service unavailable",
		"rate limit exceeded",
		"too many requests",
		"gateway timeout",
		"bad gateway",
	}

	for _, errMsg := range retryableErrors {
		err := errors.New(errMsg)
		if !isRetryableError(err) {
			t.Errorf("Expected error '%s' to be retryable", errMsg)
		}
	}
}

func TestIsRetryableError_NonRetryableErrors(t *testing.T) {
	nonRetryableErrors := []string{
		"permission denied",
		"unauthorized access",
		"forbidden operation",
		"invalid configuration",
		"resource not found",
		"category does not exist",
		"tag already exists",
		"not associable with VirtualMachine",
	}

	for _, errMsg := range nonRetryableErrors {
		err := errors.New(errMsg)
		if isRetryableError(err) {
			t.Errorf("Expected error '%s' to be non-retryable", errMsg)
		}
	}
}

func TestIsRetryableError_NilError(t *testing.T) {
	if isRetryableError(nil) {
		t.Error("Expected nil error to be non-retryable")
	}
}

func TestIsRetryableError_UnknownError(t *testing.T) {
	// Unknown errors should not be retried by default
	err := errors.New("some unknown error")
	if isRetryableError(err) {
		t.Error("Expected unknown error to be non-retryable")
	}
}

func TestCalculateBackoff(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:   5,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 1 * time.Second},   // 1 * 2^0 = 1
		{2, 2 * time.Second},   // 1 * 2^1 = 2
		{3, 4 * time.Second},   // 1 * 2^2 = 4
		{4, 8 * time.Second},   // 1 * 2^3 = 8
		{5, 16 * time.Second},  // 1 * 2^4 = 16
		{6, 30 * time.Second},  // 1 * 2^5 = 32, capped at 30
		{10, 30 * time.Second}, // Capped at max delay
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			delay := calculateBackoff(tt.attempt, config)
			if delay != tt.expected {
				t.Errorf("Expected delay %v for attempt %d, got %v", tt.expected, tt.attempt, delay)
			}
		})
	}
}

func TestCalculateBackoff_NilConfig(t *testing.T) {
	// Should use default config
	delay := calculateBackoff(1, nil)
	if delay != 1*time.Second {
		t.Errorf("Expected delay 1s with nil config, got %v", delay)
	}
}

func TestWithRetry_NilConfig(t *testing.T) {
	ctx := context.Background()

	callCount := 0
	operation := func() error {
		callCount++
		return nil
	}

	// Should use default config when nil is passed
	err := withRetry(ctx, nil, operation)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected operation to be called once, got %d", callCount)
	}
}
