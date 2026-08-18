// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package iso

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/acctest"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
	"github.com/vmware/packer-plugin-vsphere/testing/acceptance"
	"github.com/vmware/packer-plugin-vsphere/testing/env"
)

// ---------------------------------------------------------------------------
// Shared Helpers
// ---------------------------------------------------------------------------

func alpineExampleConfig() map[string]any {
	acc := env.AccFromEnv()
	exampleDir := alpineExampleDir()

	return map[string]any{
		"vcenter_server":      acc.VCenterServer,
		"username":            acc.Username,
		"password":            acc.Password,
		"datacenter":          acc.Datacenter,
		"host":                acc.Host,
		"cluster":             acc.Cluster,
		"datastore":           acc.Datastore,
		"folder":              acc.Folder,
		"resource_pool":       acc.ResourcePool,
		"insecure_connection": true,

		"vm_name":       acceptance.NewVmName(),
		"vm_version":    21,
		"guest_os_type": "other6xLinux64Guest",
		"CPUs":          1,
		"RAM":           512,

		"disk_controller_type": []string{"pvscsi"},
		"storage": map[string]any{
			"disk_size":             1024,
			"disk_thin_provisioned": true,
		},
		"network_adapters": map[string]any{
			"network_card": "vmxnet3",
			"network":      acc.Network,
		},

		"iso_paths": []string{acc.ISOPath},
		"floppy_files": []string{
			filepath.Join(exampleDir, "answerfile"),
			filepath.Join(exampleDir, "setup.sh"),
		},

		"boot_wait": "15s",
		"boot_command": []string{
			"<wait30>",
			"root<enter><wait>",
			"mount -t vfat /dev/fd0 /media/floppy<enter><wait>",
			"setup-alpine -f /media/floppy/answerfile<enter>",
			"<wait15>",
			"VMw@re1!<enter>",
			"VMw@re1!<enter>",
			"<wait10>",
			"y<enter>",
			"<wait45>",
			"reboot<enter>",
			// EFI reboot + DHCP need more headroom than the example's 20s.
			"<wait60>",
			"root<enter><wait>",
			"VMw@re1!<enter><wait>",
			"mount -t vfat /dev/fd0 /media/floppy<enter><wait>",
			"/media/floppy/SETUP.SH<enter>",
			// Give open-vm-tools time to install before IP wait starts polling.
			"<wait90>",
		},

		"ssh_username": "root",
		"ssh_password": "VMw@re1!",
	}
}

func alpineExampleDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "examples", "builder", "vsphere-iso", "alpine")
}

func alpineMatrixGuest(config map[string]any) {
	config["CPUs"] = 2
	config["RAM"] = 2048
	config["firmware"] = "efi"
	config["boot_wait"] = "30s"
	config["storage"] = map[string]any{
		"disk_size":             2048,
		"disk_thin_provisioned": true,
	}
	config["ip_settle_timeout"] = "30s"
}

func checkBuildSucceeded(buildCommand *exec.Cmd, logfile string) error {
	if buildCommand.ProcessState != nil && buildCommand.ProcessState.ExitCode() != 0 {
		return fmt.Errorf("bad exit code; logfile: %s", logfile)
	}
	return nil
}

func checkFolderAndResourcePool(d driver.Driver, parent, resourcePool *types.ManagedObjectReference, acc env.AccConfig, requireResourcePool bool) error {
	if parent == nil {
		return fmt.Errorf("expected VM parent folder %q", acc.Folder)
	}
	folder := d.NewFolder(parent)
	folderInfo, err := folder.Info("name")
	if err != nil {
		return fmt.Errorf("cannot read folder properties: %v", err)
	}
	if folderInfo.Name != acc.Folder {
		return fmt.Errorf("unexpected folder: expected %q, got %q", acc.Folder, folderInfo.Name)
	}

	if !requireResourcePool {
		return nil
	}

	if resourcePool == nil {
		return fmt.Errorf("expected VM resource pool %q", acc.ResourcePool)
	}
	rp := d.NewResourcePool(resourcePool)
	rpInfo, err := rp.Info("name")
	if err != nil {
		return fmt.Errorf("cannot read resource pool properties: %v", err)
	}
	if rpInfo.Name != acc.ResourcePool {
		return fmt.Errorf("unexpected resource pool: expected %q, got %q", acc.ResourcePool, rpInfo.Name)
	}
	return nil
}

func checkTagsByName(d driver.Driver, vm driver.VirtualMachine, categoryName string, expectedTagNames []string) error {
	if err := restLogin(d); err != nil {
		return fmt.Errorf("REST login for tags: %v", err)
	}

	ctx := context.Background()
	tagsManager := tags.NewManager(d.GetRestClient())
	attachedTagIDs, err := tagsManager.ListAttachedTags(ctx, vm.Reference())
	if err != nil {
		return fmt.Errorf("cannot list attached tags: %v", err)
	}

	attachedTagNames := make(map[string]bool)
	for _, tagID := range attachedTagIDs {
		tagInfo, err := tagsManager.GetTag(ctx, tagID)
		if err != nil {
			continue
		}
		categoryInfo, err := tagsManager.GetCategory(ctx, tagInfo.CategoryID)
		if err != nil {
			continue
		}
		if categoryInfo.Name == categoryName {
			attachedTagNames[tagInfo.Name] = true
		}
	}

	for _, expectedTagName := range expectedTagNames {
		if !attachedTagNames[expectedTagName] {
			return fmt.Errorf("expected tag %q in category %q not attached", expectedTagName, categoryName)
		}
	}
	return nil
}

func ensureTagCategoryMultiple(t *testing.T, categoryName string) {
	t.Helper()
	d, err := acceptance.TestConn()
	if err != nil {
		t.Fatalf("cannot connect: %v", err)
	}
	if err := restLogin(d); err != nil {
		t.Fatalf("REST login: %v", err)
	}

	ctx := context.Background()
	tm := tags.NewManager(d.GetRestClient())
	categories, err := tm.GetCategories(ctx)
	if err != nil {
		t.Fatalf("list tag categories: %v", err)
	}

	for i := range categories {
		cat := categories[i]
		if cat.Name != categoryName {
			continue
		}
		if cat.Cardinality == "MULTIPLE" {
			return
		}
		cat.Cardinality = "MULTIPLE"
		if err := tm.UpdateCategory(ctx, &cat); err != nil {
			t.Fatalf("tag category %q must allow MULTIPLE tags (attach blue+red); update failed: %v", categoryName, err)
		}
		return
	}

	if _, err := tm.CreateCategory(ctx, &tags.Category{
		Name:            categoryName,
		Cardinality:     "MULTIPLE",
		AssociableTypes: []string{"VirtualMachine"},
	}); err != nil {
		t.Fatalf("create tag category %q: %v", categoryName, err)
	}
}

func restLogin(d driver.Driver) error {
	vd, ok := d.(*driver.VCenterDriver)
	if !ok {
		return fmt.Errorf("unexpected driver type %T", d)
	}
	return vd.RestClient.Login(vd.Ctx)
}

func teardownContentLibraryItem(libraryName, itemName string) error {
	d, err := acceptance.TestConn()
	if err != nil {
		return fmt.Errorf("cannot connect %v", err)
	}
	item, err := d.ResolveContentLibraryItem(libraryName, itemName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return err
	}
	return d.DeleteContentLibraryItem(item.ID)
}

func teardownVM(vmName string) error {
	d, err := acceptance.TestConn()
	if err != nil {
		return fmt.Errorf("cannot connect %v", err)
	}
	return acceptance.CleanupVm(d, vmName)
}

// ---------------------------------------------------------------------------
// Matrix A — Datastore Placement with Hardware Overlays
// ---------------------------------------------------------------------------

func TestAccISOBuilder_MatrixA(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	ensureTagCategoryMultiple(t, acc.TagCategory)

	config := alpineExampleConfig()
	alpineMatrixGuest(config)
	config["notes"] = acc.Notes
	config["tag"] = []map[string]any{
		{"category": acc.TagCategory, "name": acc.TagA},
		{"category": acc.TagCategory, "name": acc.TagB},
	}
	config["cpu_cores"] = 2
	config["CPU_reservation"] = 1000
	config["CPU_limit"] = 1500
	config["RAM_reservation"] = 1024
	config["NestedHV"] = true
	config["video_ram"] = 8192
	config["remove_cdrom"] = true
	config["create_snapshot"] = true
	config["snapshot_name"] = "acc-test-snapshot"
	config["convert_to_template"] = true

	vmName := config["vm_name"].(string)
	testCase := &acctest.PluginTestCase{
		Name:     "vsphere-iso-matrix-a",
		Template: acceptance.RenderConfig("vsphere-iso", config),
		Teardown: func() error {
			return teardownVM(vmName)
		},
		Check: func(buildCommand *exec.Cmd, logfile string) error {
			if err := checkBuildSucceeded(buildCommand, logfile); err != nil {
				return err
			}
			return checkMatrixA(vmName, acc)
		},
	}
	acctest.TestPlugin(t, testCase)
}

func checkMatrixA(name string, acc env.AccConfig) error {
	d, err := acceptance.TestConn()
	if err != nil {
		return fmt.Errorf("cannot connect %v", err)
	}
	vm, err := d.FindVM(name)
	if err != nil {
		return fmt.Errorf("cannot find VM: %v", err)
	}

	vmInfo, err := vm.Info(
		"name",
		"parent",
		"runtime.host",
		"resourcePool",
		"datastore",
		"config",
		"snapshot",
	)
	if err != nil {
		return fmt.Errorf("cannot read VM properties: %v", err)
	}

	if !vmInfo.Config.Template {
		return fmt.Errorf("expected convert_to_template result, but config.template is false")
	}
	if vmInfo.Config.Annotation != acc.Notes {
		return fmt.Errorf("unexpected notes: expected %q, got %q", acc.Notes, vmInfo.Config.Annotation)
	}
	if vmInfo.Config.Firmware != "efi" {
		return fmt.Errorf("unexpected firmware: expected 'efi', got %q", vmInfo.Config.Firmware)
	}
	if vmInfo.Config.Hardware.NumCPU != 2 {
		return fmt.Errorf("expected 2 CPU sockets, got %v", vmInfo.Config.Hardware.NumCPU)
	}
	if vmInfo.Config.Hardware.NumCoresPerSocket == nil || *vmInfo.Config.Hardware.NumCoresPerSocket != 2 {
		return fmt.Errorf("expected 2 CPU cores per socket")
	}
	if vmInfo.Config.CpuAllocation == nil || vmInfo.Config.CpuAllocation.Reservation == nil || *vmInfo.Config.CpuAllocation.Reservation != 1000 {
		return fmt.Errorf("expected CPU reservation 1000 MHz")
	}
	if vmInfo.Config.CpuAllocation.Limit == nil || *vmInfo.Config.CpuAllocation.Limit != 1500 {
		return fmt.Errorf("expected CPU limit 1500 MHz")
	}
	if vmInfo.Config.Hardware.MemoryMB != 2048 {
		return fmt.Errorf("expected 2048 MB RAM, got %v", vmInfo.Config.Hardware.MemoryMB)
	}
	if vmInfo.Config.MemoryAllocation == nil || vmInfo.Config.MemoryAllocation.Reservation == nil || *vmInfo.Config.MemoryAllocation.Reservation != 1024 {
		return fmt.Errorf("expected RAM reservation 1024 MB")
	}
	if vmInfo.Config.NestedHVEnabled == nil || !*vmInfo.Config.NestedHVEnabled {
		return fmt.Errorf("expected NestedHV enabled")
	}

	h := d.NewHost(vmInfo.Runtime.Host)
	hostInfo, err := h.Info("name")
	if err != nil {
		return fmt.Errorf("cannot read host properties: %#v", err)
	}
	if hostInfo.Name != acc.Host {
		return fmt.Errorf("unexpected host name: expected %q, got %q", acc.Host, hostInfo.Name)
	}

	if err := checkFolderAndResourcePool(d, vmInfo.Parent, vmInfo.ResourcePool, acc, !vmInfo.Config.Template); err != nil {
		return err
	}

	dsr := vmInfo.Datastore[0].Reference()
	ds := d.NewDatastore(&dsr)
	dsInfo, err := ds.Info("name")
	if err != nil {
		return fmt.Errorf("cannot read datastore properties: %#v", err)
	}
	if dsInfo.Name != acc.Datastore {
		return fmt.Errorf("unexpected datastore name: expected %q, got %q", acc.Datastore, dsInfo.Name)
	}

	if vmInfo.Snapshot == nil || vmInfo.Snapshot.RootSnapshotList == nil || len(vmInfo.Snapshot.RootSnapshotList) == 0 {
		return fmt.Errorf("expected create_snapshot to leave a snapshot on the template")
	}

	devices, err := vm.Devices()
	if err != nil {
		return fmt.Errorf("cannot read devices: %v", err)
	}
	cdroms := devices.SelectByType((*types.VirtualCdrom)(nil))
	if len(cdroms) != 0 {
		return fmt.Errorf("expected remove_cdrom to leave zero CD-ROM devices, got %d", len(cdroms))
	}
	videos := devices.SelectByType((*types.VirtualMachineVideoCard)(nil))
	if len(videos) != 1 {
		return fmt.Errorf("expected one video card")
	}
	if videos[0].(*types.VirtualMachineVideoCard).VideoRamSizeInKB != 8192 {
		return fmt.Errorf("expected video_ram 8192")
	}

	return checkTagsByName(d, vm, acc.TagCategory, []string{acc.TagA, acc.TagB})
}

// ---------------------------------------------------------------------------
// Matrix B — Datastore Cluster Placement
// ---------------------------------------------------------------------------

func TestAccISOBuilder_MatrixB(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	config := alpineExampleConfig()
	alpineMatrixGuest(config)
	delete(config, "datastore")
	config["datastore_cluster"] = acc.DatastoreCluster

	vmName := config["vm_name"].(string)
	testCase := &acctest.PluginTestCase{
		Name:     "vsphere-iso-matrix-b",
		Template: acceptance.RenderConfig("vsphere-iso", config),
		Teardown: func() error {
			return teardownVM(vmName)
		},
		Check: func(buildCommand *exec.Cmd, logfile string) error {
			if err := checkBuildSucceeded(buildCommand, logfile); err != nil {
				return err
			}
			return checkMatrixB(vmName, acc)
		},
	}
	acctest.TestPlugin(t, testCase)
}

func checkMatrixB(name string, acc env.AccConfig) error {
	d, err := acceptance.TestConn()
	if err != nil {
		return fmt.Errorf("cannot connect %v", err)
	}
	vm, err := d.FindVM(name)
	if err != nil {
		return fmt.Errorf("cannot find VM: %v", err)
	}
	vmInfo, err := vm.Info("name", "parent", "resourcePool", "datastore")
	if err != nil {
		return fmt.Errorf("cannot read VM properties: %v", err)
	}
	if err := checkFolderAndResourcePool(d, vmInfo.Parent, vmInfo.ResourcePool, acc, true); err != nil {
		return err
	}
	if len(vmInfo.Datastore) == 0 {
		return fmt.Errorf("expected datastore_cluster placement to assign a datastore")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Matrix C — Content Library VM Template
// ---------------------------------------------------------------------------

func TestAccISOBuilder_MatrixC(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	config := alpineExampleConfig()
	alpineMatrixGuest(config)
	vmName := config["vm_name"].(string)
	clItemName := vmName + "-vm-template"

	config["reattach_cdroms"] = 1
	config["content_library_destination"] = map[string]any{
		"library": acc.ContentLibrary,
		"name":    clItemName,
		"ovf":     false,
		"destroy": true,
	}

	testCase := &acctest.PluginTestCase{
		Name:     "vsphere-iso-matrix-c",
		Template: acceptance.RenderConfig("vsphere-iso", config),
		Teardown: func() error {
			_ = teardownVM(vmName)
			return teardownContentLibraryItem(acc.ContentLibrary, clItemName)
		},
		Check: func(buildCommand *exec.Cmd, logfile string) error {
			if err := checkBuildSucceeded(buildCommand, logfile); err != nil {
				return err
			}
			return checkMatrixC(vmName, acc.ContentLibrary, clItemName)
		},
	}
	acctest.TestPlugin(t, testCase)
}

func checkMatrixC(vmName, libraryName, itemName string) error {
	d, err := acceptance.TestConn()
	if err != nil {
		return fmt.Errorf("cannot connect %v", err)
	}

	if _, err := d.FindVM(vmName); err == nil {
		return fmt.Errorf("expected VM %q to be destroyed after content library import", vmName)
	} else if !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("unexpected FindVM error: %v", err)
	}

	item, err := d.ResolveContentLibraryItem(libraryName, itemName)
	if err != nil {
		return fmt.Errorf("expected content library VM template item: %v", err)
	}
	if item.Type == "ovf" {
		return fmt.Errorf("unexpected content library item type %q for VMTX import", item.Type)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Matrix D — Content Library OVF Template
// ---------------------------------------------------------------------------

func TestAccISOBuilder_MatrixD(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	config := alpineExampleConfig()
	alpineMatrixGuest(config)
	vmName := config["vm_name"].(string)
	clItemName := vmName + "-ovf-template"
	exportDir := filepath.Join(os.TempDir(), vmName+"ovf-export")

	config["reattach_cdroms"] = 1
	config["content_library_destination"] = map[string]any{
		"library": acc.ContentLibrary,
		"name":    clItemName,
		"ovf":     true,
	}
	config["export"] = map[string]any{
		"force":            true,
		"output_directory": exportDir,
		"output_format":    "ovf",
	}

	testCase := &acctest.PluginTestCase{
		Name:     "vsphere-iso-matrix-d",
		Template: acceptance.RenderConfig("vsphere-iso", config),
		Teardown: func() error {
			_ = os.RemoveAll(exportDir)
			_ = teardownVM(vmName)
			return teardownContentLibraryItem(acc.ContentLibrary, clItemName)
		},
		Check: func(buildCommand *exec.Cmd, logfile string) error {
			if err := checkBuildSucceeded(buildCommand, logfile); err != nil {
				return err
			}
			return checkMatrixD(vmName, acc, acc.ContentLibrary, clItemName, exportDir)
		},
	}
	acctest.TestPlugin(t, testCase)
}

func checkMatrixD(vmName string, acc env.AccConfig, libraryName, itemName, exportDir string) error {
	d, err := acceptance.TestConn()
	if err != nil {
		return fmt.Errorf("cannot connect %v", err)
	}

	vm, err := d.FindVM(vmName)
	if err != nil {
		return fmt.Errorf("cannot find VM after OVF import/export: %v", err)
	}

	vmInfo, err := vm.Info("name", "parent", "resourcePool")
	if err != nil {
		return fmt.Errorf("cannot read VM properties: %v", err)
	}
	if err := checkFolderAndResourcePool(d, vmInfo.Parent, vmInfo.ResourcePool, acc, true); err != nil {
		return err
	}

	devices, err := vm.Devices()
	if err != nil {
		return fmt.Errorf("cannot read devices: %v", err)
	}
	cdroms := devices.SelectByType((*types.VirtualCdrom)(nil))
	if len(cdroms) != 1 {
		return fmt.Errorf("expected reattach_cdroms=1 to leave one CD-ROM, got %d", len(cdroms))
	}
	_, ok := cdroms[0].(*types.VirtualCdrom).Backing.(*types.VirtualCdromRemotePassthroughBackingInfo)
	if !ok {
		return fmt.Errorf("expected reattached CD-ROM to have empty/passthrough backing")
	}

	item, err := d.ResolveContentLibraryItem(libraryName, itemName)
	if err != nil {
		return fmt.Errorf("expected content library OVF item: %v", err)
	}
	if !strings.EqualFold(item.Type, "ovf") {
		return fmt.Errorf("unexpected content library item type %q, want ovf", item.Type)
	}

	ovfPath := filepath.Join(exportDir, vmName+".ovf")
	if _, err := os.Stat(ovfPath); err != nil {
		entries, _ := os.ReadDir(exportDir)
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return fmt.Errorf("expected export OVF at %s (dir contents: %v): %v", ovfPath, names, err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Matrix E — Storage Policy PBM Placement
// ---------------------------------------------------------------------------

func TestAccISOBuilder_MatrixE(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	policies := acc.StoragePolicies()

	config := alpineExampleConfig()
	alpineMatrixGuest(config)
	delete(config, "datastore")

	disks := make([]map[string]any, 0, len(policies))
	for _, policy := range policies {
		disks = append(disks, map[string]any{
			"disk_size":             2048,
			"disk_thin_provisioned": true,
			"storage_policy":        policy,
		})
	}
	config["storage"] = disks

	vmName := config["vm_name"].(string)
	testCase := &acctest.PluginTestCase{
		Name:     "vsphere-iso-matrix-e",
		Template: acceptance.RenderConfig("vsphere-iso", config),
		Teardown: func() error {
			return teardownVM(vmName)
		},
		Check: func(buildCommand *exec.Cmd, logfile string) error {
			if err := checkBuildSucceeded(buildCommand, logfile); err != nil {
				return err
			}
			return checkMatrixE(vmName, policies)
		},
	}
	acctest.TestPlugin(t, testCase)
}

func checkMatrixE(name string, policies []string) error {
	d, err := acceptance.TestConn()
	if err != nil {
		return fmt.Errorf("cannot connect %v", err)
	}
	vm, err := d.FindVM(name)
	if err != nil {
		return fmt.Errorf("cannot find VM: %v", err)
	}
	return acceptance.CheckStoragePolicyDiskPlacements(d, vm, policies)
}

// ---------------------------------------------------------------------------
// Matrix F — vTPM add and remove with OVF export and content library
// ---------------------------------------------------------------------------

func TestAccISOBuilder_MatrixF(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acceptance.RequireKeyProvider(t)
	acc := env.AccFromEnv()

	config := alpineExampleConfig()
	alpineMatrixGuest(config)
	config["guest_os_type"] = "ubuntu64Guest"
	config["vTPM"] = true
	config["remove_vtpm"] = true

	vmName := config["vm_name"].(string)
	clItemName := vmName + "-ovf-template"
	exportDir := filepath.Join(os.TempDir(), vmName+"-vtpm-export")
	config["content_library_destination"] = map[string]any{
		"library": acc.ContentLibrary,
		"name":    clItemName,
		"ovf":     true,
	}
	config["export"] = map[string]any{
		"force":            true,
		"output_directory": exportDir,
		"output_format":    "ovf",
	}

	testCase := &acctest.PluginTestCase{
		Name:     "vsphere-iso-matrix-f",
		Template: acceptance.RenderConfig("vsphere-iso", config),
		Teardown: func() error {
			_ = os.RemoveAll(exportDir)
			_ = teardownVM(vmName)
			return teardownContentLibraryItem(acc.ContentLibrary, clItemName)
		},
		Check: func(buildCommand *exec.Cmd, logfile string) error {
			if err := checkBuildSucceeded(buildCommand, logfile); err != nil {
				return err
			}
			return checkMatrixF(vmName, acc, acc.ContentLibrary, clItemName, exportDir)
		},
	}
	acctest.TestPlugin(t, testCase)
}

func checkMatrixF(name string, acc env.AccConfig, libraryName, itemName, exportDir string) error {
	d, err := acceptance.TestConn()
	if err != nil {
		return fmt.Errorf("cannot connect %v", err)
	}
	vm, err := d.FindVM(name)
	if err != nil {
		return fmt.Errorf("cannot find VM: %v", err)
	}

	vmInfo, err := vm.Info("name", "parent", "resourcePool", "config")
	if err != nil {
		return fmt.Errorf("cannot read VM properties: %v", err)
	}
	if err := checkFolderAndResourcePool(d, vmInfo.Parent, vmInfo.ResourcePool, acc, true); err != nil {
		return err
	}
	if vmInfo.Config == nil || vmInfo.Config.Firmware != "efi" {
		got := ""
		if vmInfo.Config != nil {
			got = vmInfo.Config.Firmware
		}
		return fmt.Errorf("unexpected firmware: expected 'efi', got %q", got)
	}
	if err := acceptance.CheckNoVTPM(vm); err != nil {
		return err
	}

	item, err := d.ResolveContentLibraryItem(libraryName, itemName)
	if err != nil {
		return fmt.Errorf("expected content library OVF item: %v", err)
	}
	if !strings.EqualFold(item.Type, "ovf") {
		return fmt.Errorf("unexpected content library item type %q, want ovf", item.Type)
	}

	ovfPath := filepath.Join(exportDir, name+".ovf")
	if _, err := os.Stat(ovfPath); err != nil {
		entries, _ := os.ReadDir(exportDir)
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return fmt.Errorf("expected export OVF at %s (dir contents: %v): %v", ovfPath, names, err)
	}
	return nil
}
