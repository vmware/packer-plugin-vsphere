// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package acceptance

import (
	"os"
	"testing"

	"github.com/vmware/packer-plugin-vsphere/testing/env"
)

// RequireAcceptance skips the test unless PACKER_ACC is set. Call at the start
// of live acceptance tests that need a real vSphere environment.
func RequireAcceptance(t *testing.T) {
	t.Helper()
	if os.Getenv(env.EnvPackerAcc) == "" {
		t.Skipf("Acceptance tests skipped: set %s=1 to run against a live environment (see testing/acceptance/acc_config.md)", env.EnvPackerAcc)
	}
}

// AccFromEnv returns the shared acceptance inventory configuration. It is a
// thin wrapper around env.AccFromEnv for callers that already import this
// package.
func AccFromEnv() env.AccConfig {
	return env.AccFromEnv()
}
