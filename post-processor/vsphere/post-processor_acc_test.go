// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package vsphere

import (
	"context"
	"testing"

	"github.com/vmware/packer-plugin-vsphere/testing/acceptance"
	"github.com/vmware/packer-plugin-vsphere/testing/env"
)

// ---------------------------------------------------------------------------
// Upload OVA using OVFTool
// ---------------------------------------------------------------------------

func TestAccPostProcessorVSphere(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acceptance.RequireOvftool(t)

	acc := env.AccFromEnv()
	acceptance.RequireOVAURL(t, acc)

	ovaPath, cleanup := acceptance.DownloadOVA(t, acc)
	defer cleanup()

	vmName := acceptance.NewVmName()
	pp := &PostProcessor{}
	if err := pp.Configure(map[string]any{
		"host":          acc.VCenterServer,
		"username":      acc.Username,
		"password":      acc.Password,
		"datacenter":    acc.Datacenter,
		"cluster":       acc.Cluster,
		"datastore":     acc.Datastore,
		"resource_pool": acc.ResourcePool,
		"vm_folder":     acc.Folder,
		"vm_network":    acc.Network,
		"vm_name":       vmName,
		"esxi_host":     acc.Host,
		"insecure":      true,
		"disk_mode":     "thin",
		"overwrite":     true,
	}); err != nil {
		t.Fatalf("configure the post-processor: %v", err)
	}

	artifact := &acceptance.FileArtifact{Path: ovaPath, BuilderID: "packer.file"}
	ui := acceptance.BasicUI()
	out, _, _, err := pp.PostProcess(context.Background(), ui, artifact)
	if err != nil {
		t.Fatalf("upload virtual machine: %v", err)
	}
	if out == nil {
		t.Fatal("expected non-nil artifact from the post-processor")
	}

	t.Cleanup(func() {
		d, err := acceptance.TestConn()
		if err != nil {
			t.Logf("teardown connect: %v", err)
			return
		}
		if err := acceptance.CleanupVm(d, vmName); err != nil {
			t.Logf("teardown %s: %v", vmName, err)
		}
	})

	if err := acceptance.CheckVMExists(vmName); err != nil {
		t.Fatalf("check if virtual machine exists: %v", err)
	}
}
