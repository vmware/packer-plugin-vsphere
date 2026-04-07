// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import "testing"

func TestResourcePoolAcc(t *testing.T) {
	t.Skip("Acceptance tests not configured yet.")
	d := newTestDriver(t)
	p, err := d.FindResourcePool("", "esx01.example.com", "pool1/pool2")
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	path, err := p.Path()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	if path != "pool1/pool2" {
		t.Errorf("unexpected result: expected: '%s', but returned: '%s'", "pool1/pool2", path)
	}
}
