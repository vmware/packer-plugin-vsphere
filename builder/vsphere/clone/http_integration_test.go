// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package clone

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestHttpTestServer validates the HTTP test server.
func TestHttpTestServer(t *testing.T) {
	server := NewHttpTestServer()
	defer server.Close()

	tests := []struct {
		name          string
		url           string
		expectStatus  int
		expectContent string
		useAuth       bool
		username      string
		password      string
		skipTLSVerify bool
	}{
		{
			name:          "http anonymous ovf access",
			url:           server.GetHttpUrl("example.ovf"),
			expectStatus:  http.StatusOK,
			expectContent: "<?xml version=\"1.0\" encoding=\"UTF-8\"?>",
		},
		{
			name:          "http anonymous ova access",
			url:           server.GetHttpUrl("example.ova"),
			expectStatus:  http.StatusOK,
			expectContent: "MINIMAL_OVA_CONTENT_FOR_TESTING",
		},
		{
			name:         "http basic authentication ovf access: valid credentials",
			url:          server.GetHttpAuthUrl("example.ovf"),
			expectStatus: http.StatusOK,
			useAuth:      true,
			username:     "testuser",
			password:     "testpass",
		},
		{
			name:         "http basic authentication ovf access: invalid credentials",
			url:          server.GetHttpAuthUrl("example.ovf"),
			expectStatus: http.StatusUnauthorized,
			useAuth:      true,
			username:     "wronguser",
			password:     "wrongpass",
		},
		{
			name:         "http basic authentication ovf access: no credentials",
			url:          server.GetHttpAuthUrl("example.ovf"),
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:          "https anonymous ovf access: skip tls verification",
			url:           server.GetHttpsUrl("example.ovf"),
			expectStatus:  http.StatusOK,
			skipTLSVerify: true,
		},
		{
			name:          "https basic authentication ovf access: skip tls verification",
			url:           server.GetHttpsAuthUrl("example.ovf"),
			expectStatus:  http.StatusOK,
			useAuth:       true,
			username:      "testuser",
			password:      "testpass",
			skipTLSVerify: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{
				Timeout: 5 * time.Second,
			}

			if tt.skipTLSVerify {
				client.Transport = &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				}
			}

			req, err := http.NewRequest("GET", tt.url, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			if tt.useAuth {
				req.SetBasicAuth(tt.username, tt.password)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, resp.StatusCode)
			}

			if tt.expectContent != "" {
				body := make([]byte, len(tt.expectContent))
				_, err := resp.Body.Read(body)
				if err != nil {
					t.Errorf("failed to read response body: %v", err)
				}
				if !strings.Contains(string(body), tt.expectContent) {
					t.Errorf("expected content to contain '%s', got '%s'", tt.expectContent, string(body))
				}
			}
		})
	}
}

// TestHttpClientConfiguration validates HTTP client configuration scenarios.
func TestHttpClientConfiguration(t *testing.T) {
	server := NewHttpTestServer()
	defer server.Close()

	t.Run("Anonymous HTTP Access Scenarios", func(t *testing.T) {
		tests := []struct {
			name        string
			fileType    string
			expectError bool
		}{
			{
				name:        "anonymous http ovf access",
				fileType:    "example.ovf",
				expectError: false,
			},
			{
				name:        "anonymous http ova access",
				fileType:    "example.ova",
				expectError: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &RemoteSourceConfig{
					URL: server.GetHttpUrl(tt.fileType),
				}

				client := createHttpClientFromConfig(config)
				req, err := http.NewRequest("GET", config.URL, nil)
				if err != nil {
					t.Fatalf("failed to create request: %v", err)
				}

				resp, err := client.Do(req)
				if tt.expectError {
					if err == nil && resp.StatusCode < 400 {
						t.Errorf("expected error but request succeeded")
					}
					if resp != nil {
						resp.Body.Close()
					}
					return
				}

				if err != nil {
					t.Errorf("unexpected error for anonymous access: %v", err)
					return
				}

				if resp.StatusCode != http.StatusOK {
					t.Errorf("expected status 200 for anonymous access, got %d", resp.StatusCode)
				}

				contentType := resp.Header.Get("Content-Type")
				if tt.fileType == "example.ovf" && contentType != "application/xml" {
					t.Errorf("expected Content-Type 'application/xml' for OVF, got '%s'", contentType)
				}
				if tt.fileType == "example.ova" && contentType != "application/octet-stream" {
					t.Errorf("expected Content-Type 'application/octet-stream' for OVA, got '%s'", contentType)
				}

				resp.Body.Close()
			})
		}
	})

	t.Run("Basic Authentication Scenarios", func(t *testing.T) {
		tests := []struct {
			name          string
			username      string
			password      string
			expectError   bool
			expectedError string
		}{
			{
				name:        "correct credentials",
				username:    "testuser",
				password:    "testpass",
				expectError: false,
			},
			{
				name:          "incorrect username",
				username:      "wronguser",
				password:      "testpass",
				expectError:   true,
				expectedError: "401 Unauthorized",
			},
			{
				name:          "incorrect password",
				username:      "testuser",
				password:      "wrongpass",
				expectError:   true,
				expectedError: "401 Unauthorized",
			},
			{
				name:          "empty credentials",
				username:      "",
				password:      "",
				expectError:   true,
				expectedError: "401 Unauthorized",
			},
			{
				name:          "missing password",
				username:      "testuser",
				password:      "",
				expectError:   true,
				expectedError: "401 Unauthorized",
			},
			{
				name:          "missing username",
				username:      "",
				password:      "testpass",
				expectError:   true,
				expectedError: "401 Unauthorized",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &RemoteSourceConfig{
					URL:      server.GetHttpAuthUrl("example.ovf"),
					Username: tt.username,
					Password: tt.password,
				}

				client := createHttpClientFromConfig(config)
				req, err := http.NewRequest("GET", config.URL, nil)
				if err != nil {
					t.Fatalf("failed to create request: %v", err)
				}

				if config.Username != "" && config.Password != "" {
					req.SetBasicAuth(config.Username, config.Password)
				}

				resp, err := client.Do(req)
				if tt.expectError {
					if err == nil && resp != nil && resp.StatusCode < 400 {
						t.Errorf("expected authentication error but request succeeded")
						resp.Body.Close()
						return
					}
					if resp != nil {
						if resp.StatusCode != http.StatusUnauthorized {
							t.Errorf("expected status 401 Unauthorized, got %d", resp.StatusCode)
						}
						resp.Body.Close()
					}
					return
				}

				if err != nil {
					t.Errorf("unexpected error with correct credentials: %v", err)
					return
				}

				if resp.StatusCode != http.StatusOK {
					t.Errorf("expected status 200 with correct credentials, got %d", resp.StatusCode)
				}

				resp.Body.Close()
			})
		}
	})

	t.Run("HTTPS with TLS Configuration", func(t *testing.T) {
		tests := []struct {
			name                  string
			skipTLSVerify         bool
			expectError           bool
			expectedErrorContains string
		}{
			{
				name:          "https without tls verification",
				skipTLSVerify: true,
				expectError:   false,
			},
			{
				name:                  "https with tls verification (should fail with self-signed cert)",
				skipTLSVerify:         false,
				expectError:           true,
				expectedErrorContains: "certificate",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &RemoteSourceConfig{
					URL:           server.GetHttpsUrl("example.ovf"),
					SkipTlsVerify: tt.skipTLSVerify,
				}

				client := createHttpClientFromConfig(config)
				req, err := http.NewRequest("GET", config.URL, nil)
				if err != nil {
					t.Fatalf("failed to create request: %v", err)
				}

				resp, err := client.Do(req)

				if tt.expectError {
					if err == nil {
						t.Error("expected tls error but request succeeded")
						if resp != nil {
							resp.Body.Close()
						}
						return
					}
					if !strings.Contains(err.Error(), tt.expectedErrorContains) {
						t.Errorf("expected error to contain '%s', got '%s'", tt.expectedErrorContains, err.Error())
					}
					return
				}

				if err != nil {
					t.Errorf("unexpected error with SkipTlsVerify: %v", err)
					return
				}

				if resp.StatusCode != http.StatusOK {
					t.Errorf("expected status 200 with SkipTlsVerify, got %d", resp.StatusCode)
				}

				resp.Body.Close()
			})
		}
	})

	t.Run("HTTPS with Authentication and TLS", func(t *testing.T) {
		tests := []struct {
			name                  string
			username              string
			password              string
			skipTLSVerify         bool
			expectError           bool
			expectedErrorContains string
		}{
			{
				name:          "https basic authentication with correct credentials and tls skip",
				username:      "testuser",
				password:      "testpass",
				skipTLSVerify: true,
				expectError:   false,
			},
			{
				name:                  "https basic authentication with incorrect credentials and tls skip",
				username:              "wronguser",
				password:              "wrongpass",
				skipTLSVerify:         true,
				expectError:           true,
				expectedErrorContains: "401",
			},
			{
				name:                  "https basic authentication with correct credentials and tls verification",
				username:              "testuser",
				password:              "testpass",
				skipTLSVerify:         false,
				expectError:           true,
				expectedErrorContains: "certificate",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &RemoteSourceConfig{
					URL:           server.GetHttpsAuthUrl("example.ovf"),
					Username:      tt.username,
					Password:      tt.password,
					SkipTlsVerify: tt.skipTLSVerify,
				}

				client := createHttpClientFromConfig(config)
				req, err := http.NewRequest("GET", config.URL, nil)
				if err != nil {
					t.Fatalf("failed to create request: %v", err)
				}

				if config.Username != "" && config.Password != "" {
					req.SetBasicAuth(config.Username, config.Password)
				}

				resp, err := client.Do(req)

				if tt.expectError {
					if err == nil && resp != nil && resp.StatusCode < 400 {
						t.Errorf("expected error but request succeeded")
						resp.Body.Close()
						return
					}
					if err != nil && !strings.Contains(err.Error(), tt.expectedErrorContains) {
						t.Errorf("expected error to contain '%s', got '%s'", tt.expectedErrorContains, err.Error())
					}
					if resp != nil && resp.StatusCode >= 400 && !strings.Contains(fmt.Sprintf("%d", resp.StatusCode), tt.expectedErrorContains) {
						t.Errorf("expected status to contain '%s', got %d", tt.expectedErrorContains, resp.StatusCode)
					}
					if resp != nil {
						resp.Body.Close()
					}
					return
				}

				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}

				if resp.StatusCode != http.StatusOK {
					t.Errorf("expected status 200, got %d", resp.StatusCode)
				}

				resp.Body.Close()
			})
		}
	})
}

// TestUrlCredentialHandling validates URL credential embedding and encoding.
func TestUrlCredentialHandling(t *testing.T) {
	server := NewHttpTestServer()
	defer server.Close()

	t.Run("Credential Validation", func(t *testing.T) {
		tests := []struct {
			name          string
			username      string
			password      string
			expectError   bool
			expectedError string
		}{
			{
				name:        "valid credentials pair",
				username:    "testuser",
				password:    "testpass",
				expectError: false,
			},
			{
				name:          "username without password",
				username:      "testuser",
				password:      "",
				expectError:   true,
				expectedError: "'password' is required when 'username' is specified",
			},
			{
				name:          "password without username",
				username:      "",
				password:      "testpass",
				expectError:   true,
				expectedError: "'username' is required when 'password' is specified",
			},
			{
				name:        "empty credentials (anonymous access)",
				username:    "",
				password:    "",
				expectError: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &RemoteSourceConfig{
					URL:      server.GetHttpAuthUrl("example.ovf"),
					Username: tt.username,
					Password: tt.password,
				}

				errs := validateRemoteSourceCredentials(config)
				if tt.expectError {
					if len(errs) == 0 {
						t.Error("expected validation error but got none")
					} else if !strings.Contains(errs[0].Error(), tt.expectedError) {
						t.Errorf("expected error to contain '%s', got '%s'", tt.expectedError, errs[0].Error())
					}
				} else {
					if len(errs) > 0 {
						t.Errorf("unexpected validation error: %v", errs[0])
					}
				}
			})
		}
	})

	t.Run("Special Characters in Credentials", func(t *testing.T) {
		tests := []struct {
			name     string
			username string
			password string
		}{
			{
				name:     "email-style username",
				username: "testuser@packages.example.com",
				password: "VMw@re1!",
			},
			{
				name:     "url encoded characters",
				username: "testuser%40packages.example.com",
				password: "testp%40ss",
			},
			{
				name:     "special characters",
				username: "testuser+",
				password: "testpass#$123",
			},
			{
				name:     "unicode characters",
				username: "testüser",
				password: "testpäss",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				baseURL := server.GetHttpAuthUrl("example.ovf")
				parsedURL, err := url.Parse(baseURL)
				if err != nil {
					t.Fatalf("failed to parse URL: %v", err)
				}

				parsedURL.User = url.UserPassword(tt.username, tt.password)
				embeddedURL := parsedURL.String()

				reparsedURL, err := url.Parse(embeddedURL)
				if err != nil {
					t.Errorf("failed to parse URL with embedded credentials: %v", err)
				}

				if reparsedURL.User != nil {
					extractedUsername := reparsedURL.User.Username()
					extractedPassword, _ := reparsedURL.User.Password()

					if extractedUsername != tt.username {
						decodedUsername, _ := url.QueryUnescape(extractedUsername)
						if decodedUsername != tt.username {
							t.Errorf("username mismatch: expected '%s', got '%s' (decoded: '%s')",
								tt.username, extractedUsername, decodedUsername)
						}
					}

					if extractedPassword != tt.password {
						decodedPassword, _ := url.QueryUnescape(extractedPassword)
						if decodedPassword != tt.password {
							t.Errorf("password mismatch: expected '%s', got '%s' (decoded: '%s')",
								tt.password, extractedPassword, decodedPassword)
						}
					}
				}

				config := &RemoteSourceConfig{
					URL:      baseURL,
					Username: tt.username,
					Password: tt.password,
				}

				errs := validateRemoteSourceCredentials(config)
				if len(errs) > 0 {
					t.Errorf("unexpected validation error for special characters: %v", errs[0])
				}
			})
		}
	})

	t.Run("URL Encoding and Decoding", func(t *testing.T) {
		tests := []struct {
			name         string
			rawUsername  string
			rawPassword  string
			encodedCheck bool
		}{
			{
				name:         "basic ascii characters",
				rawUsername:  "testuser",
				rawPassword:  "testpass",
				encodedCheck: false,
			},
			{
				name:         "characters requiring encoding",
				rawUsername:  "testuser@packages.example.com",
				rawPassword:  "VMw@re1!",
				encodedCheck: true,
			},
			{
				name:         "encoded characters",
				rawUsername:  "testuser%40packages.example.com",
				rawPassword:  "testp%40ss",
				encodedCheck: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				baseURL := server.GetHttpAuthUrl("example.ovf")
				parsedURL, err := url.Parse(baseURL)
				if err != nil {
					t.Fatalf("failed to parse URL: %v", err)
				}

				parsedURL.User = url.UserPassword(tt.rawUsername, tt.rawPassword)
				embeddedURL := parsedURL.String()

				_, err = url.Parse(embeddedURL)
				if err != nil {
					t.Errorf("Generated URL is invalid: %v", err)
				}

				if tt.encodedCheck && strings.Contains(tt.rawUsername, "@") {
					if !strings.Contains(embeddedURL, "%40") && !strings.Contains(embeddedURL, "@") {
						t.Errorf("expected @ character to be handled properly in URL")
					}
				}
			})
		}
	})
}

// TestUrlSanitization validates URL sanitization for logging and error messages.
func TestUrlSanitization(t *testing.T) {
	tests := []struct {
		name        string
		inputURL    string
		expectedURL string
	}{
		{
			name:        "url with embedded credentials",
			inputURL:    "https://testuser:testpass@packages.example.com/artifacts/example.ovf",
			expectedURL: "https://***:***@packages.example.com/artifacts/example.ovf",
		},
		{
			name:        "url without credentials",
			inputURL:    "https://packages.example.com/artifacts/example.ovf",
			expectedURL: "https://packages.example.com/artifacts/example.ovf",
		},
		{
			name:        "url with special characters in credentials",
			inputURL:    "https://testuser%40packages.example.com:testp%40ss@packages.example.com/artifacts/example.ovf",
			expectedURL: "https://***:***@packages.example.com/artifacts/example.ovf",
		},
		{
			name:        "http url with credentials",
			inputURL:    "http://testuser:testpass@packages.example.com/artifacts/example.ovf",
			expectedURL: "http://***:***@packages.example.com/artifacts/example.ovf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := sanitizeUrl(tt.inputURL)
			if sanitized != tt.expectedURL {
				t.Errorf("expected sanitized URL '%s', got '%s'", tt.expectedURL, sanitized)
			}
		})
	}
}

// TestTlsConfigurationIntegration validates tls configuration integration.
func TestTlsConfigurationIntegration(t *testing.T) {
	server := NewHttpTestServer()
	defer server.Close()

	t.Run("TLS Context Configuration", func(t *testing.T) {
		tests := []struct {
			name                 string
			skipTLSVerify        bool
			expectTLSConfig      bool
			expectedInsecureSkip bool
		}{
			{
				name:                 "default tls configuration: strict verification",
				skipTLSVerify:        false,
				expectTLSConfig:      false,
				expectedInsecureSkip: false,
			},
			{
				name:                 "relaxed tls configuration: skip verification",
				skipTLSVerify:        true,
				expectTLSConfig:      true,
				expectedInsecureSkip: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &RemoteSourceConfig{
					URL:           server.GetHttpsUrl("example.ovf"),
					SkipTlsVerify: tt.skipTLSVerify,
				}

				client := createHttpClientFromConfig(config)

				if transport, ok := client.Transport.(*http.Transport); ok {
					if tt.expectTLSConfig {
						if transport.TLSClientConfig == nil {
							t.Error("expected tls client config to be set")
						} else {
							if transport.TLSClientConfig.InsecureSkipVerify != tt.expectedInsecureSkip {
								t.Errorf("expected InsecureSkipVerify to be %v, got %v",
									tt.expectedInsecureSkip, transport.TLSClientConfig.InsecureSkipVerify)
							}

							tlsConfig := transport.TLSClientConfig
							if tlsConfig.InsecureSkipVerify != tt.skipTLSVerify {
								t.Errorf("tls config InsecureSkipVerify mismatch: expected %v, got %v",
									tt.skipTLSVerify, tlsConfig.InsecureSkipVerify)
							}
						}
					} else {
						if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
							t.Error("default client should not have InsecureSkipVerify enabled")
						}
					}
				} else {
					if tt.expectTLSConfig {
						t.Error("expected httl transport for relaxed tls configuration")
					}
				}
			})
		}
	})

	t.Run("Certificate Validation Scenarios", func(t *testing.T) {
		tests := []struct {
			name          string
			skipTLSVerify bool
			expectError   bool
			expectedError string
			description   string
		}{
			{
				name:          "valid certificate validation (simulated with self-signed rejection)",
				skipTLSVerify: false,
				expectError:   true,
				expectedError: "certificate",
				description:   "should reject self-signed certificates when verification is enabled",
			},
			{
				name:          "invalid certificate handling (self-signed accepted)",
				skipTLSVerify: true,
				expectError:   false,
				description:   "should accept self-signed certificates when verification is disabled",
			},
			{
				name:          "self-signed certificate rejection",
				skipTLSVerify: false,
				expectError:   true,
				expectedError: "certificate",
				description:   "should reject self-signed certificates",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &RemoteSourceConfig{
					URL:           server.GetHttpsUrl("example.ovf"),
					SkipTlsVerify: tt.skipTLSVerify,
				}

				client := createHttpClientFromConfig(config)

				req, err := http.NewRequest("GET", config.URL, nil)
				if err != nil {
					t.Fatalf("failed to create request: %v", err)
				}

				resp, err := client.Do(req)

				if tt.expectError {
					if err == nil {
						t.Errorf("expected tls error for %s but request succeeded", tt.description)
						if resp != nil {
							resp.Body.Close()
						}
					} else if tt.expectedError != "" && !strings.Contains(err.Error(), tt.expectedError) {
						t.Errorf("expected error to contain '%s' for %s, got '%s'",
							tt.expectedError, tt.description, err.Error())
					}
				} else {
					if err != nil {
						t.Errorf("unexpected error for %s: %v", tt.description, err)
					} else {
						if resp.StatusCode != http.StatusOK {
							t.Errorf("expected status 200 for %s, got %d", tt.description, resp.StatusCode)
						}
						resp.Body.Close()
					}
				}
			})
		}
	})

	t.Run("Skip tls Verification", func(t *testing.T) {
		tests := []struct {
			name          string
			skipTLSVerify bool
			expectSuccess bool
			description   string
		}{
			{
				name:          "skip tls verification (accepts self-signed certificates)",
				skipTLSVerify: true,
				expectSuccess: true,
				description:   "should connect with self-signed certificates when flag is enabled",
			},
			{
				name:          "tls verification (rejects self-signed certificates)",
				skipTLSVerify: false,
				expectSuccess: false,
				description:   "should reject self-signed certificates when flag is disabled",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &RemoteSourceConfig{
					URL:           server.GetHttpsUrl("example.ovf"),
					SkipTlsVerify: tt.skipTLSVerify,
				}

				client := createHttpClientFromConfig(config)
				req, err := http.NewRequest("GET", config.URL, nil)
				if err != nil {
					t.Fatalf("failed to create request: %v", err)
				}

				resp, err := client.Do(req)

				if tt.expectSuccess {
					if err != nil {
						t.Errorf("expected success for %s, got error: %v", tt.description, err)
					} else {
						if resp.StatusCode != http.StatusOK {
							t.Errorf("expected status 200 for %s, got %d", tt.description, resp.StatusCode)
						}
						resp.Body.Close()
					}
				} else {
					if err == nil {
						t.Errorf("expected tls error for %s but request succeeded", tt.description)
						if resp != nil {
							resp.Body.Close()
						}
					}
				}
			})
		}
	})

	t.Run("TLS Error Handling", func(t *testing.T) {
		tests := []struct {
			name             string
			skipTLSVerify    bool
			expectError      bool
			expectedKeywords []string
			forbiddenContent []string
			description      string
		}{
			{
				name:             "xertificate verification error messages",
				skipTLSVerify:    false,
				expectError:      true,
				expectedKeywords: []string{"certificate", "x509"},
				forbiddenContent: []string{"testuser", "testpass", "password"},
				description:      "should provide clear certificate error without exposing credentials",
			},
			{
				name:             "successful tls connection logging",
				skipTLSVerify:    true,
				expectError:      false,
				forbiddenContent: []string{"testuser", "testpass", "password"},
				description:      "should not expose credentials in successful connections",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &RemoteSourceConfig{
					URL:           server.GetHttpsUrl("example.ovf"),
					SkipTlsVerify: tt.skipTLSVerify,
					Username:      "testuser", // Add credentials to test they're not exposed
					Password:      "testpass",
				}

				client := createHttpClientFromConfig(config)
				req, err := http.NewRequest("GET", config.URL, nil)
				if err != nil {
					t.Fatalf("failed to create request: %v", err)
				}

				if config.Username != "" && config.Password != "" {
					req.SetBasicAuth(config.Username, config.Password)
				}

				resp, err := client.Do(req)

				if tt.expectError {
					if err == nil {
						t.Errorf("expected tls error for %s but request succeeded", tt.description)
						if resp != nil {
							resp.Body.Close()
						}
						return
					}

					errorMsg := err.Error()

					if len(tt.expectedKeywords) > 0 {
						found := false
						for _, keyword := range tt.expectedKeywords {
							if strings.Contains(strings.ToLower(errorMsg), keyword) {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("expected tls error message to contain one of %v for %s, got: %s",
								tt.expectedKeywords, tt.description, errorMsg)
						}
					}

					for _, forbidden := range tt.forbiddenContent {
						if strings.Contains(errorMsg, forbidden) {
							t.Errorf("tls error message should not expose '%s' for %s, got: %s",
								forbidden, tt.description, errorMsg)
						}
					}
				} else {
					if err != nil {
						t.Errorf("unexpected error for %s: %v", tt.description, err)
						return
					}

					if resp.StatusCode != http.StatusOK {
						t.Errorf("expected status 200 for %s, got %d", tt.description, resp.StatusCode)
					}
					resp.Body.Close()
				}
			})
		}
	})

	t.Run("TLS Configuration with Authentication", func(t *testing.T) {
		tests := []struct {
			name              string
			username          string
			password          string
			skipTLSVerify     bool
			expectError       bool
			expectedErrorType string
			description       string
		}{
			{
				name:          "https with valid credentials and tls skip",
				username:      "testuser",
				password:      "testpass",
				skipTLSVerify: true,
				expectError:   false,
				description:   "should succeed with correct credentials and tls verification disabled",
			},
			{
				name:              "https with invalid credentials and tls skip",
				username:          "wronguser",
				password:          "wrongpass",
				skipTLSVerify:     true,
				expectError:       true,
				expectedErrorType: "auth",
				description:       "should fail with authentication error even when tls verification is disabled",
			},
			{
				name:              "https with valid credentials but strict tls",
				username:          "testuser",
				password:          "testpass",
				skipTLSVerify:     false,
				expectError:       true,
				expectedErrorType: "certificate",
				description:       "should fail with certificate error before authentication when tls verification is enabled",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &RemoteSourceConfig{
					URL:           server.GetHttpsAuthUrl("example.ovf"),
					Username:      tt.username,
					Password:      tt.password,
					SkipTlsVerify: tt.skipTLSVerify,
				}

				client := createHttpClientFromConfig(config)
				req, err := http.NewRequest("GET", config.URL, nil)
				if err != nil {
					t.Fatalf("failed to create request: %v", err)
				}

				if config.Username != "" && config.Password != "" {
					req.SetBasicAuth(config.Username, config.Password)
				}

				resp, err := client.Do(req)

				if tt.expectError {
					if err == nil && resp != nil && resp.StatusCode < 400 {
						t.Errorf("expected error for %s but request succeeded", tt.description)
						resp.Body.Close()
						return
					}

					if err != nil {
						errorMsg := strings.ToLower(err.Error())
						switch tt.expectedErrorType {
						case "certificate":
							if !strings.Contains(errorMsg, "certificate") && !strings.Contains(errorMsg, "x509") {
								t.Errorf("expected certificate error for %s, got: %v", tt.description, err)
							}
						case "auth":
							if resp == nil {
								t.Errorf("expected http auth error for %s, got connection error: %v", tt.description, err)
							}
						}
					}

					if resp != nil {
						if tt.expectedErrorType == "auth" && resp.StatusCode != http.StatusUnauthorized {
							t.Errorf("expected 401 Unauthorized for %s, got %d", tt.description, resp.StatusCode)
						}
						resp.Body.Close()
					}
				} else {
					if err != nil {
						t.Errorf("unexpected error for %s: %v", tt.description, err)
						return
					}

					if resp.StatusCode != http.StatusOK {
						t.Errorf("expected status 200 for %s, got %d", tt.description, resp.StatusCode)
					}
					resp.Body.Close()
				}
			})
		}
	})
}

// TestHttpClientErrorScenarios validates various HTTP client error scenarios.
func TestHttpClientErrorScenarios(t *testing.T) {
	server := NewHttpTestServer()
	defer server.Close()

	t.Run("HTTP Error Status Codes", func(t *testing.T) {
		tests := []struct {
			name           string
			endpoint       string
			expectedStatus int
		}{
			{
				name:           "404 Not Found",
				endpoint:       "/error/404",
				expectedStatus: http.StatusNotFound,
			},
			{
				name:           "500 Internal Server Error",
				endpoint:       "/error/500",
				expectedStatus: http.StatusInternalServerError,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &RemoteSourceConfig{
					URL: server.HttpServer.URL + tt.endpoint,
				}

				client := createHttpClientFromConfig(config)
				req, err := http.NewRequest("GET", config.URL, nil)
				if err != nil {
					t.Fatalf("failed to create request: %v", err)
				}

				resp, err := client.Do(req)
				if err != nil {
					t.Errorf("unexpected network error: %v", err)
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != tt.expectedStatus {
					t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
				}
			})
		}
	})

	t.Run("Network Connectivity Errors", func(t *testing.T) {
		tests := []struct {
			name        string
			url         string
			expectError bool
		}{
			{
				name:        "invalid hostname",
				url:         "http://nonexistent.invalid.domain.test/example.ovf",
				expectError: true,
			},
			{
				name:        "invalid port",
				url:         "http://localhost:99999/example.ovf",
				expectError: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &RemoteSourceConfig{
					URL: tt.url,
				}

				client := createHttpClientFromConfig(config)
				client.Timeout = 2 * time.Second // Short timeout for faster test.

				req, err := http.NewRequest("GET", config.URL, nil)
				if err != nil {
					t.Fatalf("failed to create request: %v", err)
				}

				_, err = client.Do(req)
				if tt.expectError {
					if err == nil {
						t.Error("expected network error but request succeeded")
					}
				} else {
					if err != nil {
						t.Errorf("unexpected error: %v", err)
					}
				}
			})
		}
	})

	t.Run("Authentication Error Messages", func(t *testing.T) {
		config := &RemoteSourceConfig{
			URL:      server.GetHttpAuthUrl("example.ovf"),
			Username: "wronguser",
			Password: "wrongpass",
		}

		client := createHttpClientFromConfig(config)
		req, err := http.NewRequest("GET", config.URL, nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		req.SetBasicAuth(config.Username, config.Password)

		resp, err := client.Do(req)
		if err != nil {
			t.Errorf("unexpected network error: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401 Unauthorized, got %d", resp.StatusCode)
		}

		// Verify WWW-Authenticate header is present.
		authHeader := resp.Header.Get("WWW-Authenticate")
		if authHeader == "" {
			t.Error("expected WWW-Authenticate header in 401 response")
		}
	})
}

// TestTlsContextCreation validates tls context configuration and creation.
func TestTlsContextCreation(t *testing.T) {
	server := NewHttpTestServer()
	defer server.Close()

	t.Run("TLS Context Creation", func(t *testing.T) {
		tests := []struct {
			name            string
			skipTLSVerify   bool
			expectCustomTLS bool
			description     string
		}{
			{
				name:            "default tls configuration: strict verification",
				skipTLSVerify:   false,
				expectCustomTLS: false,
				description:     "should use default tls settings when verification is enabled",
			},
			{
				name:            "relaxed tls configuration: skip verification",
				skipTLSVerify:   true,
				expectCustomTLS: true,
				description:     "should create relaxed tls context when verification is disabled",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &RemoteSourceConfig{
					URL:           server.GetHttpsUrl("example.ovf"),
					SkipTlsVerify: tt.skipTLSVerify,
				}

				tlsConfig := createTlsConfig(config)

				if tt.expectCustomTLS {
					if tlsConfig == nil {
						t.Errorf("expected relaxed tls config for %s", tt.description)
					} else {
						if tlsConfig.InsecureSkipVerify != tt.skipTLSVerify {
							t.Errorf("expected InsecureSkipVerify=%v for tls config for %s, got %v",
								tt.skipTLSVerify, tt.description, tlsConfig.InsecureSkipVerify)
						}
					}
				} else {
					// For default configuration, we might still have a tls config but InsecureSkipVerify should be false
					if tlsConfig != nil && tlsConfig.InsecureSkipVerify {
						t.Errorf("default tls config should not have InsecureSkipVerify enabled for %s", tt.description)
					}
				}

				client := &http.Client{
					Timeout: 5 * time.Second,
				}

				if tlsConfig != nil {
					client.Transport = &http.Transport{
						TLSClientConfig: tlsConfig,
					}
				}

				req, err := http.NewRequest("GET", config.URL, nil)
				if err != nil {
					t.Fatalf("failed to create request for %s: %v", tt.description, err)
				}

				resp, err := client.Do(req)

				if tt.skipTLSVerify {
					if err != nil {
						t.Errorf("expected success with InsecureSkipVerify for %s, got error: %v", tt.description, err)
					} else {
						if resp.StatusCode != http.StatusOK {
							t.Errorf("expected status 200 for %s, got %d", tt.description, resp.StatusCode)
						}
						resp.Body.Close()
					}
				} else {
					if err == nil {
						t.Errorf("expected certificate error for %s but request succeeded", tt.description)
						if resp != nil {
							resp.Body.Close()
						}
					}
				}
			})
		}
	})

	t.Run("TLS Configuration Validation", func(t *testing.T) {
		tests := []struct {
			name             string
			skipTLSVerify    bool
			expectedBehavior string
		}{
			{
				name:             "strict tls verification",
				skipTLSVerify:    false,
				expectedBehavior: "reject_self_signed",
			},
			{
				name:             "relaxed tls verification",
				skipTLSVerify:    true,
				expectedBehavior: "accept_self_signed",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &RemoteSourceConfig{
					URL:           server.GetHttpsUrl("example.ovf"),
					SkipTlsVerify: tt.skipTLSVerify,
				}

				tlsConfig := createTlsConfig(config)
				client := &http.Client{Timeout: 5 * time.Second}

				if tlsConfig != nil {
					client.Transport = &http.Transport{TLSClientConfig: tlsConfig}
				}

				req, err := http.NewRequest("GET", config.URL, nil)
				if err != nil {
					t.Fatalf("failed to create request: %v", err)
				}

				resp, err := client.Do(req)

				switch tt.expectedBehavior {
				case "accept_self_signed":
					if err != nil {
						t.Errorf("expected to accept self-signed certificate, got error: %v", err)
					} else {
						resp.Body.Close()
					}
				case "reject_self_signed":
					if err == nil {
						t.Error("expected to reject self-signed certificate, but request succeeded")
						if resp != nil {
							resp.Body.Close()
						}
					}
				}
			})
		}
	})
}

// createHttpClientFromConfig creates an HTTP client with tls configuration from RemoteSourceConfig.
func createHttpClientFromConfig(config *RemoteSourceConfig) *http.Client {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	if config.SkipTlsVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	return client
}

// createTlsConfig creates a tls configuration based on the remote source config.
func createTlsConfig(config *RemoteSourceConfig) *tls.Config {
	if config.SkipTlsVerify {
		return &tls.Config{
			InsecureSkipVerify: true,
		}
	}
	return nil
}

// validateRemoteSourceCredentials validates that username and password are provided together.
func validateRemoteSourceCredentials(config *RemoteSourceConfig) []error {
	var errs []error

	if config.Username != "" && config.Password == "" {
		errs = append(errs, fmt.Errorf("'password' is required when 'username' is specified for remote source"))
	}

	if config.Password != "" && config.Username == "" {
		errs = append(errs, fmt.Errorf("'username' is required when 'password' is specified for remote source"))
	}

	return errs
}

// sanitizeUrl masks credentials in URLs for safe logging.
func sanitizeUrl(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	if parsedURL.User != nil {
		scheme := parsedURL.Scheme
		host := parsedURL.Host
		path := parsedURL.Path
		query := parsedURL.RawQuery
		fragment := parsedURL.Fragment

		result := scheme + "://***:***@" + host + path
		if query != "" {
			result += "?" + query
		}
		if fragment != "" {
			result += "#" + fragment
		}
		return result
	}

	return parsedURL.String()
}
