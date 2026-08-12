// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package supervisor_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	imgregv1a1 "github.com/vmware-tanzu/image-registry-operator-api/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/supervisor"
)

func TestValidatePublish_Prepare(t *testing.T) {
	config := &supervisor.ValidatePublishConfig{}
	if errs := config.Prepare(); len(errs) != 0 {
		t.Fatalf("unexpected failure: expected success, but failed: %v", errs[0])
	}
}

func TestValidatePublish_Run(t *testing.T) {
	// Test with `publish_location_name` not set.
	step := &supervisor.StepValidatePublish{
		Config: &supervisor.ValidatePublishConfig{
			PublishLocationName: "",
		},
	}

	ctx := context.Background()
	testWriter := new(bytes.Buffer)
	state := newBasicTestState(testWriter)

	action := step.Run(ctx, state)
	if action == multistep.ActionHalt {
		if rawErr, ok := state.GetOk("error"); ok {
			t.Errorf("unexpected error: %s", rawErr.(error))
		}
		t.Fatalf("unexpected result: expected '%#v', but returned '%#v'", multistep.ActionContinue, action)
	}

	expectedOutput := []string{
		"Validating VM publish location...",
		"VM publish step will be skipped as the `publish_location_name` config is not set",
	}
	checkOutputLines(t, testWriter, expectedOutput)

	// Test with non-existing "publish_location_name".
	testPublishLocationName := "test-publish-location"
	testNamespace := "test-namespace"
	kubeClient := newFakeKubeClient()
	step.Config.PublishLocationName = testPublishLocationName
	state.Put(supervisor.StateKeySupervisorNamespace, testNamespace)
	state.Put(supervisor.StateKeyKubeClient, kubeClient)

	action = step.Run(ctx, state)
	if action != multistep.ActionHalt {
		t.Fatalf("unexpected result: expected '%#v', but returned '%#v'", multistep.ActionHalt, action)
	}

	expectedError := fmt.Sprintf("%q not found", testPublishLocationName)
	if rawErr, ok := state.GetOk("error"); ok {
		if !strings.Contains(rawErr.(error).Error(), expectedError) {
			t.Errorf("Expected error contains '%v', but returned '%v'", expectedError, rawErr.(error).Error())
		}
	}

	expectedOutput = []string{
		"Validating VM publish location...",
	}
	checkOutputLines(t, testWriter, expectedOutput)

	// Test with existing but non-writable "publish_location_name".
	clObj := &imgregv1a1.ContentLibrary{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      testPublishLocationName,
		},
		Spec: imgregv1a1.ContentLibrarySpec{
			Writable: false,
		},
	}
	kubeClient = newFakeKubeClient(clObj)
	state.Put(supervisor.StateKeyKubeClient, kubeClient)

	action = step.Run(ctx, state)
	if action != multistep.ActionHalt {
		t.Fatalf("unexpected result: expected '%#v', but returned '%#v'", multistep.ActionHalt, action)
	}

	expectedError = fmt.Sprintf("publish location %q is not writable", testPublishLocationName)
	if rawErr, ok := state.GetOk("error"); ok {
		if rawErr.(error).Error() != expectedError {
			t.Errorf("unexpected error: expected '%s', but returned '%s'", expectedError, rawErr.(error).Error())
		}
	}

	expectedOutput = []string{
		"Validating VM publish location...",
	}
	checkOutputLines(t, testWriter, expectedOutput)

	// Test with valid (existing and writable) "publish_location_name".
	clObj.Spec.Writable = true
	kubeClient = newFakeKubeClient(clObj)
	state.Put(supervisor.StateKeyKubeClient, kubeClient)

	action = step.Run(ctx, state)
	if action != multistep.ActionContinue {
		t.Fatalf("unexpected result: expected '%#v', but returned '%#v'", multistep.ActionContinue, action)
	}

	expectedOutput = []string{
		"Validating VM publish location...",
		"VM publish location is valid",
	}
	checkOutputLines(t, testWriter, expectedOutput)
}
