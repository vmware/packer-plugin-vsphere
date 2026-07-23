// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package vsphere_template

import (
	"context"
	"testing"

	vspherepost "github.com/vmware/packer-plugin-vsphere/post-processor/vsphere"
	"github.com/vmware/packer-plugin-vsphere/testing/acceptance"
	"github.com/vmware/packer-plugin-vsphere/testing/env"
)

// ---------------------------------------------------------------------------
// Upload OVA and Mark as Template
// ---------------------------------------------------------------------------

func TestAccPostProcessorVSphereTemplate(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acceptance.RequireOvftool(t)

	acc := env.AccFromEnv()
	acceptance.RequireOVAURL(t, acc)

	ovaPath, cleanup := acceptance.DownloadOVA(t, acc)
	defer cleanup()

	vmName := acceptance.NewVmName()
	upload := &vspherepost.PostProcessor{}
	if err := upload.Configure(map[string]interface{}{
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

	ui := acceptance.BasicUI()
	artifact, _, _, err := upload.PostProcess(context.Background(), ui, &acceptance.FileArtifact{
		Path:      ovaPath,
		BuilderID: "packer.file",
	})
	if err != nil {
		t.Fatalf("upload virtual machine: %v", err)
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

	mark := &PostProcessor{}
	if err := mark.Configure(map[string]interface{}{
		"host":       acc.VCenterServer,
		"username":   acc.Username,
		"password":   acc.Password,
		"datacenter": acc.Datacenter,
		"folder":     acc.Folder,
		"insecure":   true,
		"override":   true,
	}); err != nil {
		t.Fatalf("configure the post-processor: %v", err)
	}

	if _, _, _, err := mark.PostProcess(context.Background(), ui, artifact); err != nil {
		t.Fatalf("mark virtual machine as template: %v", err)
	}

	if err := acceptance.CheckVMIsTemplate(vmName); err != nil {
		t.Fatalf("check if virtual machine is a template: %v", err)
	}
}
