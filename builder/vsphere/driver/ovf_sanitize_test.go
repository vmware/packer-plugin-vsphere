// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"testing"
)

// TestSanitizeOvfErrorMessage tests the sanitization of sensitive information in error messages.
func TestSanitizeOvfErrorMessage(t *testing.T) {
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
			result := SanitizeOvfErrorMessage(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// TestSanitizeOvfURL tests the sanitization of credentials from URLs.
func TestSanitizeOvfURL(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "URL with username and password",
			input:    "https://testuser:testpass@packages.example.com/artifacts/example.ovf",
			expected: "https://packages.example.com/artifacts/example.ovf",
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
			result := SanitizeOvfURL(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestSanitizeOvfSource(t *testing.T) {
	tests := []struct {
		name     string
		urlStr   string
		pathStr  string
		expected string
	}{
		{
			name:     "path takes precedence",
			urlStr:   "https://user:pass@packages.example.com/artifacts/example.ovf",
			pathStr:  "./artifacts/example.ova",
			expected: "./artifacts/example.ova",
		},
		{
			name:     "url with credentials stripped",
			urlStr:   "https://user:pass@packages.example.com/artifacts/example.ovf",
			pathStr:  "",
			expected: "https://packages.example.com/artifacts/example.ovf",
		},
		{
			name:     "url without credentials",
			urlStr:   "https://packages.example.com/artifacts/example.ovf",
			pathStr:  "",
			expected: "https://packages.example.com/artifacts/example.ovf",
		},
		{
			name:     "empty path and empty url",
			urlStr:   "",
			pathStr:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeOvfSource(tt.urlStr, tt.pathStr)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
