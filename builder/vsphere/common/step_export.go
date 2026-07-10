// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type ExportConfig

package common

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/pkg/errors"
	"github.com/vmware/govmomi/nfc"
	"github.com/vmware/govmomi/ovf"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

type exportStepVM interface {
	Export() (*nfc.Lease, error)
	NewOvfManager() *ovf.Manager
	GetOvfExportOptions(m *ovf.Manager) ([]types.OvfOptionInfo, error)
	CreateDescriptor(m *ovf.Manager, cdp types.OvfCreateDescriptorParams) (*types.OvfCreateDescriptorResult, error)
}

const OvftoolWindows = "ovftool.exe"

// You can export an image in Open Virtualization Format (OVF) to the Packer
// host.
//
// HCL Example:
//
// ```hcl
//
//	# ...
//	vm_name = "example-ubuntu"
//	# ...
//	export {
//	  force = true
//	  output_directory = "./output-artifacts"
//	}
//
// ```
//
// JSON Example:
//
// ```json
// ...
//
//	"vm_name": "example-ubuntu",
//
// ...
//
//	"export": {
//	  "force": true,
//	  "output_directory": "./output-artifacts"
//	},
//
// ```
//
// The above configuration would create the following files:
//
// ```text
// ./output-artifacts/example-ubuntu-disk-0.vmdk
// ./output-artifacts/example-ubuntu.mf
// ./output-artifacts/example-ubuntu.ovf
// ```
type ExportConfig struct {
	// The name of the exported image in Open Virtualization Format (OVF).
	//
	// -> **Note:** The name of the virtual machine with the `.ovf` extension is
	// used if this option is not specified.
	Name string `mapstructure:"name"`
	// Forces the export to overwrite existing files. Defaults to `false`.
	// If set to `false`, an error is returned if the file(s) already exists.
	Force bool `mapstructure:"force"`
	// Include additional image files that are  associated with the virtual
	// machine. Defaults to `false`. For example, `.nvram` and `.log` files.
	ImageFiles bool `mapstructure:"image_files"`
	// The hash algorithm to use when generating a manifest file. Defaults to
	// `sha256`.
	//
	// The available options for this setting are: 'none', 'sha1', 'sha256', and
	// 'sha512'.
	//
	// --> **Tip:** Use `none` to disable the creation of a manifest file.
	Manifest string `mapstructure:"manifest"`
	// The path to the directory where the exported image will be saved.
	OutputDir OutputConfig `mapstructure:",squash"`
	// Advanced image export options. Available options include:
	// * `mac` - MAC address is exported for each Ethernet device.
	// * `uuid` - UUID is exported for the virtual machine.
	// * `extraconfig` - Extra configuration options are exported for the
	//   virtual machine.
	// * `nodevicesubtypes` - Resource subtypes for CD/DVD drives, floppy
	//   drives, and SCSI controllers are not exported.
	//
	// For example, adding the following export configuration option outputs the
	// MAC addresses for each Ethernet device in the OVF descriptor:
	//
	// HCL Example:
	//
	// ```hcl
	// ...
	//   export {
	//     options = ["mac"]
	//   }
	// ```
	//
	// JSON Example:
	//
	// ```json
	// ...
	//   "export": {
	//     "options": ["mac"]
	//   },
	// ```
	Options []string `mapstructure:"options"`
	// The output format for the exported virtual machine image.
	// Defaults to `ovf`. Available options include `ovf` and `ova`.
	//
	// When set to `ova`, the image is first exported using Open Virtualization
	// Format (`.ovf`) and then converted to an Open Virtualization Archive
	// (`.ova`) using the VMware [Open Virtualization Format Tool](https://developer.broadcom.com/tools/open-virtualization-format-ovf-tool/latest)
	// (ovftool). The intermediate files are removed after the conversion.
	//
	// ~> **Note:** To use the `ova` format option, VMware ovftool must be
	// installed on the Packer host and accessible in either the system `PATH`
	// or the user's `PATH`.
	Format string `mapstructure:"output_format"`
}

// Supported hash algorithms.
var sha = map[string]func() hash.Hash{
	"none":   nil,
	"sha1":   sha1.New,
	"sha256": sha256.New,
	"sha512": sha512.New,
}

func (c *ExportConfig) Prepare(ctx *interpolate.Context, lc *LocationConfig, pc *common.PackerConfig) []error {
	var errs *packersdk.MultiError

	errs = packersdk.MultiErrorAppend(errs, c.OutputDir.Prepare(ctx, pc)...)

	// Default the name to the name of the virtual machine if not specified.
	if c.Name == "" {
		c.Name = lc.VMName
	}

	// Check if the output directory exists.
	if err := os.MkdirAll(c.OutputDir.OutputDir, *c.OutputDir.DirPerm); err != nil {
		errs = packersdk.MultiErrorAppend(errs, errors.Wrap(err, "unable to make directory for export"))
	}

	// Check if the export format is valid.
	switch c.Format {
	case "", "ovf":
		// Set the target path for the target OVF file.
		target := getTarget(c.OutputDir.OutputDir, c.Name, ".ovf")

		// If the export is not forced, check if the file already exists.
		if !c.Force {
			if _, err := os.Stat(target); err == nil {
				errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("force export disabled, file already exists: %s", target))
			} else if !errors.Is(err, os.ErrNotExist) {
				errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("unable to check if file exists: %s", target))
			}
		}
	case "ova":
		ovftool := getOvftool()

		// Check if ovftool is available.
		_, err := exec.LookPath(ovftool)
		if err != nil {
			return []error{errors.Wrap(err, ovftool+" is either not installed or not in path.")}
		}

		// Set the target path for the OVA file.
		ovaTarget := getTarget(c.OutputDir.OutputDir, c.Name, ".ova")

		// Check if the OVA file already exists.
		if !c.Force {
			// Check if the OVA file already exists. If it does, remove it.
			_, err := os.Stat(ovaTarget)
			if err == nil {
				return []error{fmt.Errorf("force export disabled, file already exists: %s", ovaTarget)}
			} else if !errors.Is(err, os.ErrNotExist) {
				return []error{fmt.Errorf("unable to check if file exists: %s", ovaTarget)}
			}
		}
	default:
		return []error{fmt.Errorf("unsupported output format: %s. available options include 'ovf' and 'ova'", c.Format)}
	}

	// Check if the hash algorithm is supported.
	switch c.Manifest {
	case "":
		c.Manifest = "sha256"
	case "none", "sha1", "sha256", "sha512":
		// Supported hash algorithms; do nothing.
	default:
		errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("unsupported hash: %s. available options include 'none', 'sha1', 'sha256', and 'sha512'", c.Manifest))
	}

	if errs != nil && len(errs.Errors) > 0 {
		return errs.Errors
	}

	return nil
}

// Returns the target path for the exported image.
func getTarget(dir string, name string, ext string) string {
	return filepath.Join(dir, name+ext)
}

// Returns the name of the ovftool executable based on the operating system.
func getOvftool() string {
	if runtime.GOOS == "windows" {
		return OvftoolWindows
	}
	return "ovftool"
}

type StepExport struct {
	Name       string
	Force      bool
	ImageFiles bool
	Manifest   string
	OutputDir  string
	Options    []string
	Format     string
	mf         bytes.Buffer
}

func (s *StepExport) Cleanup(multistep.StateBag) {
}

func validateExportOptions(requested []string, available []types.OvfOptionInfo) error {
	if len(requested) == 0 {
		return nil
	}

	availableNames := make([]string, 0, len(available))
	availableSet := make(map[string]struct{}, len(available))
	for _, opt := range available {
		availableNames = append(availableNames, opt.Option)
		availableSet[opt.Option] = struct{}{}
	}

	var unknown []string
	for _, option := range requested {
		if _, ok := availableSet[option]; !ok {
			unknown = append(unknown, option)
		}
	}
	if len(unknown) == 0 {
		return nil
	}

	return fmt.Errorf("unknown export options: %s. available options: %s",
		strings.Join(unknown, ", "),
		strings.Join(availableNames, ", "))
}

func (s *StepExport) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	vm := state.Get("vm").(exportStepVM)

	cdp := types.OvfCreateDescriptorParams{
		Name: s.Name,
	}

	m := vm.NewOvfManager()
	if len(s.Options) > 0 {
		exportOptions, err := vm.GetOvfExportOptions(m)
		if err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}
		if err := validateExportOptions(s.Options, exportOptions); err != nil {
			ui.Errorf("%s", err)
			state.Put("error", err)
			return multistep.ActionHalt
		}
		cdp.ExportOption = append(cdp.ExportOption, s.Options...)
	}

	// Start exporting the virtual machine image to Open Virtualization Format.
	ui.Say("Exporting to Open Virtualization Format (OVF)...")
	lease, err := vm.Export()
	if err != nil {
		state.Put("error", errors.Wrap(err, "error exporting virtual machine"))
		return multistep.ActionHalt
	}

	info, err := lease.Wait(ctx, nil)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	u := lease.StartUpdater(ctx, info)
	defer u.Done()

	for _, i := range info.Items {
		if !s.include(&i) {
			continue
		}

		if !strings.HasPrefix(i.Path, s.Name) {
			i.Path = s.Name + "-" + i.Path
		}

		file := i.File()

		// Download the virtual machine image in Open Virtualization Format.
		ui.Sayf("Downloading %s...", file.Path)
		size, err := s.Download(ctx, lease, i)
		if err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}

		// Set the file size in the Open Virtualization Format descriptor.
		file.Size = size

		// Export the virtual machine image in Open Virtualization Format.
		ui.Sayf("Exporting %s...", file.Path)
		cdp.OvfFiles = append(cdp.OvfFiles, file)
	}

	if err = lease.Complete(ctx); err != nil {
		state.Put("error", errors.Wrap(err, "unable to complete lease"))
		return multistep.ActionHalt
	}

	desc, err := vm.CreateDescriptor(m, cdp)
	if err != nil {
		state.Put("error", errors.Wrap(err, "unable to create descriptor"))
		return multistep.ActionHalt
	}

	target := getTarget(s.OutputDir, s.Name, ".ovf")
	file, err := os.Create(target)
	if err != nil {
		state.Put("error", errors.Wrap(err, "unable to create file"))
		return multistep.ActionHalt
	}

	var w io.Writer = file
	h, ok := s.newHash()
	if ok {
		w = io.MultiWriter(file, h)
	}

	// Write the Open Virtualization Format descriptor.
	ui.Sayf("Writing OVF descriptor %s...", s.Name+".ovf")
	_, err = io.WriteString(w, desc.OvfDescriptor)
	if err != nil {
		state.Put("error", errors.Wrap(err, "unable to write ovf descriptor"))
		return multistep.ActionHalt
	}

	if err = file.Close(); err != nil {
		state.Put("error", errors.Wrap(err, "unable to close ovf descriptor"))
		return multistep.ActionHalt
	}

	// Manifest file will not be created. Continue to the next step.
	if s.Manifest == "none" {
		return multistep.ActionContinue
	}

	// Create a manifest file with the specified hash algorithm.
	ui.Sayf("Writing %s manifest %s...", strings.ToUpper(s.Manifest), s.Name+".mf")
	s.addHash(filepath.Base(target), h)

	file, err = os.Create(filepath.Join(s.OutputDir, s.Name+".mf"))
	if err != nil {
		state.Put("error", errors.Wrap(err, "unable to create manifest"))
		return multistep.ActionHalt
	}

	_, err = io.Copy(file, &s.mf)
	if err != nil {
		state.Put("error", errors.Wrap(err, "unable to write to manifest"))
		return multistep.ActionHalt
	}

	err = file.Close()
	if err != nil {
		state.Put("error", errors.Wrap(err, "unable to close the manifest"))
		return multistep.ActionHalt
	}

	// Check the export format to determine if the image should be converted.
	switch s.Format {
	case "", "ovf":
		ui.Sayf("Completed export to Open Virtualization Format (OVF): %s", s.Name+".ovf")
		return multistep.ActionContinue
	case "ova":
		ovftool := getOvftool()
		ovaTarget := getTarget(s.OutputDir, s.Name, ".ova")

		// If the OVA file already exists, remove it.
		if s.Force {
			_, err := os.Stat(ovaTarget)
			if err == nil {
				ui.Sayf("Force export enabled; removing existing OVA file: %s...", s.Name+".ova")
				err := os.Remove(ovaTarget)
				if err != nil {
					state.Put("error", errors.Wrap(err, "unable to remove existing ova file"))
					return multistep.ActionHalt
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				state.Put("error", errors.Wrap(err, "unable to check if ova file exists"))
				return multistep.ActionHalt
			}
		}

		// Convert the Open Virtualization Format (OVF) to Open Virtualization
		// Archive (OVA).
		ui.Say("Converting to Open Virtualization Archive (OVA)...")
		cmd := exec.Command(ovftool, target, ovaTarget)
		err = cmd.Run()
		if err != nil {
			state.Put("error", errors.Wrap(err, "unable to convert ovf to ova"))
			return multistep.ActionHalt
		}

		// Check if the OVA file exists.
		_, err = os.Stat(ovaTarget)
		if os.IsNotExist(err) {
			state.Put("error", errors.New("unable to convert ovf to ova; ova file not found"))
			return multistep.ActionHalt
		}

		// Clean up the files used for the conversion.
		ui.Say("Removing intermediate files...")

		// Removes the .vmdk files.
		for _, file := range cdp.OvfFiles {
			absPath := filepath.Join(s.OutputDir, file.Path)
			ui.Sayf("Removing %s...", file.Path)
			err := os.Remove(absPath)
			if err != nil {
				ui.Sayf("Unable to remove file %s: %s", file.Path, err)
			}
		}

		// Removes the .mf, .ovf, .nvram, and .log files.
		for _, ext := range []string{".mf", ".ovf", ".nvram", ".log"} {
			filePath := filepath.Join(s.OutputDir, s.Name+ext)
			ui.Sayf("Removing %s...", s.Name+ext)
			err := os.Remove(filePath)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				ui.Sayf("Unable to remove file %s: %s", s.Name+ext, err)
			}
		}

		ui.Say("Completed removing intermediate files.")
		ui.Sayf("Completed export to Open Virtualization Archive (OVA): %s", s.Name+".ova")
	}
	return multistep.ActionContinue
}

func (s *StepExport) include(item *nfc.FileItem) bool {
	if s.ImageFiles {
		return true
	}
	return filepath.Ext(item.Path) == ".vmdk"
}

func (s *StepExport) newHash() (hash.Hash, bool) {
	// Check if the hash function is nil to handle the 'none' case.
	if h, ok := sha[s.Manifest]; ok && h != nil {
		return h(), true
	}
	return nil, false
}

func (s *StepExport) addHash(p string, h hash.Hash) {
	_, _ = fmt.Fprintf(&s.mf, "%s(%s)= %x\n", strings.ToUpper(s.Manifest), p, h.Sum(nil))
}

func (s *StepExport) Download(ctx context.Context, lease *nfc.Lease, item nfc.FileItem) (int64, error) {
	path := filepath.Join(s.OutputDir, item.Path)
	opts := soap.Download{}

	if h, ok := s.newHash(); ok {
		opts.Writer = h
		defer s.addHash(item.Path, h)
	}

	err := lease.DownloadFile(ctx, path, item, opts)
	if err != nil {
		return 0, err
	}

	f, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return f.Size(), err
}
