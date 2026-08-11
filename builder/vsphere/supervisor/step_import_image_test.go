// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package supervisor_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	imgregv1 "github.com/vmware-tanzu/image-registry-operator-api/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/supervisor"
)

const (
	testSourceURL      = "https://example.com/example.ovf"
	testSSLCertificate = "-----BEGIN CERTIFICATE-----xxxxx-----END CERTIFICATE-----"
	testTargetLibrary  = "cl-6066c61f7931c5ef9"
	testNamespace      = "test-ns"

	testImportReqName = "test-req-name"
	testImageName     = "test-image-name"
	testCLItemName    = "clitem-d876e13ff4e6d51e2"
)

func TestImportImage_Prepare(t *testing.T) {
	// 1. Prepare() should succeed with required configs.
	config := &supervisor.ImportImageConfig{
		ImportSourceURL:            testSourceURL,
		ImportSourceSSLCertificate: testSSLCertificate,
		ImportTargetLocationName:   testTargetLibrary,
	}
	if actualErrs := config.Prepare(); len(actualErrs) != 0 {
		t.Fatalf("Prepare should NOT fail: %v", actualErrs)
	}

	if config.ImportTargetImageType != "ovf" {
		t.Fatal("The default import target image type should be 'ovf'.")
	}

	if config.WatchImportTimeoutSec != supervisor.DefaultWatchImageImportTimeoutSec {
		t.Fatalf("Default timeout should be %d, but got %d",
			supervisor.DefaultWatchImageImportTimeoutSec, config.WatchImportTimeoutSec)
	}

	if !strings.HasPrefix(config.ImportRequestName, supervisor.ImportRequestDefaultNamePrefix) {
		t.Fatal("The default import request name should start with packer-vsphere-supervisor-import-req-")
	}

	// 2. Prepare() should fail by setting image type other.
	config.ImportTargetImageType = "other"
	actualErrs := config.Prepare()
	if len(actualErrs) != 1 {
		t.Fatalf("Prepare should have failed.")
	}
	expectedErr := "unsupported ImportTargetImageType: other"
	if actualErrs[0].Error() != expectedErr {
		t.Fatalf("expected error is %v, but got %v", expectedErr, actualErrs[0].Error())
	}

	// 3. Prepare() should fail by not setting SSL certificate.
	config.ImportTargetImageType = "ovf"
	config.ImportSourceSSLCertificate = ""
	actualErrs = config.Prepare()
	if len(actualErrs) != 1 {
		t.Fatalf("Prepare should have failed.")
	}
	expectedErr = "config import_source_ssl_certificate is required for https based source urls"
	if actualErrs[0].Error() != expectedErr {
		t.Fatalf("expected error is %v, but got %v", expectedErr, actualErrs[0].Error())
	}

	// 4. Prepare() should fail by not setting target location name.
	config.ImportSourceSSLCertificate = testSSLCertificate
	config.ImportTargetLocationName = ""
	actualErrs = config.Prepare()
	if len(actualErrs) != 1 {
		t.Fatalf("Prepare should have failed.")
	}
	expectedErr = "config import_target_location_name is required for importing image"
	if actualErrs[0].Error() != expectedErr {
		t.Fatalf("expected error is %v, but got %v", expectedErr, actualErrs[0].Error())
	}
}

func TestStepImportImage_Run_Validate(t *testing.T) {
	// 1. Test with `supervisor_namespace` not set.
	config := &supervisor.ImportImageConfig{
		ImportRequestName:        testImportReqName,
		ImportSourceURL:          testSourceURL,
		ImportTargetLocationName: testTargetLibrary,
		ImportTargetImageName:    testImageName,
	}
	step := &supervisor.StepImportImage{
		ImportImageConfig: config,
	}

	ctx := context.TODO()
	testWriter := new(bytes.Buffer)
	state := newBasicTestState(testWriter)

	action := step.Run(ctx, state)
	if action != multistep.ActionHalt {
		t.Fatal("Step should halt")
	}

	if rawErr, ok := state.GetOk("error"); ok {
		if rawErr.(error).Error() != "missing required state: supervisor_namespace" {
			t.Errorf("error running step: %s", rawErr.(error))
		}
	} else {
		t.Fatal("Step should throw an error")
	}

	expectedOutput := []string{
		"error checking required states: missing required state: supervisor_namespace",
		"failed to initialize image import: missing required state: supervisor_namespace",
	}
	checkOutputLines(t, testWriter, expectedOutput)

	// 2. Test with incorrect type of "kube_client".
	state.Put(supervisor.StateKeySupervisorNamespace, testNamespace)
	state.Put(supervisor.StateKeyKubeClient, "kubeClient")

	action = step.Run(ctx, state)
	if action != multistep.ActionHalt {
		t.Fatal("Step should halt")
	}

	expectedError := "failed to cast kube_client to type client.WithWatch"
	if rawErr, ok := state.GetOk("error"); ok {
		if !strings.Contains(rawErr.(error).Error(), expectedError) {
			t.Errorf("expected error contains %v, but got %v", expectedError, rawErr.(error).Error())
		}
	}

	expectedOutput = []string{
		"failed to initialize image import: failed to cast kube_client to type client.WithWatch",
	}
	checkOutputLines(t, testWriter, expectedOutput)

	// 3. Test with non-existing target content library.
	kubeClient := newFakeKubeClient()
	step.ImportImageConfig.ImportSourceURL = testSourceURL
	step.ImportImageConfig.ImportSourceSSLCertificate = testSSLCertificate

	state.Put(supervisor.StateKeySupervisorNamespace, testNamespace)
	state.Put(supervisor.StateKeyKubeClient, kubeClient)
	state.Put(supervisor.StateKeySupervisorNamespace, testNamespace)
	state.Put(supervisor.StateKeyKubeClient, kubeClient)

	action = step.Run(ctx, state)
	if action != multistep.ActionHalt {
		t.Fatal("Step should halt")
	}

	expectedError = fmt.Sprintf("contentlibraries.imageregistry.vmware.com \"%s\" not found", testTargetLibrary)
	if rawErr, ok := state.GetOk("error"); ok {
		if !strings.Contains(rawErr.(error).Error(), expectedError) {
			t.Errorf("expected error contains %v, but got %v", expectedError, rawErr.(error).Error())
		}
	}

	expectedOutput = []string{
		"Validating image import request...",
		fmt.Sprintf("failed to return the content library by name %s in namespace %s", testTargetLibrary, testNamespace),
		fmt.Sprintf("failed to validate import image configs: contentlibraries.imageregistry.vmware.com %q not found", testTargetLibrary),
	}
	checkOutputLines(t, testWriter, expectedOutput)

	// 4. Test with existing but non-allow-import "import_target_location_name".
	clObj := &imgregv1.ContentLibrary{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      testTargetLibrary,
		},
		Spec: imgregv1.ContentLibrarySpec{
			Writable:    true,
			AllowImport: false,
		},
	}
	kubeClient = newFakeKubeClient(clObj)
	state.Put(supervisor.StateKeyKubeClient, kubeClient)

	action = step.Run(ctx, state)
	if action != multistep.ActionHalt {
		t.Fatal("Step should halt")
	}

	expectedError = fmt.Sprintf("import target content library %q is not writable or does not allow import", testTargetLibrary)
	if rawErr, ok := state.GetOk("error"); ok {
		if rawErr.(error).Error() != expectedError {
			t.Errorf("expected error is %v, but got %v", expectedError, rawErr.(error).Error())
		}
	}

	expectedOutput = []string{
		"Validating image import request...",
		fmt.Sprintf("failed to validate import image configs: import target content library %q is not writable or does not allow import", testTargetLibrary),
	}
	checkOutputLines(t, testWriter, expectedOutput)

	// 5. Test with invalid target type.
	clObj.Spec.AllowImport = true
	kubeClient = newFakeKubeClient(clObj)
	state.Put(supervisor.StateKeyKubeClient, kubeClient)
	step.ImportImageConfig.ImportTargetImageType = "other"

	action = step.Run(ctx, state)
	if action != multistep.ActionHalt {
		t.Fatal("Step should halt")
	}
	expectedError = "image type other is not supported"
	if rawErr, ok := state.GetOk("error"); ok {
		if rawErr.(error).Error() != expectedError {
			t.Errorf("expected error is %v, but got %v", expectedError, rawErr.(error).Error())
		}
	}

	expectedOutput = []string{
		"Validating image import request...",
		"failed to validate import image configs: image type other is not supported",
	}
	checkOutputLines(t, testWriter, expectedOutput)
}

func TestStepImportImage_Run(t *testing.T) {
	// Initialize the step with valid configs.
	config := &supervisor.ImportImageConfig{
		WatchImportTimeoutSec:      5,
		ImportRequestName:          testImportReqName,
		ImportSourceURL:            testSourceURL,
		ImportSourceSSLCertificate: testSSLCertificate,
		ImportTargetLocationName:   testTargetLibrary,
		ImportTargetImageType:      "ovf",
		ImportTargetImageName:      testImageName,
	}
	step := &supervisor.StepImportImage{
		ImportImageConfig: config,
	}

	targetLib := &imgregv1.ContentLibrary{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testTargetLibrary,
			Namespace: testNamespace,
		},
		Spec: imgregv1.ContentLibrarySpec{
			UUID:        "968389fb-8e4c-44e7-a450-cd53366e384c",
			Writable:    true,
			AllowImport: true,
		},
	}
	testKubeClient := newFakeKubeClient(targetLib)

	// Set up required state for running this step.
	testWriter := new(bytes.Buffer)
	state := newBasicTestState(testWriter)
	state.Put(supervisor.StateKeySupervisorNamespace, testNamespace)
	state.Put(supervisor.StateKeyKubeClient, testKubeClient)
	state.Put(supervisor.StateKeyKeepInputArtifact, true)

	ctx := context.TODO()
	var wg sync.WaitGroup

	wg.Go(func() {
		action := step.Run(ctx, state)
		if action == multistep.ActionHalt {
			if rawErr, ok := state.GetOk("error"); ok {
				t.Errorf("error running step: %s", rawErr.(error))
			}
			t.Errorf("step should not halt")
		}

		// check if the ContentLibraryItemImportRequest object is created with the expected spec.
		objKey := client.ObjectKey{
			Name:      testImportReqName,
			Namespace: testNamespace,
		}
		importReq := &imgregv1.ContentLibraryItemImportRequest{}
		if err := testKubeClient.Get(ctx, objKey, importReq); err != nil {
			t.Errorf("failed to return the expected ContentLibraryItemImportRequest object, err: %s", err.Error())
		}
		if importReq.Spec.Target.Library.Name != testTargetLibrary {
			t.Errorf("expected ContentLibraryItemImportRequest target library to be '%s', got '%s'",
				testTargetLibrary, importReq.Spec.Target.Library.Name)
		}

		expectedOutput := []string{
			"Validating image import request...",
			"Image import configs are valid.",
			fmt.Sprintf("Importing the source image from %s to %s.", testSourceURL, testTargetLibrary),
			fmt.Sprintf("Creating ContentLibraryItemImportRequest object %s in namespace %s.", testImportReqName, testNamespace),
			fmt.Sprintf("Successfully created the ContentLibraryItemImportRequest object %s.", testImportReqName),
			"Waiting for the image import request to complete...",
			fmt.Sprintf("Successfully imported the image as a content library item &{\"imageregistry.vmware.com/v1alpha1\" \"ContentLibraryItem\" \"%s\"}.", testCLItemName),
			fmt.Sprintf("Finished importing the image from %s to %s.", testSourceURL, testTargetLibrary),
		}
		checkOutputLines(t, testWriter, expectedOutput)
	})

	// Wait for the watch to be established from Builder before updating the fake ContentLibraryItemImportRequest resource below.
	for i := 0; i < step.ImportImageConfig.WatchImportTimeoutSec; i++ {
		step.Mu.Lock()
		if step.IsWatchingImageImport {
			step.Mu.Unlock()
			break
		}
		step.Mu.Unlock()
		time.Sleep(time.Second)
	}

	objKey := client.ObjectKey{
		Name:      testImportReqName,
		Namespace: testNamespace,
	}
	importReq := &imgregv1.ContentLibraryItemImportRequest{}
	if err := testKubeClient.Get(ctx, objKey, importReq); err != nil {
		t.Errorf("failed to return the expected ContentLibraryItemImportRequest object, err: %s", err.Error())
	}

	importReq.Status.Conditions = []imgregv1.Condition{
		{
			Type:   imgregv1.ContentLibraryItemImportRequestComplete,
			Status: corev1.ConditionTrue,
		},
	}
	importReq.Status.ItemRef = &imgregv1.LocalObjectRef{
		Kind:       "ContentLibraryItem",
		APIVersion: supervisor.ImportTargetAPIVersion,
		Name:       testCLItemName,
	}
	if err := testKubeClient.Update(ctx, importReq); err != nil {
		t.Errorf("failed to update the ContentLibraryItemImportRequest object status image name, err: %s", err.Error())
	}

	wg.Wait()
}

// TestStepImportImage_Cleanup_Request tests cleaning request resource only by setting clean_imported_image false.
func TestStepImportImage_Cleanup_Request(t *testing.T) {
	// 1. Test when 'keep_input_artifact' config is set to true (should skip cleanup).
	step := &supervisor.StepImportImage{
		ImportImageConfig: &supervisor.ImportImageConfig{
			KeepImportRequest:  true,
			CleanImportedImage: false,
		},
	}
	testWriter := new(bytes.Buffer)
	state := newBasicTestState(testWriter)
	state.Put(supervisor.StateKeyImageImportRequestCreated, true)
	step.Cleanup(state)

	expectedOutput := []string{"Skipping clean up of the ContentLibraryItemImportRequest object as specified in config."}
	checkOutputLines(t, testWriter, expectedOutput)

	// 2. Test when 'keep_input_artifact' config is false (should delete the ContentLibraryItemImportRequest object).
	step.ImportImageConfig.KeepImportRequest = false
	step.ImportImageConfig = &supervisor.ImportImageConfig{}
	step.ImportImageConfig.ImportRequestName = testImportReqName
	step.Namespace = testNamespace
	importReq := &imgregv1.ContentLibraryItemImportRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testImportReqName,
			Namespace: testNamespace,
		},
	}
	fakeClient := newFakeKubeClient(importReq)
	step.KubeWatchClient = fakeClient
	state.Put(supervisor.StateKeyKeepInputArtifact, true)
	state.Put(supervisor.StateKeyImageImportRequestCreated, true)
	step.Cleanup(state)

	// Check if the ContentLibraryItemImportRequest object is deleted from the cluster.
	ctx := context.TODO()
	objKey := client.ObjectKey{
		Name:      testImportReqName,
		Namespace: testNamespace,
	}
	if err := fakeClient.Get(ctx, objKey, &imgregv1.ContentLibraryItemImportRequest{}); !errors.IsNotFound(err) {
		t.Fatal("Expected the ContentLibraryItemImportRequest object to be deleted")
	}

	// Check the output lines from the step runs.
	expectedOutput = []string{
		fmt.Sprintf("Deleting the ContentLibraryItemImportRequest object %s in namespace %s.", testImportReqName, testNamespace),
		fmt.Sprintf("Successfully deleted the ContentLibraryItemImportRequest object %s in namespace %s.", testImportReqName, testNamespace),
	}
	checkOutputLines(t, testWriter, expectedOutput)
}

// TestStepImportImage_Cleanup_Image tests cleaning imported image only by setting keep_input_artifact true.
func TestStepImportImage_Cleanup_Image(t *testing.T) {
	step := &supervisor.StepImportImage{
		Namespace: testNamespace,
		ImportImageConfig: &supervisor.ImportImageConfig{
			KeepImportRequest: true,
		},
	}
	importedImage := &imgregv1.ContentLibraryItem{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testCLItemName,
			Namespace: testNamespace,
		},
	}
	fakeClient := newFakeKubeClient(importedImage)
	step.KubeWatchClient = fakeClient

	ctx := context.TODO()
	objKey := client.ObjectKey{
		Name:      testCLItemName,
		Namespace: testNamespace,
	}
	testWriter := new(bytes.Buffer)
	state := newBasicTestState(testWriter)

	// 1. Test when 'clean_imported_image' config is not set.
	state.Put(supervisor.StateKeyImageImportRequestCreated, true)
	step.ImportImageConfig.ImportRequestName = testImportReqName

	step.Cleanup(state)

	expectedOutput := []string{"Skipping clean up of the ContentLibraryItemImportRequest object as specified in config."}
	checkOutputLines(t, testWriter, expectedOutput)

	if err := fakeClient.Get(ctx, objKey, &imgregv1.ContentLibraryItem{}); err != nil {
		t.Fatal("The ContentLibraryItem object should still exist")
	}

	// 2. Test when 'clean_imported_image' is set as true but imported item name is not set.
	step.ImportImageConfig.CleanImportedImage = true
	step.ImportItemResourceName = ""
	step.Cleanup(state)

	if err := fakeClient.Get(ctx, objKey, &imgregv1.ContentLibraryItem{}); err != nil {
		t.Fatal("The ContentLibraryItem object should still exist")
	}

	// 3. Test when 'clean_imported_image' and the imported item name is set.
	step.ImportItemResourceName = testCLItemName
	step.Cleanup(state)

	// Check if the imported image object is deleted from the cluster.
	if err := fakeClient.Get(ctx, objKey, &imgregv1.ContentLibraryItem{}); !errors.IsNotFound(err) {
		t.Fatal("Expected the ContentLibraryItem object to be deleted")
	}

	// Check the output lines from the step runs.
	expectedOutput = []string{
		fmt.Sprintf("Deleting the imported ContentLibraryItem object %s in namespace %s.", testCLItemName, testNamespace),
		fmt.Sprintf("Successfully deleted the ContentLibraryItem object %s in namespace %s.", testCLItemName, testNamespace),
	}
	checkOutputLines(t, testWriter, expectedOutput)
}
