// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package supervisor_test

import (
	"bytes"
	"context"
	"log"
	"os"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/supervisor"
)

func TestConnectSupervisor_Prepare(t *testing.T) {
	// Check when non-existing kubeconfig file is provided.
	config := &supervisor.ConnectSupervisorConfig{
		KubeconfigPath: "non-existing-file",
	}
	if err := config.Prepare(); err == nil {
		t.Fatalf("config prepare should fail by non-existing kubeconfig file")
	}

	// Check when an invalid kubeconfig file is provided.
	fakeFile, err := os.CreateTemp(t.TempDir(), "invalid-kubeconfig-file")
	if err != nil {
		t.Fatalf("Failed to create an invalid kubeconfig file: %v", err)
	}
	defer func() {
		if err := fakeFile.Close(); err != nil {
			log.Printf("[WARN] Failed to close file: %v", err)
		}
	}()

	config.KubeconfigPath = fakeFile.Name()
	if err := config.Prepare(); err == nil {
		t.Fatalf("config prepare should fail by an invalid kubeconfig file")
	}

	// Check kubeconfig path value when the KUBECONFIG env var is set.
	config.KubeconfigPath = ""
	validKubeconfigPath := getTestKubeconfigFile(t, "test-ns").Name()
	t.Setenv(clientcmd.RecommendedConfigPathEnvVar, validKubeconfigPath)
	if errs := config.Prepare(); len(errs) != 0 {
		t.Fatalf("config config prepare should not fail: %v", errs)
	}
	if config.KubeconfigPath != validKubeconfigPath {
		t.Fatalf("config.KubeconfigPath should be %q, but returned %q",
			validKubeconfigPath, config.KubeconfigPath)
	}

	// Check if Supervisor namespace is set from the given kubeconfig file context.
	config.SupervisorNamespace = ""
	if errs := config.Prepare(); len(errs) != 0 {
		t.Fatalf("unexpected failure: expected success, but failed: %s", errs[0])
	}
	if config.SupervisorNamespace != "test-ns" {
		t.Errorf("unexpected result: expected %q, but returned %q", "test-ns", config.SupervisorNamespace)
	}
}

func TestConnectSupervisor_Run(t *testing.T) {
	// Set up required config for running the step.
	testFile := getTestKubeconfigFile(t, "test-ns")
	config := &supervisor.ConnectSupervisorConfig{
		KubeconfigPath:      testFile.Name(),
		SupervisorNamespace: "test-ns",
	}
	step := supervisor.StepConnectSupervisor{
		Config: config,
	}
	testWriter := new(bytes.Buffer)
	state := newBasicTestState(testWriter)

	// Mock the InitKubeClientFunc as controller-client always requires a valid kubeconfig to initialize.
	originalFunc := supervisor.InitKubeClientFunc
	defer func() {
		supervisor.InitKubeClientFunc = originalFunc
	}()
	supervisor.InitKubeClientFunc = func(s *supervisor.StepConnectSupervisor) (client.WithWatch, error) {
		return client.WithWatch(nil), nil
	}

	action := step.Run(context.Background(), state)
	if action == multistep.ActionHalt {
		if rawErr, ok := state.GetOk("error"); ok {
			t.Errorf("unexpected error: %s", rawErr.(error))
		}
		t.Fatalf("unexpected action: expected '%#v', but returned '%#v'", multistep.ActionContinue, action)
	}

	// Check if all the required states are set after the step is run.
	if err := supervisor.CheckRequiredStates(state,
		supervisor.StateKeyKubeClient,
		supervisor.StateKeySupervisorNamespace,
	); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Check if the Supervisor namespace value is set correctly in the state.
	namespace := state.Get(supervisor.StateKeySupervisorNamespace)
	if namespace != "test-ns" {
		t.Errorf(`Unexpected result: expected %q to be 'test-ns', but returned %q`, supervisor.StateKeySupervisorNamespace, namespace)
	}

	// Check the output lines from the step runs.
	expectedLines := []string{
		"Connecting to Supervisor cluster...",
		"Successfully connected to Supervisor cluster",
	}
	checkOutputLines(t, testWriter, expectedLines)
}
