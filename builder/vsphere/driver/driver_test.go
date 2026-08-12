// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"fmt"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/vmware/packer-plugin-vsphere/testing/env"
)

// testUI provides a simple UI implementation for testing.
type testUI struct{}

func (ui *testUI) Ask(string) (string, error)                                      { return "", nil }
func (ui *testUI) Askf(format string, args ...any) (string, error)                 { return "", nil }
func (ui *testUI) Say(message string)                                              {}
func (ui *testUI) Sayf(format string, args ...any)                                 {}
func (ui *testUI) Message(message string)                                          {}
func (ui *testUI) Messagef(format string, args ...any)                             {}
func (ui *testUI) Error(message string)                                            {}
func (ui *testUI) Errorf(format string, args ...any)                               {}
func (ui *testUI) Machine(string, ...string)                                       {}
func (ui *testUI) TrackProgress(string, int64, int64, io.ReadCloser) io.ReadCloser { return nil }

// newTestDriver creates a new driver instance for testing against a live endpoint.
func newTestDriver(t *testing.T) Driver {
	vcenter := env.GetenvOrDefault(env.VcenterServer, env.DefaultVcenterServer)
	username := env.GetenvOrDefault(env.VsphereUsername, env.DefaultVsphereUsername)
	password := env.GetenvOrDefault(env.VspherePassword, env.DefaultVspherePassword)

	d, err := NewDriver(&ConnectConfig{
		VCenterServer:      vcenter,
		Username:           username,
		Password:           password,
		InsecureConnection: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	return d
}

// newVMName generates a random VM name for testing.
func newVMName() string {
	r := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
	return fmt.Sprintf("test-%v", r.Intn(1000))
}
