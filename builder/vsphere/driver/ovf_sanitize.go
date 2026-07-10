// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"net/url"
	"regexp"
)

var (
	ovfURLWithCredentialsPattern = regexp.MustCompile(`https?://[^:]+:[^@]+@[^\s]+`)
	ovfCredentialPatterns        = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(password[=:]\s*)[^\s&]+`),
		regexp.MustCompile(`(?i)(passwd[=:]\s*)[^\s&]+`),
		regexp.MustCompile(`(?i)(pwd[=:]\s*)[^\s&]+`),
		regexp.MustCompile(`(?i)(token[=:]\s*)[^\s&]+`),
		regexp.MustCompile(`(?i)(auth[=:]\s*)[^\s&]+`),
		regexp.MustCompile(`(?i)(credential[s]?[=:]\s*)[^\s&]+`),
	}
)

// SanitizeOvfURL removes credentials from URLs for safe logging.
func SanitizeOvfURL(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "[invalid URL]"
	}

	u.User = nil
	return u.String()
}

// SanitizeOvfSource returns a safe label for an OVF source. HTTP(S) URLs have
// credentials stripped; local filesystem paths are returned unchanged.
func SanitizeOvfSource(urlStr, pathStr string) string {
	if pathStr != "" {
		return pathStr
	}
	return SanitizeOvfURL(urlStr)
}

// SanitizeOvfErrorMessage removes sensitive information from error messages.
func SanitizeOvfErrorMessage(errMsg string) string {
	sanitized := sanitizeOvfURLsInString(errMsg)
	return sanitizeOvfCredentialPatterns(sanitized)
}

func sanitizeOvfURLsInString(str string) string {
	return ovfURLWithCredentialsPattern.ReplaceAllStringFunc(str, func(match string) string {
		if u, err := url.Parse(match); err == nil {
			u.User = nil
			return u.String()
		}
		return "[URL with credentials removed]"
	})
}

func sanitizeOvfCredentialPatterns(str string) string {
	sanitized := str
	for _, re := range ovfCredentialPatterns {
		sanitized = re.ReplaceAllString(sanitized, "$1[credentials removed]")
	}
	return sanitized
}
