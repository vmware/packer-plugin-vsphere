// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"archive/tar"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalOvfArchive_OpenOVF(t *testing.T) {
	dir := t.TempDir()
	ovfPath := filepath.Join(dir, "example.ovf")
	diskPath := filepath.Join(dir, "disk.vmdk")

	if err := os.WriteFile(ovfPath, []byte("<Envelope/>"), 0o644); err != nil {
		t.Fatalf("write ovf: %s", err)
	}
	if err := os.WriteFile(diskPath, []byte("vmdk"), 0o644); err != nil {
		t.Fatalf("write disk: %s", err)
	}

	archive, err := newLocalOvfArchive(ovfPath)
	if err != nil {
		t.Fatalf("newLocalOvfArchive: %s", err)
	}

	rc, size, err := archive.Open("")
	if err != nil {
		t.Fatalf("Open descriptor: %s", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read descriptor: %s", err)
	}
	if string(data) != "<Envelope/>" {
		t.Fatalf("unexpected descriptor contents: %q", data)
	}
	if size != int64(len(data)) {
		t.Fatalf("unexpected size: got %d want %d", size, len(data))
	}

	disk, size, err := archive.Open("disk.vmdk")
	if err != nil {
		t.Fatalf("Open sibling: %s", err)
	}
	defer disk.Close()
	diskData, err := io.ReadAll(disk)
	if err != nil {
		t.Fatalf("read sibling: %s", err)
	}
	if string(diskData) != "vmdk" {
		t.Fatalf("unexpected sibling contents: %q", diskData)
	}
	if size != int64(len(diskData)) {
		t.Fatalf("unexpected sibling size: got %d want %d", size, len(diskData))
	}

	_, _, err = archive.Open("missing.vmdk")
	if err == nil {
		t.Fatal("expected error for missing sibling")
	}
}

func TestLocalOvfArchive_OpenOVA(t *testing.T) {
	dir := t.TempDir()
	ovaPath := filepath.Join(dir, "example.ova")

	f, err := os.Create(ovaPath)
	if err != nil {
		t.Fatalf("create ova: %s", err)
	}
	tw := tar.NewWriter(f)
	files := map[string]string{
		"example.ovf": "<Envelope/>",
		"disk.vmdk":   "vmdk-data",
	}
	for name, contents := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(contents)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header: %s", err)
		}
		if _, err := tw.Write([]byte(contents)); err != nil {
			t.Fatalf("write body: %s", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %s", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close ova: %s", err)
	}

	archive, err := newLocalOvfArchive(ovaPath)
	if err != nil {
		t.Fatalf("newLocalOvfArchive: %s", err)
	}

	rc, _, err := archive.Open("*.ovf")
	if err != nil {
		t.Fatalf("Open ovf from ova: %s", err)
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("read ovf: %s", err)
	}
	if string(data) != "<Envelope/>" {
		t.Fatalf("unexpected ovf contents: %q", data)
	}

	disk, _, err := archive.Open("disk.vmdk")
	if err != nil {
		t.Fatalf("Open disk from ova: %s", err)
	}
	diskData, err := io.ReadAll(disk)
	_ = disk.Close()
	if err != nil {
		t.Fatalf("read disk: %s", err)
	}
	if string(diskData) != "vmdk-data" {
		t.Fatalf("unexpected disk contents: %q", diskData)
	}

	_, _, err = archive.Open("missing.vmdk")
	if !os.IsNotExist(err) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestNewOvfArchive_SelectsImplementation(t *testing.T) {
	dir := t.TempDir()
	ovfPath := filepath.Join(dir, "example.ovf")
	if err := os.WriteFile(ovfPath, []byte("<Envelope/>"), 0o644); err != nil {
		t.Fatalf("write ovf: %s", err)
	}

	local, err := newOvfArchive(&OvfDeployConfig{Path: ovfPath})
	if err != nil {
		t.Fatalf("local archive: %s", err)
	}
	if _, ok := local.(*localOvfArchive); !ok {
		t.Fatalf("expected *localOvfArchive, got %T", local)
	}

	remote, err := newOvfArchive(&OvfDeployConfig{URL: "https://packages.example.com/artifacts/example.ovf"})
	if err != nil {
		t.Fatalf("remote archive: %s", err)
	}
	if _, ok := remote.(*remoteOvfArchive); !ok {
		t.Fatalf("expected *remoteOvfArchive, got %T", remote)
	}

	_, err = newOvfArchive(&OvfDeployConfig{
		URL:  "https://packages.example.com/artifacts/example.ovf",
		Path: ovfPath,
	})
	if err == nil || !strings.Contains(err.Error(), "both URL and Path") {
		t.Fatalf("expected exclusivity error, got %v", err)
	}

	_, err = newOvfArchive(&OvfDeployConfig{})
	if err == nil || !strings.Contains(err.Error(), "requires either URL or Path") {
		t.Fatalf("expected missing source error, got %v", err)
	}
}

// TestRemoteOvfArchive_AuthURL tests that newRemoteOvfArchive embeds
// authentication credentials into the raw URL correctly.
func TestRemoteOvfArchive_AuthURL(t *testing.T) {
	tests := []struct {
		name        string
		originalURL string
		auth        *OvfAuthConfig
		expectedURL string
		expectError bool
	}{
		{
			name:        "No authentication",
			originalURL: "https://packages.example.com/artifacts/example.ovf",
			auth:        nil,
			expectedURL: "https://packages.example.com/artifacts/example.ovf",
			expectError: false,
		},
		{
			name:        "Empty authentication",
			originalURL: "https://packages.example.com/artifacts/example.ovf",
			auth:        &OvfAuthConfig{},
			expectedURL: "https://packages.example.com/artifacts/example.ovf",
			expectError: false,
		},
		{
			name:        "Basic authentication",
			originalURL: "https://packages.example.com/artifacts/example.ovf",
			auth: &OvfAuthConfig{
				Username: "testuser",
				Password: "testpass",
			},
			expectedURL: "https://testuser:testpass@packages.example.com/artifacts/example.ovf",
			expectError: false,
		},
		{
			name:        "Invalid URL",
			originalURL: "://invalid-url",
			auth: &OvfAuthConfig{
				Username: "testuser",
				Password: "testpass",
			},
			expectedURL: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archive, err := newRemoteOvfArchive(tt.originalURL, tt.auth, false)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %s", err)
				return
			}
			if archive.rawURL != tt.expectedURL {
				t.Errorf("expected rawURL '%s', got '%s'", tt.expectedURL, archive.rawURL)
			}
		})
	}
}

func TestHttpOvfFetchError(t *testing.T) {
	const fileURL = "https://user:secret@packages.example.com/artifacts/example.ovf"
	const sanitized = "https://packages.example.com/artifacts/example.ovf"

	tests := []struct {
		name           string
		statusCode     int
		wantContains   []string
		wantNotContain string
	}{
		{
			name:         "unauthorized",
			statusCode:   http.StatusUnauthorized,
			wantContains: []string{"authentication failed", "(HTTP 401)", sanitized},
		},
		{
			name:         "forbidden",
			statusCode:   http.StatusForbidden,
			wantContains: []string{"access denied", "(HTTP 403)", sanitized},
		},
		{
			name:         "not found",
			statusCode:   http.StatusNotFound,
			wantContains: []string{"OVF/OVA file not found", "(HTTP 404)", sanitized},
		},
		{
			name:         "gone",
			statusCode:   http.StatusGone,
			wantContains: []string{"no longer available", "(HTTP 410)", sanitized},
		},
		{
			name:         "rate limited",
			statusCode:   http.StatusTooManyRequests,
			wantContains: []string{"rate-limited", "(HTTP 429)", sanitized},
		},
		{
			name:         "server error",
			statusCode:   http.StatusInternalServerError,
			wantContains: []string{"remote server error", "(HTTP 500)", sanitized},
		},
		{
			name:         "unexpected client error",
			statusCode:   http.StatusBadRequest,
			wantContains: []string{"unexpected HTTP 400", sanitized},
		},
		{
			name:           "strips credentials from URL",
			statusCode:     http.StatusNotFound,
			wantContains:   []string{sanitized},
			wantNotContain: "secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := httpOvfFetchError(tt.statusCode, fileURL)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			msg := err.Error()
			for _, want := range tt.wantContains {
				if !strings.Contains(msg, want) {
					t.Errorf("error %q missing %q", msg, want)
				}
			}
			if tt.wantNotContain != "" && strings.Contains(msg, tt.wantNotContain) {
				t.Errorf("error %q must not contain %q", msg, tt.wantNotContain)
			}
		})
	}
}

func TestRemoteOvfArchive_OpenHTTPStatusErrors(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		wantContains string
	}{
		{
			name:         "unauthorized",
			statusCode:   http.StatusUnauthorized,
			wantContains: "authentication failed",
		},
		{
			name:         "not found",
			statusCode:   http.StatusNotFound,
			wantContains: "OVF/OVA file not found",
		},
		{
			name:         "forbidden",
			statusCode:   http.StatusForbidden,
			wantContains: "access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			archive, err := newRemoteOvfArchive(server.URL+"/artifacts/example.ovf", nil, false)
			if err != nil {
				t.Fatalf("unexpected error creating archive: %s", err)
			}

			_, _, err = archive.Open("")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantContains) {
				t.Errorf("error %q missing %q", err.Error(), tt.wantContains)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("(HTTP %d)", tt.statusCode)) {
				t.Errorf("error %q missing HTTP status %d", err.Error(), tt.statusCode)
			}
		})
	}
}
