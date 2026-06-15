// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package vsphere_template

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/post-processor/vsphere"
)

type StepMarkAsTemplate struct {
	VMName       string
	TemplateName string
	RemoteFolder string
	ReregisterVM config.Trilean
	Override     bool
}

func (s *StepMarkAsTemplate) getEffectiveTemplateName() string {
	if s.TemplateName != "" {
		return s.TemplateName
	}
	return s.VMName
}

func NewStepMarkAsTemplate(artifact packersdk.Artifact, p *PostProcessor) *StepMarkAsTemplate {
	// Set the default folder.
	remoteFolder := "Discovered virtual machine"

	// If the post-processor configuration's folder is defined, use it as the `remoteFolder`.
	if p.config.Folder != "" {
		remoteFolder = p.config.Folder
	}

	vmName := artifact.Id()

	if artifact.BuilderId() == vsphere.BuilderId {
		id := strings.Split(artifact.Id(), "::")
		remoteFolder = id[1]
		vmName = id[2]
	}

	return &StepMarkAsTemplate{
		VMName:       vmName,
		TemplateName: p.config.TemplateName,
		RemoteFolder: remoteFolder,
		ReregisterVM: p.config.ReregisterVM,
		Override:     p.config.Override,
	}
}

func (s *StepMarkAsTemplate) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	cli := state.Get("client").(*govmomi.Client)
	folder := state.Get("folder").(*object.Folder)
	dcPath := state.Get("dcPath").(string)

	vm, err := findVirtualMachineWithFallback(cli, dcPath, s.VMName, s.RemoteFolder)
	if err != nil {
		state.Put("error", err)
		ui.Errorf("%s", err)
		return multistep.ActionHalt
	}

	templateName := s.getEffectiveTemplateName()

	action, err := handleExistingTemplate(cli, folder, templateName, s.Override, ui)
	if err != nil {
		state.Put("error", err)
		ui.Errorf("%s", err)
		return action
	}

	// Use the MarkAsTemplate method unless the `reregister_vm` is set to `true`.
	if s.ReregisterVM.False() {
		ui.Say("Marking as a template...")

		if err := vm.MarkAsTemplate(context.Background()); err != nil {
			state.Put("error", err)
			ui.Errorf("vm.MarkAsTemplate: %s", err)
			return multistep.ActionHalt
		}

		// Put the template VM in state for tag application
		state.Put("vm", vm)

		return multistep.ActionContinue
	}

	dsPath, err := datastorePath(vm)
	if err != nil {
		state.Put("error", err)
		ui.Errorf("datastorePath: %s", err)
		return multistep.ActionHalt
	}

	host, err := vm.HostSystem(context.Background())
	if err != nil {
		state.Put("error", err)
		ui.Errorf("vm.HostSystem: %s", err)
		return multistep.ActionHalt
	}

	if err := vm.Unregister(context.Background()); err != nil {
		state.Put("error", err)
		ui.Errorf("vm.Unregister: %s", err)
		return multistep.ActionHalt
	}

	artifactName := s.getEffectiveTemplateName()

	if err := unregisterVirtualMachine(cli, folder, artifactName); err != nil {
		state.Put("error", err)
		ui.Errorf("unregisterVirtualMachine: %s", err)
		return multistep.ActionHalt
	}

	// Check if a template with the target name already exists in the destination folder.
	action, err = handleExistingTemplate(cli, folder, artifactName, s.Override, ui)
	if err != nil {
		state.Put("error", err)
		ui.Errorf("%s", err)
		return action
	}

	ui.Say("Registering virtual machine as a template: " + artifactName)

	task, err := folder.RegisterVM(context.Background(), dsPath.String(), artifactName, true, nil, host)
	if err != nil {
		state.Put("error", err)
		ui.Errorf("RegisterVM: %s", err)
		return multistep.ActionHalt
	}

	if err = task.Wait(context.Background()); err != nil {
		state.Put("error", err)
		ui.Errorf("task.Wait: %s", err)
		return multistep.ActionHalt
	}

	// Get the newly registered template for tag application
	template, err := findTemplate(cli, folder, artifactName)
	if err != nil {
		state.Put("error", err)
		ui.Errorf("findTemplate: %s", err)
		return multistep.ActionHalt
	}

	// Put the template VM in state for tag application
	state.Put("vm", template)

	return multistep.ActionContinue
}

func datastorePath(vm *object.VirtualMachine) (*object.DatastorePath, error) {
	devices, err := vm.Device(context.Background())
	if err != nil {
		return nil, err
	}

	disk := ""
	for _, device := range devices {
		if d, ok := device.(*types.VirtualDisk); ok {
			if b, ok := d.Backing.(types.BaseVirtualDeviceFileBackingInfo); ok {
				disk = b.GetVirtualDeviceFileBackingInfo().FileName
			}
			break
		}
	}

	if disk == "" {
		return nil, fmt.Errorf("error finding disk in %q", vm.Name())
	}

	return datastorePathFromDisk(disk, vm.Name())
}

func datastorePathFromDisk(disk, vmName string) (*object.DatastorePath, error) {
	re := regexp.MustCompile(`\[(.*?)\]`)
	matches := re.FindStringSubmatch(disk)
	if len(matches) < 2 || matches[1] == "" {
		return nil, fmt.Errorf("error parsing datastore from disk path %q", disk)
	}
	datastore := matches[1]

	parts := strings.SplitN(disk, " ", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("error parsing virtual machine disk path %q", disk)
	}
	diskPath := strings.TrimSpace(parts[1])
	if diskPath == "" {
		return nil, fmt.Errorf("error parsing virtual machine disk path %q", disk)
	}
	vmxPath := path.Join("/", path.Dir(diskPath), vmName+".vmx")

	return &object.DatastorePath{
		Datastore: datastore,
		Path:      vmxPath,
	}, nil
}

func findVirtualMachine(cli *govmomi.Client, dcPath, name, remoteFolder string) (*object.VirtualMachine, error) {
	si := object.NewSearchIndex(cli.Client)
	fullPath := path.Join(dcPath, "vm", strings.TrimPrefix(remoteFolder, "/"), name)

	ref, err := si.FindByInventoryPath(context.Background(), fullPath)
	if err != nil {
		return nil, err
	}

	if ref == nil {
		return nil, fmt.Errorf("error finding virtual machine at path %s", fullPath)
	}

	vm, ok := ref.(*object.VirtualMachine)
	if !ok {
		return nil, fmt.Errorf("unexpected object type at path %s", fullPath)
	}
	if vm.InventoryPath == "" {
		vm.SetInventoryPath(fullPath)
	}
	return vm, nil
}

func findVirtualMachineWithFallback(cli *govmomi.Client, dcPath, name, remoteFolder string) (*object.VirtualMachine, error) {
	var lastErr error
	tried := map[string]bool{}

	candidates := []string{remoteFolder, "Discovered virtual machine", ""}
	for _, folder := range candidates {
		folder = strings.TrimSpace(folder)
		if tried[folder] {
			continue
		}
		tried[folder] = true

		vm, err := findVirtualMachine(cli, dcPath, name, folder)
		if err == nil {
			return vm, nil
		}
		lastErr = err
	}

	return nil, lastErr
}

func unregisterVirtualMachine(cli *govmomi.Client, folder *object.Folder, name string) error {
	si := object.NewSearchIndex(cli.Client)
	fullPath := path.Join(folder.InventoryPath, name)

	ref, err := si.FindByInventoryPath(context.Background(), fullPath)
	if err != nil {
		return err
	}

	if ref != nil {
		if vm, ok := ref.(*object.VirtualMachine); ok {
			return vm.Unregister(context.Background())
		}
		return fmt.Errorf("object name '%v' already exists", name)
	}
	return nil
}

func findTemplate(cli *govmomi.Client, folder *object.Folder, name string) (*object.VirtualMachine, error) {
	si := object.NewSearchIndex(cli.Client)
	fullPath := path.Join(folder.InventoryPath, name)

	ref, err := si.FindByInventoryPath(context.Background(), fullPath)
	if err != nil {
		return nil, err
	}

	if ref != nil {
		if vm, ok := ref.(*object.VirtualMachine); ok {
			t, err := vm.IsTemplate(context.Background())
			if err != nil {
				return nil, err
			}
			if t {
				return vm, nil
			}
		}
	}
	return nil, nil
}

func handleExistingTemplate(cli *govmomi.Client, folder *object.Folder, templateName string, override bool, ui packersdk.Ui) (multistep.StepAction, error) {
	existingTemplate, err := findTemplate(cli, folder, templateName)
	if err != nil {
		return multistep.ActionHalt, fmt.Errorf("error checking for existing template: %s", err)
	}

	if existingTemplate != nil {
		if !override {
			return multistep.ActionHalt, fmt.Errorf("template '%s' already exists. Set 'override = true' to replace existing templates", templateName)
		}

		ui.Say(fmt.Sprintf("Removing existing template '%s'...", templateName))
		task, err := existingTemplate.Destroy(context.Background())
		if err != nil {
			return multistep.ActionHalt, fmt.Errorf("failed to remove existing template '%s': %s", templateName, err)
		}
		if err = task.Wait(context.Background()); err != nil {
			return multistep.ActionHalt, fmt.Errorf("failed to remove existing template '%s': %s", templateName, err)
		}
		ui.Say(fmt.Sprintf("Successfully removed existing template '%s'", templateName))
	}

	return multistep.ActionContinue, nil
}

func (s *StepMarkAsTemplate) Cleanup(multistep.StateBag) {}
