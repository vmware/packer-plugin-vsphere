// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"net/url"
	"regexp"
)

// SanitizeOvfURL removes credentials from URLs for safe logging.
func SanitizeOvfURL(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "[invalid URL]"
	}

	if u.User != nil {
		u.User = url.User(u.User.Username())
	}
	return u.String()
}

// SanitizeOvfErrorMessage removes sensitive information from error messages.
func SanitizeOvfErrorMessage(errMsg string) string {
	sanitized := sanitizeOvfURLsInString(errMsg)
	return sanitizeOvfCredentialPatterns(sanitized)
}

func sanitizeOvfURLsInString(str string) string {
	urlPattern := regexp.MustCompile(`https?://[^:]+:[^@]+@[^\s]+`)
	return urlPattern.ReplaceAllStringFunc(str, func(match string) string {
		if u, err := url.Parse(match); err == nil {
			u.User = nil
			return u.String()
		}
		return "[URL with credentials removed]"
	})
}

func sanitizeOvfCredentialPatterns(str string) string {
	patterns := []string{
		`password[=:]\s*[^\s&]+`,
		`passwd[=:]\s*[^\s&]+`,
		`pwd[=:]\s*[^\s&]+`,
		`token[=:]\s*[^\s&]+`,
		`auth[=:]\s*[^\s&]+`,
		`credential[s]?[=:]\s*[^\s&]+`,
	}

	sanitized := str
	for _, pattern := range patterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		sanitized = re.ReplaceAllString(sanitized, "[credentials removed]")
	}
	return sanitized
}
