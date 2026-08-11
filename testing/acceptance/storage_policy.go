// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package acceptance

import (
	"fmt"

	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

// CheckStoragePolicyDiskPlacements verifies that the disks on the virtual
// machine have the expected storage policy names (in order) and non-empty
// datastore backings. When at least two distinct policy names are configured,
// at least two distinct datastores among those disks are required so per-disk
// placement is observable.
//
// - For the -iso builder, policyCount should equal the total disk count.
// - For the -clone builder, policyCount should equal the number of added disks.
func CheckStoragePolicyDiskPlacements(d driver.Driver, vm driver.VirtualMachine, wantPolicies []string) error {
	if len(wantPolicies) == 0 {
		return fmt.Errorf("expected at least one storage policy")
	}

	vc, ok := d.(*driver.VCenterDriver)
	if !ok {
		return fmt.Errorf("driver does not support storage policy placement queries")
	}

	placements, err := vc.DiskStoragePlacements(vm)
	if err != nil {
		return err
	}
	if len(placements) < len(wantPolicies) {
		return fmt.Errorf("expected at least %d disks, got %d", len(wantPolicies), len(placements))
	}

	start := len(placements) - len(wantPolicies)
	checked := placements[start:]

	datastores := make(map[string]struct{})
	for i, want := range wantPolicies {
		got := checked[i]
		if got.PolicyName != want {
			return fmt.Errorf("disk %d: expected storage policy %q, got %q", i+1, want, got.PolicyName)
		}
		if got.DatastoreName == "" {
			return fmt.Errorf("disk %d: expected a datastore backing for storage policy %q", i+1, want)
		}
		datastores[got.DatastoreName] = struct{}{}
	}

	distinctPolicies := make(map[string]struct{}, len(wantPolicies))
	for _, p := range wantPolicies {
		distinctPolicies[p] = struct{}{}
	}
	if len(distinctPolicies) >= 2 && len(datastores) < 2 {
		return fmt.Errorf("expected at least two distinct datastores for policies %v, got one shared placement among %v", wantPolicies, checked)
	}

	return nil
}
