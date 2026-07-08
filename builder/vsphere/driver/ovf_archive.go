// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"archive/tar"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ovfArchive opens named files from an OVF/OVA source on the Packer host.
type ovfArchive interface {
	Open(name string) (io.ReadCloser, int64, error)
}

// newOvfArchive selects a remote or local archive implementation based on
// OvfDeployConfig. Exactly one of URL or Path must be set.
func newOvfArchive(config *OvfDeployConfig) (ovfArchive, error) {
	if config == nil {
		return nil, fmt.Errorf("OVF deployment configuration cannot be nil")
	}
	if config.URL != "" && config.Path != "" {
		return nil, fmt.Errorf("OVF source cannot specify both URL and Path")
	}
	if config.URL != "" {
		return newRemoteOvfArchive(config.URL, config.Authentication, config.SkipTlsVerify)
	}
	if config.Path != "" {
		return newLocalOvfArchive(config.Path)
	}
	return nil, fmt.Errorf("OVF source requires either URL or Path")
}

// isOvfSourceOVA reports whether the configured source is an OVA archive.
func isOvfSourceOVA(config *OvfDeployConfig) bool {
	if config == nil {
		return false
	}
	if config.Path != "" {
		return strings.HasSuffix(strings.ToLower(config.Path), ".ova")
	}
	if config.URL != "" {
		u, err := url.Parse(config.URL)
		if err != nil {
			return false
		}
		return strings.HasSuffix(strings.ToLower(u.Path), ".ova")
	}
	return false
}

// remoteOvfArchive fetches remote OVF/OVA content over HTTP(S) on the Packer
// host. It handles both OVA (TAR archive) and OVF (multi-file) layouts.
type remoteOvfArchive struct {
	rawURL     string
	httpClient *http.Client
	isOva      bool
}

// newRemoteOvfArchive constructs an archive for the given remote OVF/OVA URL.
// TLS verification and HTTP basic authentication are configured from the supplied
// arguments.
func newRemoteOvfArchive(rawURL string, auth *OvfAuthConfig, skipTLSVerify bool) (*remoteOvfArchive, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipTLSVerify, //nolint:gosec
		},
	}
	client := &http.Client{Transport: transport}

	authenticatedURL := rawURL
	if auth != nil && auth.Username != "" && auth.Password != "" {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("invalid OVF URL: %s", err)
		}
		u.User = url.UserPassword(auth.Username, auth.Password)
		authenticatedURL = u.String()
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid OVF URL: %s", err)
	}
	isOva := strings.HasSuffix(strings.ToLower(u.Path), ".ova")

	return &remoteOvfArchive{
		rawURL:     authenticatedURL,
		httpClient: client,
		isOva:      isOva,
	}, nil
}

// Open fetches a named file from the remote OVF/OVA source.
func (a *remoteOvfArchive) Open(name string) (io.ReadCloser, int64, error) {
	if a.isOva {
		return a.openFromTar(name)
	}
	return a.openFromHTTP(name)
}

// httpOvfFetchError maps non-OK HTTP responses to actionable errors for remote
// OVF/OVA fetches.
func httpOvfFetchError(statusCode int, fileURL string) error {
	sanitizedURL := SanitizeOvfURL(fileURL)
	switch statusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed for '%s' (HTTP 401); verify username and password", sanitizedURL)
	case http.StatusForbidden:
		return fmt.Errorf("access denied for '%s' (HTTP 403); verify credentials and server permissions", sanitizedURL)
	case http.StatusNotFound:
		return fmt.Errorf("OVF/OVA file not found at '%s' (HTTP 404); verify the URL is correct", sanitizedURL)
	case http.StatusGone:
		return fmt.Errorf("OVF/OVA file no longer available at '%s' (HTTP 410)", sanitizedURL)
	case http.StatusTooManyRequests:
		return fmt.Errorf("remote server rate-limited the request for '%s' (HTTP 429); retry later", sanitizedURL)
	default:
		if statusCode >= http.StatusInternalServerError {
			return fmt.Errorf("remote server error fetching '%s' (HTTP %d)", sanitizedURL, statusCode)
		}
		return fmt.Errorf("unexpected HTTP %d fetching '%s'", statusCode, sanitizedURL)
	}
}

// openFromTar streams the OVA TAR and returns the first entry whose base name
// matches the glob pattern given by name.
func (a *remoteOvfArchive) openFromTar(name string) (io.ReadCloser, int64, error) {
	resp, err := a.httpClient.Get(a.rawURL)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch OVA from remote source: %s", SanitizeOvfErrorMessage(err.Error()))
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, 0, httpOvfFetchError(resp.StatusCode, a.rawURL)
	}

	return openTarEntry(resp.Body, name)
}

// openFromHTTP fetches a file from the OVF HTTP source.
func (a *remoteOvfArchive) openFromHTTP(name string) (io.ReadCloser, int64, error) {
	fileURL := a.rawURL
	if name != "" && !strings.HasPrefix(name, "http://") && !strings.HasPrefix(name, "https://") {
		// Resolve relative file name against the directory containing the OVF.
		idx := strings.LastIndex(a.rawURL, "/")
		if idx != -1 {
			fileURL = a.rawURL[:idx+1] + name
		}
	}

	resp, err := a.httpClient.Get(fileURL)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch '%s': %s", SanitizeOvfURL(fileURL), SanitizeOvfErrorMessage(err.Error()))
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, 0, httpOvfFetchError(resp.StatusCode, fileURL)
	}

	return resp.Body, resp.ContentLength, nil
}

// localOvfArchive opens OVF/OVA content from the local filesystem.
type localOvfArchive struct {
	path  string
	isOva bool
}

func newLocalOvfArchive(localPath string) (*localOvfArchive, error) {
	cleaned := filepath.Clean(localPath)
	if cleaned == "" || cleaned == "." {
		return nil, fmt.Errorf("local OVF/OVA path is required")
	}
	return &localOvfArchive{
		path:  cleaned,
		isOva: strings.HasSuffix(strings.ToLower(cleaned), ".ova"),
	}, nil
}

// Open opens a named file from the local OVF/OVA source.
func (a *localOvfArchive) Open(name string) (io.ReadCloser, int64, error) {
	if a.isOva {
		return a.openFromTar(name)
	}
	return a.openFromFile(name)
}

func (a *localOvfArchive) openFromTar(name string) (io.ReadCloser, int64, error) {
	f, err := os.Open(a.path)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open local OVA '%s': %s", a.path, err)
	}
	return openTarEntry(f, name)
}

func (a *localOvfArchive) openFromFile(name string) (io.ReadCloser, int64, error) {
	filePath := a.path
	if name != "" && name != a.path && !filepath.IsAbs(name) {
		filePath = filepath.Join(filepath.Dir(a.path), name)
	}

	f, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open local file '%s': %s", filePath, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, fmt.Errorf("failed to stat local file '%s': %s", filePath, err)
	}
	return f, info.Size(), nil
}

// openTarEntry streams a TAR and returns the first entry whose base name
// matches the glob pattern given by name. closer is closed when the entry is closed
// or when no matching entry is found.
func openTarEntry(closer io.ReadCloser, name string) (io.ReadCloser, int64, error) {
	tr := tar.NewReader(closer)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			_ = closer.Close()
			return nil, 0, os.ErrNotExist
		}
		if err != nil {
			_ = closer.Close()
			return nil, 0, fmt.Errorf("error reading OVA archive: %s", err)
		}

		matched, matchErr := path.Match(name, path.Base(hdr.Name))
		if matchErr != nil {
			_ = closer.Close()
			return nil, 0, fmt.Errorf("invalid glob pattern '%s': %s", name, matchErr)
		}
		if matched {
			return &ovaEntry{Reader: tr, closer: closer}, hdr.Size, nil
		}
	}
}

// ovaEntry wraps a TAR entry reader, closing the underlying source when done.
type ovaEntry struct {
	io.Reader
	closer io.Closer
}

func (e *ovaEntry) Close() error {
	return e.closer.Close()
}
