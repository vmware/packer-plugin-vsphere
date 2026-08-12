// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package acceptance

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/packer-plugin-vsphere/testing/env"
)

// FileArtifact is a minimal packersdk.Artifact whose Files() include a local
// .ova/.ovf/.vmx path for post-processor acceptance tests.
type FileArtifact struct {
	Path      string
	BuilderID string
}

func (a *FileArtifact) BuilderId() string { return a.BuilderID }
func (a *FileArtifact) Files() []string   { return []string{a.Path} }
func (a *FileArtifact) Id() string        { return a.Path }
func (a *FileArtifact) String() string    { return a.Path }
func (a *FileArtifact) State(string) any  { return nil }
func (a *FileArtifact) Destroy() error    { return nil }

// RequireOVAURL skips the test unless VSPHERE_OVA_URL is set.
func RequireOVAURL(t *testing.T, acc env.AccConfig) {
	t.Helper()
	if acc.OVAURL == "" {
		t.Skipf("set %s to an HTTPS .ova URL to run this ACC row", env.OVAURL)
	}
}

// RequireOvftool skips the test unless ovftool is on PATH.
func RequireOvftool(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ovftool"); err != nil {
		t.Skip("ovftool not found on PATH; required for vsphere post-processor ACC")
	}
}

// DownloadOVA downloads VSPHERE_OVA_URL to a temp file and returns its path
// plus a cleanup func.
func DownloadOVA(t *testing.T, acc env.AccConfig) (ovaPath string, cleanup func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "packer-acc-ova-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	ovaPath = filepath.Join(dir, "source.ova")
	DownloadURLToFile(t, acc, acc.OVAURL, ovaPath)
	return ovaPath, func() { _ = os.RemoveAll(dir) }
}

// DownloadURLToFile GETs rawURL to destPath using optional AccConfig auth/TLS.
func DownloadURLToFile(t *testing.T, acc env.AccConfig, rawURL, destPath string) {
	t.Helper()
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: acc.OVFSkipTLSVerify}, //nolint:gosec
		},
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatalf("build download request: %v", err)
	}
	if acc.OVFUsername != "" || acc.OVFPassword != "" {
		req.SetBasicAuth(acc.OVFUsername, acc.OVFPassword)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("download %s: %v", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download %s: HTTP %d", rawURL, resp.StatusCode)
	}
	f, err := os.Create(destPath)
	if err != nil {
		t.Fatalf("create %s: %v", destPath, err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		t.Fatalf("write %s: %v", destPath, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close %s: %v", destPath, err)
	}
}

// BasicUI returns a packersdk UI that buffers stdout/stderr for ACC.
func BasicUI() packersdk.Ui {
	return &packersdk.BasicUi{
		Reader:      os.Stdin,
		Writer:      io.Discard,
		ErrorWriter: io.Discard,
	}
}

// CheckVMExists asserts the named VM is findable via TestConn.
func CheckVMExists(name string) error {
	d, err := TestConn()
	if err != nil {
		return err
	}
	vm, err := d.FindVM(name)
	if err != nil {
		return fmt.Errorf("cannot find VM %q: %v", name, err)
	}
	info, err := vm.Info("name", "config.template")
	if err != nil {
		return fmt.Errorf("cannot read VM %q: %v", name, err)
	}
	if info.Name != name {
		return fmt.Errorf("unexpected VM name: expected %q, got %q", name, info.Name)
	}
	return nil
}

// CheckVMIsTemplate asserts the named inventory object is a template.
func CheckVMIsTemplate(name string) error {
	d, err := TestConn()
	if err != nil {
		return err
	}
	vm, err := d.FindVM(name)
	if err != nil {
		return fmt.Errorf("cannot find VM %q: %v", name, err)
	}
	isTemplate, err := vm.IsTemplate()
	if err != nil {
		return fmt.Errorf("IsTemplate %q: %v", name, err)
	}
	if !isTemplate {
		return fmt.Errorf("expected %q to be a template", name)
	}
	return nil
}
