// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package supervisor_test

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha3"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/supervisor"
)

func TestPublishSource_Prepare(t *testing.T) {
	config := &supervisor.PublishSourceConfig{}
	if errs := config.Prepare(); len(errs) != 0 {
		t.Fatalf("unexpected failure: expected success, but failed: %v", errs[0])
	}

	if config.WatchPublishTimeoutSec != supervisor.DefaultWatchPublishTimeoutSec {
		t.Fatalf("Default timeout should be %d, but returned %d",
			supervisor.DefaultWatchPublishTimeoutSec, config.WatchPublishTimeoutSec)
	}
}

func TestStepPublishSource_Run_Skip(t *testing.T) {
	// Initialize the step without `publish_location_name` set.
	config := &supervisor.PublishSourceConfig{
		WatchPublishTimeoutSec: 5,
	}
	step := &supervisor.StepPublishSource{
		Config: config,
	}

	// Set up required state for running this step.
	state := newBasicTestState(new(bytes.Buffer))
	state.Put(supervisor.StateKeyPublishLocationName, "")
	state.Put(supervisor.StateKeySourceName, "test-source")
	state.Put(supervisor.StateKeySupervisorNamespace, "test-ns")
	state.Put(supervisor.StateKeyKubeClient, newFakeKubeClient())
	state.Put(supervisor.StateKeyKeepInputArtifact, true)

	action := step.Run(context.Background(), state)
	if action != multistep.ActionContinue {
		if rawErr, ok := state.GetOk("error"); ok {
			t.Errorf("unexpected error: %s", rawErr.(error))
		}
		t.Fatalf("unexpected result: expected '%#v', but returned '%#v'", multistep.ActionContinue, action)
	}
}

func TestStepPublishSource_Run(t *testing.T) {
	// Initialize the step with `publish_location_name` set.
	config := &supervisor.PublishSourceConfig{
		WatchPublishTimeoutSec: 5,
	}
	step := &supervisor.StepPublishSource{
		Config: config,
	}

	testSourceName := "test-source-name"
	testImageName := "test-image-name"
	testPublishLocationName := "test-publish-location-name"
	testNamespace := "test-namespace"
	testPublishRequestName := "test-publish-request-name"
	VMPublishReqObj := newFakeVMPubReqObj(testNamespace, testPublishRequestName, testPublishLocationName)
	testKubeClient := newFakeKubeClient(VMPublishReqObj)

	// Set up required state for running this step.
	testWriter := new(bytes.Buffer)
	state := newBasicTestState(testWriter)
	state.Put(supervisor.StateKeyPublishLocationName, testPublishLocationName)
	state.Put(supervisor.StateKeySourceName, testSourceName)
	state.Put(supervisor.StateKeySupervisorNamespace, testNamespace)
	state.Put(supervisor.StateKeyKubeClient, testKubeClient)
	state.Put(supervisor.StateKeyKeepInputArtifact, true)

	ctx := context.Background()
	var wg sync.WaitGroup

	wg.Go(func() {
		action := step.Run(ctx, state)
		if action == multistep.ActionHalt {
			if rawErr, ok := state.GetOk("error"); ok {
				t.Errorf("unexpected error: %s", rawErr.(error))
			}
			t.Errorf("unexpected action: expected '%#v', but returned '%#v'", multistep.ActionContinue, action)
		}

		// check if the VirtualMachinePublishRequest object is created with the expected spec.
		objKey := client.ObjectKey{
			Name:      testPublishRequestName,
			Namespace: testNamespace,
		}
		if err := testKubeClient.Get(ctx, objKey, VMPublishReqObj); err != nil {
			t.Errorf("Failed to get the expected VirtualMachinePublishRequest object, err: %s", err)
		}
		if VMPublishReqObj.Name != testPublishRequestName {
			t.Errorf("Expected VirtualMachinePublishRequest name to be '%s', got '%s'",
				testPublishRequestName, VMPublishReqObj.Name)
		}
		if VMPublishReqObj.Namespace != testNamespace {
			t.Errorf("Expected VirtualMachinePublishRequest namespace to be '%s', got '%s'",
				testNamespace, VMPublishReqObj.Namespace)
		}
		if VMPublishReqObj.Spec.Target.Location.Name != testPublishLocationName {
			t.Errorf("Expected VirtualMachinePublishRequest target location to be '%s', got '%s'",
				testPublishLocationName, VMPublishReqObj.Spec.Target.Location.Name)
		}

		expectedOutput := []string{
			"Publishing the source VM to \"test-publish-location-name\"",
			"Creating a VirtualMachinePublishRequest object",
			"Successfully created the VirtualMachinePublishRequest object",
			"Waiting for the VM publish request to complete...",
			"Successfully published the VM to an OVF image \"test-image-name\"",
			"Finished publishing the source VM",
		}
		checkOutputLines(t, testWriter, expectedOutput)
	})

	// Wait for the watch to be established from Builder before updating the fake VirtualMachinePublishRequest resource below.
	for i := 0; i < step.Config.WatchPublishTimeoutSec; i++ {
		if supervisor.IsWatchingVMPublish.Load() {
			break
		}
		time.Sleep(time.Second)
	}

	VMPublishReqObj.Status.Ready = false
	if err := testKubeClient.Update(ctx, VMPublishReqObj); err != nil {
		t.Errorf("Failed to update the VirtualMachinePublishRequest object status ready, err: %s", err)
	}

	VMPublishReqObj.Status.Ready = true
	VMPublishReqObj.Status.ImageName = testImageName
	if err := testKubeClient.Update(ctx, VMPublishReqObj); err != nil {
		t.Errorf("Failed to update the VirtualMachinePublishRequest object status image name, err: %s", err)
	}

	wg.Wait()
}

func TestStepPublishSource_Cleanup(t *testing.T) {
	// Test when 'keep_input_artifact' config is set to true (should skip cleanup).
	step := &supervisor.StepPublishSource{}
	step.KeepInputArtifact = true
	testWriter := new(bytes.Buffer)
	state := newBasicTestState(testWriter)
	state.Put(supervisor.StateKeyVMPublishRequestCreated, true)
	step.Cleanup(state)

	expectedOutput := []string{"Skip cleaning up the VirtualMachinePublishRequest object as specified in config"}
	checkOutputLines(t, testWriter, expectedOutput)

	// Test when 'keep_input_artifact' config is false (should delete the VirtualMachinePublishRequest object).
	step.KeepInputArtifact = false
	step.SourceName = "test-source"
	step.Namespace = "test-namespace"
	vmPubReq := &vmopv1.VirtualMachinePublishRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-source",
			Namespace: "test-namespace",
		},
	}
	fakeClient := newFakeKubeClient(vmPubReq)
	step.KubeWatchClient = fakeClient
	state.Put(supervisor.StateKeyKeepInputArtifact, true)
	state.Put(supervisor.StateKeyVMPublishRequestCreated, true)
	step.Cleanup(state)

	// Check if the source objects are deleted from the cluster.
	ctx := context.Background()
	objKey := client.ObjectKey{
		Name:      "test-source",
		Namespace: "test-namespace",
	}
	if err := fakeClient.Get(ctx, objKey, &vmopv1.VirtualMachinePublishRequest{}); !errors.IsNotFound(err) {
		t.Fatal("expected VirtualMachinePublishRequest object to be deleted")
	}

	// Check the output lines from the step runs.
	expectedOutput = []string{
		"Deleting the VirtualMachinePublishRequest object from Supervisor cluster",
		"Successfully deleted the VirtualMachinePublishRequest object",
	}
	checkOutputLines(t, testWriter, expectedOutput)
}

func newFakeVMPubReqObj(ns, name, publishLocation string) *vmopv1.VirtualMachinePublishRequest {
	return &vmopv1.VirtualMachinePublishRequest{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Spec: vmopv1.VirtualMachinePublishRequestSpec{
			Target: vmopv1.VirtualMachinePublishRequestTarget{
				Location: vmopv1.VirtualMachinePublishRequestTargetLocation{
					Name: publishLocation,
				},
			},
		},
	}
}
