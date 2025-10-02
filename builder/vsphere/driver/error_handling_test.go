// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"fmt"
	"strings"
	"testing"
)

// TestSanitizeErrorMessage tests the sanitization of sensitive information in error messages.
func TestSanitizeErrorMessage(t *testing.T) {
	driver := &VCenterDriver{}

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "URL with credentials",
			input:    "error accessing https://user:password@packages.example.com/artifacts/example.ovf",
			expected: "error accessing https://packages.example.com/artifacts/example.ovf",
		},
		{
			name:     "Password in error message",
			input:    "authentication failed: password=testpass",
			expected: "authentication failed: [credentials removed]",
		},
		{
			name:     "Multiple credential patterns",
			input:    "failed with password=secret and token=abc123",
			expected: "failed with [credentials removed] and [credentials removed]",
		},
		{
			name:     "No credentials to sanitize",
			input:    "network timeout error",
			expected: "network timeout error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := driver.sanitizeErrorMessage(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// TestSanitizeURL tests the sanitization of credentials from URLs.
func TestSanitizeURL(t *testing.T) {
	driver := &VCenterDriver{}

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "URL with username and password",
			input:    "https://testuser:testpass@packages.example.com/artifacts/example.ovf",
			expected: "https://testuser@packages.example.com/artifacts/example.ovf",
		},
		{
			name:     "URL without credentials",
			input:    "https://packages.example.com/artifacts/example.ovf",
			expected: "https://packages.example.com/artifacts/example.ovf",
		},
		{
			name:     "Relative URL without credentials",
			input:    "not-a-url",
			expected: "not-a-url",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := driver.sanitizeURL(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// TestCategorizeOvfImportError tests the categorization of OVF import errors.
func TestCategorizeOvfImportError(t *testing.T) {
	driver := &VCenterDriver{}

	testCases := []struct {
		name           string
		inputError     error
		expectedPrefix string
	}{
		{
			name:           "Authentication error",
			inputError:     fmt.Errorf("HTTP 401 Unauthorized"),
			expectedPrefix: "authentication failed when accessing remote OVF/OVA source",
		},
		{
			name:           "File not found error",
			inputError:     fmt.Errorf("HTTP 404 Not Found"),
			expectedPrefix: "remote OVF/OVA file not found",
		},
		{
			name:           "Network timeout error",
			inputError:     fmt.Errorf("connection timeout"),
			expectedPrefix: "network connectivity error accessing remote OVF/OVA source",
		},
		{
			name:           "TLS certificate error",
			inputError:     fmt.Errorf("x509: certificate verify failed"),
			expectedPrefix: "TLS/SSL certificate error accessing remote OVF/OVA source",
		},
		{
			name:           "OVF validation error",
			inputError:     fmt.Errorf("invalid OVF descriptor"),
			expectedPrefix: "OVF/OVA file validation error",
		},
		{
			name:           "Resource error",
			inputError:     fmt.Errorf("insufficient disk space"),
			expectedPrefix: "insufficient vSphere resources for OVF deployment",
		},
		{
			name:           "Generic error",
			inputError:     fmt.Errorf("unknown error"),
			expectedPrefix: "OVF deployment failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := driver.categorizeOvfImportError(tc.inputError)
			if !strings.HasPrefix(result.Error(), tc.expectedPrefix) {
				t.Errorf("expected error to start with %q, got %q", tc.expectedPrefix, result.Error())
			}
		})
	}
}

// TestWrapOvfError tests the wrapping of OVF errors with context and sanitization.
func TestWrapOvfError(t *testing.T) {
	driver := &VCenterDriver{}

	context := "test operation failed"
	err := fmt.Errorf("original error")
	url := "https://testuser:testpass@packages.example.com/artifacts/example.ovf"

	result := driver.wrapOvfError(context, err, url)

	if !strings.Contains(result.Error(), context) {
		t.Errorf("expected error to contain context %q", context)
	}

	if strings.Contains(result.Error(), "password") {
		t.Errorf("expected error to not contain password, got %q", result.Error())
	}

	if !strings.Contains(result.Error(), "testuser@packages.example.com") {
		t.Errorf("expected error to contain sanitized URL with username")
	}
}
