// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package clone

import (
	"archive/tar"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/acctest"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
	"github.com/vmware/packer-plugin-vsphere/testing/acceptance"
	"github.com/vmware/packer-plugin-vsphere/testing/env"
)

const (
	accVMTXMac      = "00:50:56:00:00:11"
	accOVFMac       = "00:50:56:00:00:21"
	accLocalOVFMac  = "00:50:56:00:00:31"
	accLocalOVAMac  = "00:50:56:00:00:32"
	accRemoteOVFMac = "00:50:56:00:00:41"
	accRemoteOVAMac = "00:50:56:00:00:42"
	accDiskMiB      = 8192
	accExtraMiB     = 4096
)

// ---------------------------------------------------------------------------
// Shared Helpers
// ---------------------------------------------------------------------------

func cloneExampleConfig() map[string]interface{} {
	acc := env.AccFromEnv()
	return map[string]interface{}{
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

		"vm_name":      acceptance.NewVmName(),
		"template":     acc.Template,
		"communicator": "none",
	}
}

func ovfSourceConfig(acc env.AccConfig, mac string) map[string]interface{} {
	config := cloneExampleConfig()
	delete(config, "template")
	config["network"] = acc.Network
	config["mac_address"] = mac
	return config
}

func ovfSourceLocalPath(path string) map[string]interface{} {
	return map[string]interface{}{
		"path": path,
	}
}

func ovfSourceRemote(acc env.AccConfig, rawURL string) map[string]interface{} {
	src := map[string]interface{}{
		"url": rawURL,
	}
	if acc.OVFUsername != "" {
		src["username"] = acc.OVFUsername
	}
	if acc.OVFPassword != "" {
		src["password"] = acc.OVFPassword
	}
	if acc.OVFSkipTLSVerify {
		src["skip_tls_verify"] = true
	}
	return src
}

func requireOVAURL(t *testing.T, acc env.AccConfig) {
	t.Helper()
	if acc.OVAURL == "" {
		t.Skipf("set %s to an HTTPS .ova URL to run this ACC row", env.EnvOVAURL)
	}
}

func requireOVFURL(t *testing.T, acc env.AccConfig) {
	t.Helper()
	if acc.OVFURL == "" {
		t.Skipf("set %s to an HTTPS .ovf URL to run this ACC row", env.EnvOVFURL)
	}
}

func downloadOVA(t *testing.T, acc env.AccConfig) (ovaPath string, cleanup func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "packer-acc-ova-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	ovaPath = filepath.Join(dir, "source.ova")
	downloadURLToFile(t, acc, acc.OVAURL, ovaPath)
	return ovaPath, func() { _ = os.RemoveAll(dir) }
}

func downloadURLToFile(t *testing.T, acc env.AccConfig, rawURL, destPath string) {
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
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download %s: HTTP %d", rawURL, resp.StatusCode)
	}
	f, err := os.Create(destPath)
	if err != nil {
		t.Fatalf("create %s: %v", destPath, err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		t.Fatalf("write %s: %v", destPath, err)
	}
}

func extractOVFFromOVA(t *testing.T, acc env.AccConfig) (ovfPath string, cleanup func()) {
	t.Helper()
	ovaPath, cleanupOVA := downloadOVA(t, acc)
	dir := filepath.Dir(ovaPath)
	extractDir := filepath.Join(dir, "extracted")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		cleanupOVA()
		t.Fatalf("mkdir extract: %v", err)
	}

	f, err := os.Open(ovaPath)
	if err != nil {
		cleanupOVA()
		t.Fatalf("open ova: %v", err)
	}
	defer f.Close()

	tr := tar.NewReader(f)
	var foundOVF string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			cleanupOVA()
			t.Fatalf("read ova: %v", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(hdr.Name)
		outPath := filepath.Join(extractDir, base)
		out, err := os.Create(outPath)
		if err != nil {
			cleanupOVA()
			t.Fatalf("create %s: %v", outPath, err)
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			cleanupOVA()
			t.Fatalf("extract %s: %v", base, err)
		}
		_ = out.Close()
		if strings.HasSuffix(strings.ToLower(base), ".ovf") && foundOVF == "" {
			foundOVF = outPath
		}
	}
	if foundOVF == "" {
		cleanupOVA()
		t.Fatalf("OVA %s contained no .ovf descriptor", acc.OVAURL)
	}
	return foundOVF, cleanupOVA
}

func checkBuildSucceeded(buildCommand *exec.Cmd, logfile string) error {
	if buildCommand.ProcessState != nil && buildCommand.ProcessState.ExitCode() != 0 {
		return fmt.Errorf("bad exit code; logfile: %s", logfile)
	}
	return nil
}

func checkDiskSizesMiB(vm driver.VirtualMachine, wantMiB []int64) error {
	devices, err := vm.Devices()
	if err != nil {
		return fmt.Errorf("cannot read devices: %v", err)
	}
	disks := devices.SelectByType((*types.VirtualDisk)(nil))
	if len(disks) < len(wantMiB) {
		return fmt.Errorf("expected at least %d disks, got %d", len(wantMiB), len(disks))
	}
	for i, want := range wantMiB {
		gotKB := disks[i].(*types.VirtualDisk).CapacityInKB
		wantKB := want * 1024
		if gotKB != wantKB {
			return fmt.Errorf("disk %d: expected %d MiB (%d KB), got %d KB", i, want, wantKB, gotKB)
		}
	}
	return nil
}

func checkFolderAndResourcePool(d driver.Driver, parent, resourcePool *types.ManagedObjectReference, acc env.AccConfig) error {
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

func checkDiskAtControllerUnit(vm driver.VirtualMachine, addr string, wantMiB int64) error {
	devices, err := vm.Devices()
	if err != nil {
		return fmt.Errorf("cannot read devices: %v", err)
	}

	parsed, err := driver.ParseControllerUnit(addr)
	if err != nil {
		return fmt.Errorf("cannot parse controller address %q: %v", addr, err)
	}

	controller := driver.FindControllerByBus(devices, parsed.Kind, parsed.Bus)
	if controller == nil {
		return fmt.Errorf("controller for %q not found on cloned VM", addr)
	}
	controllerKey := controller.GetVirtualController().Key

	for _, device := range devices.SelectByType((*types.VirtualDisk)(nil)) {
		disk := device.(*types.VirtualDisk)
		vd := disk.GetVirtualDevice()
		if vd.ControllerKey != controllerKey || vd.UnitNumber == nil || int(*vd.UnitNumber) != parsed.Unit {
			continue
		}
		wantKB := wantMiB * 1024
		if disk.CapacityInKB != wantKB {
			return fmt.Errorf("disk at %q: expected %d MiB (%d KB), got %d KB", addr, wantMiB, wantKB, disk.CapacityInKB)
		}
		return nil
	}
	return fmt.Errorf("no disk found at %q on cloned VM", addr)
}

func checkOvfSourcePlacement(name string, acc env.AccConfig, mac string) error {
	d, vm, parent, rp, err := findVM(name)
	if err != nil {
		return err
	}
	if err := checkFolderAndResourcePool(d, parent, rp, acc); err != nil {
		return err
	}
	return checkPrimaryNICMac(vm, mac)
}

func checkPrimaryNICMac(vm driver.VirtualMachine, want string) error {
	devices, err := vm.Devices()
	if err != nil {
		return fmt.Errorf("cannot read devices: %v", err)
	}
	nics := devices.SelectByType((*types.VirtualEthernetCard)(nil))
	if len(nics) == 0 {
		return fmt.Errorf("expected at least one network adapter")
	}
	got := strings.ToLower(nics[0].(types.BaseVirtualEthernetCard).GetVirtualEthernetCard().MacAddress)
	want = strings.ToLower(want)
	if got != want {
		return fmt.Errorf("unexpected MAC: expected %q, got %q", want, got)
	}
	return nil
}

func findVM(name string) (driver.Driver, driver.VirtualMachine, *types.ManagedObjectReference, *types.ManagedObjectReference, error) {
	d, err := acceptance.TestConn()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("cannot connect %v", err)
	}
	vm, err := d.FindVM(name)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("cannot find VM: %v", err)
	}
	info, err := vm.Info("name", "parent", "resourcePool")
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("cannot read VM properties: %v", err)
	}
	if info.Name != name {
		return nil, nil, nil, nil, fmt.Errorf("unexpected VM name: expected %q, got %q", name, info.Name)
	}
	return d, vm, info.Parent, info.ResourcePool, nil
}

func teardownVM(vmName string) error {
	d, err := acceptance.TestConn()
	if err != nil {
		return fmt.Errorf("cannot connect %v", err)
	}
	return acceptance.CleanupVm(d, vmName)
}

// ---------------------------------------------------------------------------
// Matrix A — Template Source
// ---------------------------------------------------------------------------

func TestAccCloneBuilder_MatrixA(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	config := cloneExampleConfig()
	vmName := config["vm_name"].(string)

	testCase := &acctest.PluginTestCase{
		Name:     "vsphere-clone-matrix-a",
		Template: acceptance.RenderConfig("vsphere-clone", config),
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
	d, _, parent, rp, err := findVM(name)
	if err != nil {
		return err
	}
	return checkFolderAndResourcePool(d, parent, rp, acc)
}

// ---------------------------------------------------------------------------
// Matrix B — Content Library VM Template Source
// ---------------------------------------------------------------------------

func TestAccCloneBuilder_MatrixB(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	config := cloneExampleConfig()
	vmName := config["vm_name"].(string)

	delete(config, "template")
	config["network"] = acc.Network
	config["mac_address"] = accVMTXMac
	config["disk_size"] = accDiskMiB
	config["disk_controller_type"] = []string{"pvscsi"}
	config["storage"] = map[string]interface{}{
		"disk_size":             accExtraMiB,
		"disk_thin_provisioned": true,
	}
	config["content_library_source"] = map[string]interface{}{
		"library": acc.ContentLibrary,
		"name":    acc.ContentLibraryVMTX,
	}

	testCase := &acctest.PluginTestCase{
		Name:     "vsphere-clone-matrix-b",
		Template: acceptance.RenderConfig("vsphere-clone", config),
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
	d, vm, parent, rp, err := findVM(name)
	if err != nil {
		return err
	}
	if err := checkFolderAndResourcePool(d, parent, rp, acc); err != nil {
		return err
	}
	if err := checkPrimaryNICMac(vm, accVMTXMac); err != nil {
		return err
	}
	return checkDiskSizesMiB(vm, []int64{accDiskMiB, accExtraMiB})
}

// ---------------------------------------------------------------------------
// Matrix C — Content Library OVF Source
// ---------------------------------------------------------------------------

func TestAccCloneBuilder_MatrixC(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	config := cloneExampleConfig()
	vmName := config["vm_name"].(string)

	delete(config, "template")
	config["network"] = acc.Network
	config["mac_address"] = accOVFMac
	config["content_library_source"] = map[string]interface{}{
		"library": acc.ContentLibrary,
		"name":    acc.ContentLibraryOVF,
	}

	testCase := &acctest.PluginTestCase{
		Name:     "vsphere-clone-matrix-c",
		Template: acceptance.RenderConfig("vsphere-clone", config),
		Teardown: func() error {
			return teardownVM(vmName)
		},
		Check: func(buildCommand *exec.Cmd, logfile string) error {
			if err := checkBuildSucceeded(buildCommand, logfile); err != nil {
				return err
			}
			return checkMatrixC(vmName, acc)
		},
	}
	acctest.TestPlugin(t, testCase)
}

func checkMatrixC(name string, acc env.AccConfig) error {
	d, vm, parent, rp, err := findVM(name)
	if err != nil {
		return err
	}
	if err := checkFolderAndResourcePool(d, parent, rp, acc); err != nil {
		return err
	}
	return checkPrimaryNICMac(vm, accOVFMac)
}

// ---------------------------------------------------------------------------
// Matrix D — Explicit Disk Controller Unit
// ---------------------------------------------------------------------------

// accDiskControllerUnit targets unit 1 on the template's existing primary SCSI
// controller (scsi0). The template's boot disk is expected to occupy scsi0:0,
// leaving scsi0:1 free without needing to know the controller's exact type
// (LSI Logic vs. PVSCSI).
const accDiskControllerUnit = "scsi0:1"

func TestAccCloneBuilder_MatrixD(t *testing.T) {
	acceptance.RequireAcceptance(t)
	config := cloneExampleConfig()
	vmName := config["vm_name"].(string)

	config["storage"] = map[string]interface{}{
		"disk_size":            accExtraMiB,
		"disk_controller_unit": accDiskControllerUnit,
	}

	testCase := &acctest.PluginTestCase{
		Name:     "vsphere-clone-matrix-d",
		Template: acceptance.RenderConfig("vsphere-clone", config),
		Teardown: func() error {
			return teardownVM(vmName)
		},
		Check: func(buildCommand *exec.Cmd, logfile string) error {
			if err := checkBuildSucceeded(buildCommand, logfile); err != nil {
				return err
			}
			return checkMatrixD(vmName, accDiskControllerUnit, accExtraMiB)
		},
	}
	acctest.TestPlugin(t, testCase)
}

func checkMatrixD(name, addr string, wantMiB int64) error {
	_, vm, _, _, err := findVM(name)
	if err != nil {
		return err
	}
	return checkDiskAtControllerUnit(vm, addr, wantMiB)
}

// ---------------------------------------------------------------------------
// Matrix E — Local OVF Source
// ---------------------------------------------------------------------------

func TestAccCloneBuilder_MatrixE(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	requireOVAURL(t, acc)

	ovfPath, cleanup := extractOVFFromOVA(t, acc)
	defer cleanup()

	config := ovfSourceConfig(acc, accLocalOVFMac)
	vmName := config["vm_name"].(string)
	config["ovf_source"] = ovfSourceLocalPath(ovfPath)

	testCase := &acctest.PluginTestCase{
		Name:     "vsphere-clone-matrix-e",
		Template: acceptance.RenderConfig("vsphere-clone", config),
		Teardown: func() error {
			return teardownVM(vmName)
		},
		Check: func(buildCommand *exec.Cmd, logfile string) error {
			if err := checkBuildSucceeded(buildCommand, logfile); err != nil {
				return err
			}
			return checkOvfSourcePlacement(vmName, acc, accLocalOVFMac)
		},
	}
	acctest.TestPlugin(t, testCase)
}

// ---------------------------------------------------------------------------
// Matrix F — Local OVA Source
// ---------------------------------------------------------------------------

func TestAccCloneBuilder_MatrixF(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	requireOVAURL(t, acc)

	ovaPath, cleanup := downloadOVA(t, acc)
	defer cleanup()

	config := ovfSourceConfig(acc, accLocalOVAMac)
	vmName := config["vm_name"].(string)
	config["ovf_source"] = ovfSourceLocalPath(ovaPath)

	testCase := &acctest.PluginTestCase{
		Name:     "vsphere-clone-matrix-f",
		Template: acceptance.RenderConfig("vsphere-clone", config),
		Teardown: func() error {
			return teardownVM(vmName)
		},
		Check: func(buildCommand *exec.Cmd, logfile string) error {
			if err := checkBuildSucceeded(buildCommand, logfile); err != nil {
				return err
			}
			return checkOvfSourcePlacement(vmName, acc, accLocalOVAMac)
		},
	}
	acctest.TestPlugin(t, testCase)
}

// ---------------------------------------------------------------------------
// Matrix G — Remote HTTPS OVF Source
// ---------------------------------------------------------------------------

func TestAccCloneBuilder_MatrixG(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	requireOVFURL(t, acc)

	config := ovfSourceConfig(acc, accRemoteOVFMac)
	vmName := config["vm_name"].(string)
	config["ovf_source"] = ovfSourceRemote(acc, acc.OVFURL)

	testCase := &acctest.PluginTestCase{
		Name:     "vsphere-clone-matrix-g",
		Template: acceptance.RenderConfig("vsphere-clone", config),
		Teardown: func() error {
			return teardownVM(vmName)
		},
		Check: func(buildCommand *exec.Cmd, logfile string) error {
			if err := checkBuildSucceeded(buildCommand, logfile); err != nil {
				return err
			}
			return checkOvfSourcePlacement(vmName, acc, accRemoteOVFMac)
		},
	}
	acctest.TestPlugin(t, testCase)
}

// ---------------------------------------------------------------------------
// Matrix H — Remote HTTPS OVA Source
// ---------------------------------------------------------------------------

func TestAccCloneBuilder_MatrixH(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	requireOVAURL(t, acc)

	config := ovfSourceConfig(acc, accRemoteOVAMac)
	vmName := config["vm_name"].(string)
	config["ovf_source"] = ovfSourceRemote(acc, acc.OVAURL)

	testCase := &acctest.PluginTestCase{
		Name:     "vsphere-clone-matrix-h",
		Template: acceptance.RenderConfig("vsphere-clone", config),
		Teardown: func() error {
			return teardownVM(vmName)
		},
		Check: func(buildCommand *exec.Cmd, logfile string) error {
			if err := checkBuildSucceeded(buildCommand, logfile); err != nil {
				return err
			}
			return checkOvfSourcePlacement(vmName, acc, accRemoteOVAMac)
		},
	}
	acctest.TestPlugin(t, testCase)
}

// ---------------------------------------------------------------------------
// Matrix I — Storage Policy PBM Placement
// ---------------------------------------------------------------------------

func TestAccCloneBuilder_MatrixI(t *testing.T) {
	acceptance.RequireAcceptance(t)
	acc := env.AccFromEnv()
	policies := acc.StoragePolicies()

	config := cloneExampleConfig()
	delete(config, "datastore")
	config["disk_controller_type"] = []string{"pvscsi"}

	disks := make([]map[string]interface{}, 0, len(policies))
	for _, policy := range policies {
		disks = append(disks, map[string]interface{}{
			"disk_size":             accExtraMiB,
			"disk_thin_provisioned": true,
			"storage_policy":        policy,
		})
	}
	config["storage"] = disks

	vmName := config["vm_name"].(string)
	testCase := &acctest.PluginTestCase{
		Name:     "vsphere-clone-matrix-i",
		Template: acceptance.RenderConfig("vsphere-clone", config),
		Teardown: func() error {
			return teardownVM(vmName)
		},
		Check: func(buildCommand *exec.Cmd, logfile string) error {
			if err := checkBuildSucceeded(buildCommand, logfile); err != nil {
				return err
			}
			return checkMatrixI(vmName, acc, policies)
		},
	}
	acctest.TestPlugin(t, testCase)
}

func checkMatrixI(name string, acc env.AccConfig, policies []string) error {
	d, vm, parent, rp, err := findVM(name)
	if err != nil {
		return err
	}
	if err := checkFolderAndResourcePool(d, parent, rp, acc); err != nil {
		return err
	}
	return acceptance.CheckStoragePolicyDiskPlacements(d, vm, policies)
}
