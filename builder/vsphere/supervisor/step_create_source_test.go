// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package supervisor_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/supervisor"
)

func TestCreateSource_Prepare(t *testing.T) {
	// Check error output when missing the required config.
	config := &supervisor.CreateSourceConfig{}
	var actualErrs []error
	if actualErrs = config.Prepare(); len(actualErrs) == 0 {
		t.Fatal("unexpected success: expected failure")
	}

	expectedErrs := []error{
		fmt.Errorf("'class_name' is required for creating the source VM"),
		fmt.Errorf("'storage_class' is required for creating the source VM"),
	}
	if !reflect.DeepEqual(actualErrs, expectedErrs) {
		t.Fatalf("unexpected error: expected '%s', but returned '%s'", expectedErrs, actualErrs)
	}

	// Check error output when providing invalid bootstrap configs.
	expectedErrs = []error{
		fmt.Errorf("'bootstrap_provider' must be one of %q, %q, %q",
			supervisor.ProviderCloudInit, supervisor.ProviderSysprep, supervisor.ProviderVAppConfig),
	}
	config = &supervisor.CreateSourceConfig{
		ImageName:         "fake-image",
		ClassName:         "fake-class",
		StorageClass:      "fake-storage-class",
		BootstrapProvider: "fake-bootstrap-provider",
	}
	if actualErrs = config.Prepare(); len(actualErrs) == 0 {
		t.Fatalf("unexpected success: expected failure")
	}
	if !reflect.DeepEqual(actualErrs, expectedErrs) {
		t.Fatalf("unexpected error: expected '%s', but returned '%s'", expectedErrs, actualErrs)
	}

	expectedErrs = []error{
		fmt.Errorf("'bootstrap_data_file' is required when 'bootstrap_provider' is %q", "Sysprep"),
	}
	config.BootstrapProvider = "Sysprep"
	if actualErrs = config.Prepare(); len(actualErrs) == 0 {
		t.Fatalf("unexpected success: expected failure")
	}
	if !reflect.DeepEqual(actualErrs, expectedErrs) {
		t.Fatalf("unexpected error: expected '%s', but returned '%s'", expectedErrs, actualErrs)
	}

	// Check default values for the optional configs.
	config = &supervisor.CreateSourceConfig{
		ImageName:    "fake-image",
		ClassName:    "fake-class",
		StorageClass: "fake-storage-class",
	}
	if actualErrs = config.Prepare(); len(actualErrs) != 0 {
		t.Fatalf("unexpected failure: expected success, but failed: %v", actualErrs)
	}
	if !strings.HasPrefix(config.SourceName, supervisor.DefaultSourceNamePrefix) {
		t.Errorf("expected default SourceName has prefix %s, got %s",
			supervisor.DefaultSourceNamePrefix, config.SourceName)
	}
	if config.BootstrapProvider != supervisor.ProviderCloudInit {
		t.Errorf("expected default BootstrapProvider %s, got %s",
			supervisor.ProviderCloudInit, config.BootstrapProvider)
	}
}

func TestCreateSource_RunDefaultOVF(t *testing.T) {
	// Initialize the step with required configs.
	config := &supervisor.CreateSourceConfig{
		ClassName:         "test-class",
		StorageClass:      "test-storage-class",
		SourceName:        "test-source",
		BootstrapProvider: supervisor.ProviderCloudInit,
	}
	commConfig := &communicator.Config{
		Type: "ssh",
		SSH: communicator.SSH{
			SSHUsername: "test-username",
			SSHPort:     123,
		},
	}
	step := &supervisor.StepCreateSource{
		Config:             config,
		CommunicatorConfig: commConfig,
	}

	// Set up required state for running this step.
	testNamespace := "test-namespace"
	testVMI := &vmopv1.VirtualMachineImage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-image",
			Namespace: testNamespace,
		},
		Status: vmopv1.VirtualMachineImageStatus{
			Type: "OVF",
		},
	}
	kubeClient := newFakeKubeClient(testVMI)
	testWriter := new(bytes.Buffer)
	state := newBasicTestState(testWriter)
	state.Put(supervisor.StateKeyKubeClient, kubeClient)
	state.Put(supervisor.StateKeySupervisorNamespace, testNamespace)

	// Check error if image name is not specified.
	ctx := context.Background()
	action := step.Run(ctx, state)
	if action != multistep.ActionHalt {
		t.Fatal("Step should halt")
	}
	expectedError := "the image name should be specified in config 'image_name' or generated from image import"
	if rawErr, ok := state.GetOk("error"); ok {
		if !strings.Contains(rawErr.(error).Error(), expectedError) {
			t.Errorf("expected error contains %v, but got %v", expectedError, rawErr.(error).Error())
		}
	}

	// Step should not halt after specifying image name and the imported image name.
	importedImageName := "imported-image"
	config.ImageName = "test-image"
	state.Put(supervisor.StateKeyImportedImageName, importedImageName)
	action = step.Run(ctx, state)
	if action == multistep.ActionHalt {
		if rawErr, ok := state.GetOk("error"); ok {
			t.Errorf("unexpected error: %s", rawErr.(error))
		}
		t.Fatalf("unexpected result: expected '%#v', but returned '%#v'", multistep.ActionContinue, action)
	}

	// Check if the K8s Secret object is created with expected spec.
	objKey := client.ObjectKey{
		Namespace: testNamespace,
		Name:      config.SourceName,
	}
	secretObj := &corev1.Secret{}
	if err := kubeClient.Get(ctx, objKey, secretObj); err != nil {
		t.Fatalf("Failed to get the expected Secret object, err: %s", err)
	}
	if secretObj.StringData["user-data"] == "" {
		t.Errorf("Expected the Secret object to be created with user-data, got: %v", secretObj)
	}

	// Check if the source VM object is created with expected spec.
	vmObj := &vmopv1.VirtualMachine{}
	if err := kubeClient.Get(ctx, objKey, vmObj); err != nil {
		t.Fatalf("Failed to get the expected VM object, err: %s", err)
	}
	if vmObj.Name != "test-source" {
		t.Errorf("Expected VM name to be 'test-vm', got %q", vmObj.Name)
	}
	if vmObj.Namespace != "test-namespace" {
		t.Errorf("Expected VM namespace to be 'test-namespace', got %q", vmObj.Namespace)
	}
	if vmObj.Spec.ImageName != "test-image" {
		t.Errorf("Expected VM image name to be 'test-image', got %q", vmObj.Spec.ImageName)
	}
	if vmObj.Spec.ClassName != "test-class" {
		t.Errorf("Expected VM class name to be 'test-class', got %q", vmObj.Spec.ClassName)
	}
	if vmObj.Spec.StorageClass != "test-storage-class" {
		t.Errorf("Expected VM storage class to be 'test-storage-class', got %q", vmObj.Spec.StorageClass)
	}
	if c := vmObj.Spec.Bootstrap.CloudInit; c == nil || c.RawCloudConfig == nil {
		t.Errorf("Expected VM bootstrap to be set with raw cloud config, got %v", c)
	}
	selectorLabelVal := vmObj.Labels[supervisor.VMSelectorLabelKey]
	if selectorLabelVal != "test-source" {
		t.Errorf("Expected source VM label %q to be 'test-source', got %q", supervisor.VMSelectorLabelKey, selectorLabelVal)
	}

	// Check if the source VMService object is created with expected spec.
	vmServiceObj := &vmopv1.VirtualMachineService{}
	if err := kubeClient.Get(ctx, objKey, vmServiceObj); err != nil {
		t.Fatalf("Failed to get the expected VMService object, err: %s", err)
	}
	if vmServiceObj.Name != "test-source" {
		t.Errorf("Expected VMService name to be 'test-source', got %q", vmServiceObj.Name)
	}
	if vmServiceObj.Namespace != "test-namespace" {
		t.Errorf("Expected VMService namespace to be 'test-namespace', got %q", vmServiceObj.Namespace)
	}
	if vmServiceObj.Spec.Type != "LoadBalancer" {
		t.Errorf("Expected VMService type to be 'LoadBalancer', got %q", vmServiceObj.Spec.Type)
	}
	ports := vmServiceObj.Spec.Ports
	if len(ports) == 0 || ports[0].Port != 123 || ports[0].TargetPort != 123 {
		t.Errorf("Expected VMService Port and TargetPort to be '123', got %v", ports)
	}
	selectorMap := vmServiceObj.Spec.Selector
	if val, ok := selectorMap[supervisor.VMSelectorLabelKey]; !ok || val != "test-source" {
		t.Errorf("Expected VMService selector %q to be 'test-source', got %q", supervisor.VMSelectorLabelKey, val)
	}

	// Check if all the required states are set correctly after the step is run.
	sourceName := state.Get(supervisor.StateKeySourceName)
	if sourceName != "test-source" {
		t.Errorf("State %q should be 'test-source', but returned %q", supervisor.StateKeySourceName, sourceName)
	}
	if state.Get(supervisor.StateKeyVMCreated) != true {
		t.Errorf("State %q should be 'true'", supervisor.StateKeyVMCreated)
	}
	if state.Get(supervisor.StateKeyVMServiceCreated) != true {
		t.Errorf("State %q should be 'true'", supervisor.StateKeyVMServiceCreated)
	}
	if state.Get(supervisor.StateKeyOVFBootstrapSecretCreated) != true {
		t.Errorf("State %q should be 'true'", supervisor.StateKeyOVFBootstrapSecretCreated)
	}

	// Check the output lines from the step runs.
	expectedOutput := []string{
		fmt.Sprintf("The configured image with name %s will be used to create the source VirtualMachine object instead of the imported image %s", config.ImageName, importedImageName),
		fmt.Sprintf("Creating source objects with name %q in namespace %q", config.SourceName, testNamespace),
		fmt.Sprintf("Checking source VM image %q", config.ImageName),
		"Found namespace scoped VM image of type \"OVF\"",
		"Deploying VM from OVF image",
		"Using default cloud-init user data as the 'bootstrap_data_file' is not specified",
		"Creating a Secret object for OVF VM bootstrap",
		fmt.Sprintf("Creating a VM object with bootstrap provider %q", config.BootstrapProvider),
		"Creating a VirtualMachineService object for network connection",
		"Finished creating all required source objects",
	}
	checkOutputLines(t, testWriter, expectedOutput)
}

func TestCreateSource_RunDefaultISO(t *testing.T) {
	// Initialize the step with required configs.
	config := &supervisor.CreateSourceConfig{
		ImageName:       "test-image",
		ClassName:       "test-class",
		StorageClass:    "test-storage-class",
		SourceName:      "test-source",
		IsoBootDiskSize: "100Gi",
		GuestOSType:     "test-guest-id",
	}
	commConfig := &communicator.Config{
		Type: "ssh",
		SSH: communicator.SSH{
			SSHUsername: "test-username",
			SSHPort:     123,
		},
	}
	step := &supervisor.StepCreateSource{
		Config:             config,
		CommunicatorConfig: commConfig,
	}

	// Set up required state for running this step.
	testNamespace := "test-namespace"
	testVMI := &vmopv1.VirtualMachineImage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-image",
			Namespace: testNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "VirtualMachineImage",
		},
		Status: vmopv1.VirtualMachineImageStatus{
			Type: "ISO",
		},
	}
	kubeClient := newFakeKubeClient(testVMI)
	testWriter := new(bytes.Buffer)
	state := newBasicTestState(testWriter)
	state.Put(supervisor.StateKeyKubeClient, kubeClient)
	state.Put(supervisor.StateKeySupervisorNamespace, testNamespace)

	ctx := context.Background()
	if action := step.Run(ctx, state); action == multistep.ActionHalt {
		if rawErr, ok := state.GetOk("error"); ok {
			t.Errorf("unexpected error: %s", rawErr.(error))
		}
		t.Fatalf("unexpected result: expected '%#v', but returned '%#v'", multistep.ActionContinue, action)
	}

	// Check if the K8s PVC object is created with expected spec.
	objKey := client.ObjectKey{
		Namespace: testNamespace,
		Name:      config.SourceName,
	}
	pvc := &corev1.PersistentVolumeClaim{}
	if err := kubeClient.Get(ctx, objKey, pvc); err != nil {
		t.Fatalf("Failed to get the expected PVC object, err: %s", err)
	}
	if pvc.Spec.Resources.Requests.Storage().String() != config.IsoBootDiskSize {
		t.Errorf("Expected PVC storage size to be '%q', got %q", config.IsoBootDiskSize, pvc.Spec.Resources.Requests.Storage().String())
	}
	if *pvc.Spec.StorageClassName != config.StorageClass {
		t.Errorf("Expected PVC storage class to be '%q', got %q", config.StorageClass, *pvc.Spec.StorageClassName)
	}
	if pvc.Spec.AccessModes[0] != "ReadWriteOnce" {
		t.Errorf("Expected PVC access mode to be 'ReadWriteOnce', got %q", pvc.Spec.AccessModes[0])
	}

	// Check if the source VM object is created with expected spec.
	vmObj := &vmopv1.VirtualMachine{}
	if err := kubeClient.Get(ctx, objKey, vmObj); err != nil {
		t.Fatalf("Failed to get the expected VM object, err: %s", err)
	}
	if vmObj.Spec.ClassName != config.ClassName {
		t.Errorf("Expected VM class name to be '%q', got %q", config.ClassName, vmObj.Spec.ClassName)
	}
	if vmObj.Spec.StorageClass != config.StorageClass {
		t.Errorf("Expected VM storage class to be '%q', got %q", config.StorageClass, vmObj.Spec.StorageClass)
	}
	if vmObj.Spec.GuestID != config.GuestOSType {
		t.Errorf("Expected VM guest ID to be '%q', got %q", config.GuestOSType, vmObj.Spec.GuestID)
	}
	// Check if the source VM has the expected volume.
	if len(vmObj.Spec.Volumes) != 1 {
		t.Errorf("Expected VM volumes to be 1, got %d", len(vmObj.Spec.Volumes))
	}
	vol := vmObj.Spec.Volumes[0]
	if vol.PersistentVolumeClaim.ClaimName != config.SourceName {
		t.Errorf("Expected VM boot disk claim name to be '%q', got %q", config.SourceName, vol.PersistentVolumeClaim.ClaimName)
	}
	// Check if the source VM has the expected CD-ROM.
	if len(vmObj.Spec.Cdrom) != 1 {
		t.Errorf("Expected VM CD-ROM to be 1, got %d", len(vmObj.Spec.Cdrom))
	}
	c := vmObj.Spec.Cdrom[0]
	if c.Image.Kind != "VirtualMachineImage" {
		t.Errorf("Expected VM CD-ROM image kind to be 'VirtualMachineImage', got %q", c.Image.Kind)
	}
	if c.Image.Name != config.ImageName {
		t.Errorf("Expected VM CD-ROM image name to be '%q', got %q", config.ImageName, c.Image.Name)
	}
	if *c.Connected != true {
		t.Errorf("Expected VM CD-ROM connected to be true, got %v", c.Connected)
	}
	if *c.AllowGuestControl != true {
		t.Errorf("Expected VM CD-ROM allow guest control to be true, got %v", c.AllowGuestControl)
	}

	// Check the output lines from the step runs.
	expectedOutput := []string{
		fmt.Sprintf("Creating source objects with name %q in namespace %q", config.SourceName, testNamespace),
		fmt.Sprintf("Checking source VM image %q", config.ImageName),
		"Found namespace scoped VM image of type \"ISO\"",
		"Deploying VM from ISO image",
		"Creating a PVC object for ISO VM boot disk",
		"Creating a VM object with PVC and CD-ROM attached",
		"Creating a VirtualMachineService object for network connection",
		"Finished creating all required source objects",
	}
	checkOutputLines(t, testWriter, expectedOutput)
}

func TestCreateSource_RunCustomBootstrap(t *testing.T) {
	// Initialize the step with required configs.
	config := &supervisor.CreateSourceConfig{
		ImageName:         "test-image",
		ClassName:         "test-class",
		StorageClass:      "test-storage-class",
		SourceName:        "test-source",
		BootstrapProvider: supervisor.ProviderSysprep,
	}
	commConfig := &communicator.Config{
		Type: "ssh",
		SSH: communicator.SSH{
			SSHUsername: "test-username",
			SSHPort:     123,
		},
	}
	step := &supervisor.StepCreateSource{
		Config:             config,
		CommunicatorConfig: commConfig,
	}

	testDataFile, err := os.CreateTemp(t.TempDir(), "test-data-file")
	if err != nil {
		t.Fatalf("Failed to create temp test data file, err: %s", err)
	}
	defer func() {
		if err := os.Remove(testDataFile.Name()); err != nil {
			log.Printf("[WARN] Failed to remove test data file: %v", err)
		}
		if err := testDataFile.Close(); err != nil {
			log.Printf("[WARN] Failed to close test data file: %v", err)
		}
	}()

	testBootstrapData := []byte("unattend: test-unattend-config")
	if err := os.WriteFile(testDataFile.Name(), testBootstrapData, 0666); err != nil {
		t.Fatalf("Failed to write content to temp file: %v", err)
	}
	step.Config.BootstrapDataFile = testDataFile.Name()

	// Set up required state for running this step.
	testNamespace := "test-namespace"
	testCVMI := &vmopv1.ClusterVirtualMachineImage{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-image",
		},
		Status: vmopv1.VirtualMachineImageStatus{
			Type: "OVF",
		},
	}
	kubeClient := newFakeKubeClient(testCVMI)
	testWriter := new(bytes.Buffer)
	state := newBasicTestState(testWriter)
	state.Put(supervisor.StateKeyKubeClient, kubeClient)
	state.Put(supervisor.StateKeySupervisorNamespace, testNamespace)

	ctx := context.Background()
	if action := step.Run(ctx, state); action == multistep.ActionHalt {
		if rawErr, ok := state.GetOk("error"); ok {
			t.Errorf("unexpected error: %s", rawErr.(error))
		}
		t.Fatalf("unexpected result: expected '%#v', but returned '%#v'", multistep.ActionContinue, action)
	}

	// Check if the K8s Secret object is created with expected bootstrap data.
	objKey := client.ObjectKey{
		Namespace: testNamespace,
		Name:      config.SourceName,
	}
	secretObj := &corev1.Secret{}
	if err := kubeClient.Get(ctx, objKey, secretObj); err != nil {
		t.Fatalf("Failed to get the expected Secret object, err: %s", err)
	}
	if secretObj.StringData["unattend"] != "test-unattend-config" {
		t.Errorf("Expected the Secret object to contain bootstrap data, got: %q", secretObj.StringData)
	}

	// Check if the source VM object is created with expected bootstrap provider.
	vmObj := &vmopv1.VirtualMachine{}
	if err := kubeClient.Get(ctx, objKey, vmObj); err != nil {
		t.Fatalf("Failed to get the expected VM object, err: %s", err)
	}
	if s := vmObj.Spec.Bootstrap.Sysprep; s == nil || s.RawSysprep == nil {
		t.Errorf("Expected VM bootstrap to be set with raw sysprep config, got %v", s)
	}

	// Check the output lines from the step runs.
	expectedOutput := []string{
		fmt.Sprintf("Creating source objects with name %q in namespace %q", config.SourceName, testNamespace),
		fmt.Sprintf("Checking source VM image %q", config.ImageName),
		"Found cluster scoped VM image of type \"OVF\"",
		"Deploying VM from OVF image",
		fmt.Sprintf("Loading bootstrap data from file: %s", testDataFile.Name()),
		"Creating a Secret object for OVF VM bootstrap",
		fmt.Sprintf("Creating a VM object with bootstrap provider %q", config.BootstrapProvider),
		"Creating a VirtualMachineService object for network connection",
		"Finished creating all required source objects",
	}
	checkOutputLines(t, testWriter, expectedOutput)
}

func TestCreateSource_Cleanup(t *testing.T) {
	// Test when 'keep_input_artifact' config is set to true (should skip cleanup).
	config := &supervisor.CreateSourceConfig{
		KeepInputArtifact: true,
	}
	step := &supervisor.StepCreateSource{
		Config: config,
	}
	testWriter := &bytes.Buffer{}
	state := newBasicTestState(testWriter)
	step.Cleanup(state)

	expectedOutput := []string{"Skip cleaning up the source objects as specified in config"}
	checkOutputLines(t, testWriter, expectedOutput)

	// Test when 'keep_input_artifact' config is false (should delete all created source objects).
	step.Config.KeepInputArtifact = false
	step.Config.SourceName = "test-source"
	step.Namespace = "test-namespace"

	sourceObjMeta := metav1.ObjectMeta{
		Name:      "test-source",
		Namespace: "test-namespace",
	}
	kubeClient := newFakeKubeClient(
		&vmopv1.VirtualMachine{
			ObjectMeta: sourceObjMeta,
		},
		&vmopv1.VirtualMachineService{
			ObjectMeta: sourceObjMeta,
		},
		&corev1.Secret{
			ObjectMeta: sourceObjMeta,
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: sourceObjMeta,
		},
	)
	step.KubeClient = kubeClient

	state.Put(supervisor.StateKeyVMCreated, true)
	state.Put(supervisor.StateKeyVMServiceCreated, true)
	state.Put(supervisor.StateKeyOVFBootstrapSecretCreated, true)
	state.Put(supervisor.StateKeyISOBootDiskPVCCreated, true)
	step.Cleanup(state)

	// Check if the source objects are deleted from the cluster.
	ctx := context.Background()
	objKey := client.ObjectKey{
		Name:      "test-source",
		Namespace: "test-namespace",
	}
	if err := kubeClient.Get(ctx, objKey, &corev1.Secret{}); !errors.IsNotFound(err) {
		t.Fatal("expected the Secret object to be deleted")
	}
	if err := kubeClient.Get(ctx, objKey, &vmopv1.VirtualMachine{}); !errors.IsNotFound(err) {
		t.Fatal("expected the VirtualMachine object to be deleted")
	}
	if err := kubeClient.Get(ctx, objKey, &vmopv1.VirtualMachineService{}); !errors.IsNotFound(err) {
		t.Fatal("expected the VirtualMachineService object to be deleted")
	}
	if err := kubeClient.Get(ctx, objKey, &corev1.PersistentVolumeClaim{}); !errors.IsNotFound(err) {
		t.Fatal("expected the PVC object to be deleted")
	}

	// Check the output lines from the step runs.
	expectedOutput = []string{
		"Deleting the VirtualMachineService object from Supervisor cluster",
		"Successfully deleted the VirtualMachineService object",
		"Deleting the VirtualMachine object from Supervisor cluster",
		"Successfully deleted the VirtualMachine object",
		"Deleting the K8s Secret object from Supervisor cluster",
		"Successfully deleted the K8s Secret object",
		"Deleting the PVC object from Supervisor cluster",
		"Successfully deleted the PVC object",
	}
	checkOutputLines(t, testWriter, expectedOutput)
}
