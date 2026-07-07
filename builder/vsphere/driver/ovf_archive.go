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
	"strings"
)

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

// openFromTar streams the OVA TAR and returns the first entry whose base name
// matches the glob pattern given by name.
func (a *remoteOvfArchive) openFromTar(name string) (io.ReadCloser, int64, error) {
	resp, err := a.httpClient.Get(a.rawURL)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch OVA from remote source: %s", SanitizeOvfErrorMessage(err.Error()))
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, 0, fmt.Errorf("remote OVA source returned HTTP %d", resp.StatusCode)
	}

	tr := tar.NewReader(resp.Body)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			_ = resp.Body.Close()
			return nil, 0, os.ErrNotExist
		}
		if err != nil {
			_ = resp.Body.Close()
			return nil, 0, fmt.Errorf("error reading OVA archive: %s", err)
		}

		matched, matchErr := path.Match(name, path.Base(hdr.Name))
		if matchErr != nil {
			_ = resp.Body.Close()
			return nil, 0, fmt.Errorf("invalid glob pattern '%s': %s", name, matchErr)
		}
		if matched {
			return &ovaEntry{Reader: tr, closer: resp.Body}, hdr.Size, nil
		}
	}
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
		return nil, 0, fmt.Errorf("remote server returned HTTP %d for '%s'", resp.StatusCode, SanitizeOvfURL(fileURL))
	}

	return resp.Body, resp.ContentLength, nil
}

// ovaEntry wraps a TAR entry reader, closing the underlying HTTP response when done.
type ovaEntry struct {
	io.Reader
	closer io.Closer
}

func (e *ovaEntry) Close() error {
	return e.closer.Close()
}
