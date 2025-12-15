// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/acctest"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/packer-plugin-vsphere/testing/vsphere"
	"k8s.io/client-go/tools/clientcmd"
)

// defaultConfig initializes and returns a default configuration map for a vSphere supervisor builder.
func defaultConfig() map[string]interface{} {
	// Supervisor builder uses kubeconfig for authentication
	kubeconfigPath := os.Getenv(clientcmd.RecommendedConfigPathEnvVar)
	if kubeconfigPath == "" {
		kubeconfigPath = clientcmd.RecommendedHomeFile
	}

	config := map[string]interface{}{
		"kubeconfig_path": kubeconfigPath,
		"class_name":      "test-class",
		"storage_class":   "test-storage",
		"communicator":    "none",
	}

	return config
}

// TestAccSupervisorBuilderAcc_tags_list acceptance test validates tag application using tags list (tag IDs).
// NOTE: This test is currently skipped because the supervisor builder does not properly support tags yet.
// The StepApplyTags requires a vSphere driver and VM reference in the state bag, but the supervisor
// builder uses Kubernetes APIs and does not set up these state variables. This needs to be fixed
// before tag support can work with the supervisor builder.
func TestAccSupervisorBuilderAcc_tags_list(t *testing.T) {
	t.Skip("Supervisor builder tag support is not yet implemented - missing driver/vm state setup")

	config := defaultConfig()

	// Setup: Create test tag category and tags
	d, err := vsphere.TestConn()
	if err != nil {
		t.Fatalf("cannot connect: %v", err)
	}

	ctx := context.Background()
	restClient := d.GetRestClient()
	tagsManager := tags.NewManager(restClient)

	// Create test category
	categoryID, err := tagsManager.CreateCategory(ctx, &tags.Category{
		Name:            "test-category-supervisor-list",
		Cardinality:     "MULTIPLE",
		AssociableTypes: []string{"VirtualMachine"},
	})
	if err != nil {
		t.Fatalf("cannot create test category: %v", err)
	}

	// Create test tags
	tag1ID, err := tagsManager.CreateTag(ctx, &tags.Tag{
		Name:       "supervisor-tag-1",
		CategoryID: categoryID,
	})
	if err != nil {
		t.Fatalf("cannot create test tag 1: %v", err)
	}

	tag2ID, err := tagsManager.CreateTag(ctx, &tags.Tag{
		Name:       "supervisor-tag-2",
		CategoryID: categoryID,
	})
	if err != nil {
		t.Fatalf("cannot create test tag 2: %v", err)
	}

	// Configure tags list
	config["tags"] = []string{tag1ID, tag2ID}

	testCase := &acctest.PluginTestCase{
		Name:     "vsphere-supervisor_tags_list_test",
		Template: vsphere.RenderConfig("vsphere-supervisor", config),
		Teardown: func() error {
			// Note: Cleanup would need to be implemented for supervisor VMs
			return nil
		},
		Check: func(buildCommand *exec.Cmd, logfile string) error {
			if buildCommand.ProcessState != nil {
				if buildCommand.ProcessState.ExitCode() != 0 {
					return fmt.Errorf("bad exit code; logfile: %s", logfile)
				}
			}
			// Note: VM name would need to be retrieved from supervisor resources
			return checkTags("", []string{tag1ID, tag2ID})
		},
	}
	acctest.TestPlugin(t, testCase)
}

// TestAccSupervisorBuilderAcc_tags_blocks acceptance test validates tag application using tag blocks (category/name).
// NOTE: This test is currently skipped because the supervisor builder does not properly support tags yet.
// The StepApplyTags requires a vSphere driver and VM reference in the state bag, but the supervisor
// builder uses Kubernetes APIs and does not set up these state variables. This needs to be fixed
// before tag support can work with the supervisor builder.
func TestAccSupervisorBuilderAcc_tags_blocks(t *testing.T) {
	t.Skip("Supervisor builder tag support is not yet implemented - missing driver/vm state setup")

	config := defaultConfig()

	// Setup: Create test tag category (tags will be created by the builder)
	d, err := vsphere.TestConn()
	if err != nil {
		t.Fatalf("cannot connect: %v", err)
	}

	ctx := context.Background()
	restClient := d.GetRestClient()
	tagsManager := tags.NewManager(restClient)

	// Create test category
	_, err = tagsManager.CreateCategory(ctx, &tags.Category{
		Name:            "test-category-supervisor-blocks",
		Cardinality:     "MULTIPLE",
		AssociableTypes: []string{"VirtualMachine"},
	})
	if err != nil {
		t.Fatalf("cannot create test category: %v", err)
	}

	// Configure tag blocks
	config["tag"] = []map[string]interface{}{
		{
			"category": "test-category-supervisor-blocks",
			"name":     "supervisor-block-tag-1",
		},
		{
			"category": "test-category-supervisor-blocks",
			"name":     "supervisor-block-tag-2",
		},
	}

	testCase := &acctest.PluginTestCase{
		Name:     "vsphere-supervisor_tags_blocks_test",
		Template: vsphere.RenderConfig("vsphere-supervisor", config),
		Teardown: func() error {
			// Note: Cleanup would need to be implemented for supervisor VMs
			return nil
		},
		Check: func(buildCommand *exec.Cmd, logfile string) error {
			if buildCommand.ProcessState != nil {
				if buildCommand.ProcessState.ExitCode() != 0 {
					return fmt.Errorf("bad exit code; logfile: %s", logfile)
				}
			}
			// Note: VM name would need to be retrieved from supervisor resources
			return checkTagsByName("", "test-category-supervisor-blocks", []string{"supervisor-block-tag-1", "supervisor-block-tag-2"})
		},
	}
	acctest.TestPlugin(t, testCase)
}

// checkTags verifies that the specified tags are attached to a virtual machine.
func checkTags(vmName string, expectedTagIDs []string) error {
	d, err := vsphere.TestConn()
	if err != nil {
		return fmt.Errorf("cannot connect: %v", err)
	}

	vm, err := d.FindVM(vmName)
	if err != nil {
		return fmt.Errorf("cannot find VM: %v", err)
	}

	ctx := context.Background()
	restClient := d.GetRestClient()
	tagsManager := tags.NewManager(restClient)

	// Get attached tags
	attachedTags, err := tagsManager.ListAttachedTags(ctx, vm.Reference())
	if err != nil {
		return fmt.Errorf("cannot list attached tags: %v", err)
	}

	// Create a map of attached tag IDs for easy lookup
	attachedMap := make(map[string]bool)
	for _, tagID := range attachedTags {
		attachedMap[tagID] = true
	}

	// Verify all expected tags are attached
	for _, expectedTagID := range expectedTagIDs {
		if !attachedMap[expectedTagID] {
			return fmt.Errorf("expected tag '%s' not found in attached tags", expectedTagID)
		}
	}

	return nil
}

// checkTagsByName verifies that tags with the specified names in a category are attached to a virtual machine.
func checkTagsByName(vmName string, categoryName string, expectedTagNames []string) error {
	d, err := vsphere.TestConn()
	if err != nil {
		return fmt.Errorf("cannot connect: %v", err)
	}

	vm, err := d.FindVM(vmName)
	if err != nil {
		return fmt.Errorf("cannot find VM: %v", err)
	}

	ctx := context.Background()
	restClient := d.GetRestClient()
	tagsManager := tags.NewManager(restClient)

	// Get attached tags
	attachedTagIDs, err := tagsManager.ListAttachedTags(ctx, vm.Reference())
	if err != nil {
		return fmt.Errorf("cannot list attached tags: %v", err)
	}

	// Get tag details and filter by category
	attachedTagNames := make(map[string]bool)
	for _, tagID := range attachedTagIDs {
		tagInfo, err := tagsManager.GetTag(ctx, tagID)
		if err != nil {
			continue
		}

		// Get category info
		categoryInfo, err := tagsManager.GetCategory(ctx, tagInfo.CategoryID)
		if err != nil {
			continue
		}

		if categoryInfo.Name == categoryName {
			attachedTagNames[tagInfo.Name] = true
		}
	}

	// Verify all expected tags are attached
	for _, expectedTagName := range expectedTagNames {
		if !attachedTagNames[expectedTagName] {
			return fmt.Errorf("expected tag '%s' in category '%s' not found in attached tags", expectedTagName, categoryName)
		}
	}

	return nil
}
