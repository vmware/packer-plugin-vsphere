// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package supervisor_test

import (
	"testing"

	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/supervisor"
)

func TestConfig_Minimal(t *testing.T) {
	// Using a minimal config requires that a valid kubeconfig is loaded automatically.
	validPath := getTestKubeconfigFile(t, "").Name()
	t.Setenv("KUBECONFIG", validPath)

	c := new(supervisor.Config)
	minConfigs := getMinimalConfig()
	warns, err := c.Prepare(minConfigs)
	if len(warns) != 0 {
		t.Errorf("unexpected warning: %#v", warns)
	}
	if err != nil {
		t.Errorf("unexpected errors: %s", err)
	}
}

func TestConfig_Required(t *testing.T) {
	c := new(supervisor.Config)
	minConfigs := getMinimalConfig()
	for key, val := range minConfigs {
		minConfigs[key] = ""
		_, err := c.Prepare(minConfigs)
		if err == nil {
			t.Errorf("unexpected error: '%s'", err)
		}
		minConfigs[key] = val
	}
}

func TestConfig_Complete(t *testing.T) {
	c := new(supervisor.Config)
	allConfigs := getCompleteConfig(t)
	warns, err := c.Prepare(allConfigs)
	if len(warns) != 0 {
		t.Errorf("unexpected warning: '%#v'", warns)
	}
	if err != nil {
		t.Errorf("unexpected error: '%s", err)
	}
}

func TestConfig_Values(t *testing.T) {
	c := new(supervisor.Config)
	providedConfigs := getCompleteConfig(t)
	warns, err := c.Prepare(providedConfigs)
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: '%#v'", warns)
	}
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}

	if c.ImageName != providedConfigs["image_name"] {
		t.Errorf("unexpected result: expected '%s' for 'image_name', but returned '%s'",
			providedConfigs["image_name"], c.ImageName)
	}
	if c.ClassName != providedConfigs["class_name"] {
		t.Errorf("unexpected result: expected '%s' for 'class_name', but returned '%s'",
			providedConfigs["class_name"], c.ClassName)
	}
	if c.StorageClass != providedConfigs["storage_class"] {
		t.Errorf("unexpected result: expected '%s' for 'storage_class', but returned '%s'",
			providedConfigs["storage_class"], c.StorageClass)
	}
	if c.PublishLocationName != providedConfigs["publish_location_name"] {
		t.Errorf("unexpected result: expected '%s' for 'publish_location_name', but returned '%s'",
			providedConfigs["publish_location_name"], c.PublishLocationName)
	}
	if c.PublishImageName != providedConfigs["publish_image_name"] {
		t.Errorf("unexpected result: expected '%s' for 'publish_image_name', but returned '%s'",
			providedConfigs["publish_image_name"], c.PublishImageName)
	}
	if c.KubeconfigPath != providedConfigs["kubeconfig_path"] {
		t.Errorf("unexpected result: expected '%s' for 'kubeconfig_path', but returned '%s'",
			providedConfigs["kubeconfig_path"], c.KubeconfigPath)
	}
	if c.SupervisorNamespace != providedConfigs["supervisor_namespace"] {
		t.Errorf("unexpected result: expected '%s' for 'supervisor_namespace', but returned '%s'",
			providedConfigs["supervisor_namespace"], c.SupervisorNamespace)
	}
	if c.SourceName != providedConfigs["source_name"] {
		t.Errorf("unexpected result: expected '%s' for 'source_name', but returned '%s'",
			providedConfigs["source_name"], c.SourceName)
	}
	if c.WatchSourceTimeoutSec != providedConfigs["watch_source_timeout_sec"] {
		t.Errorf("unexpected result: expected '%d' for 'watch_publish_timeout_sec', but returned '%d'",
			providedConfigs["watch_source_timeout_sec"], c.WatchSourceTimeoutSec)
	}
	if c.WatchPublishTimeoutSec != providedConfigs["watch_publish_timeout_sec"] {
		t.Errorf("unexpected result: expected '%d' for 'watch_publish_timeout_sec', but returned '%d'",
			providedConfigs["watch_publish_timeout_sec"], c.WatchPublishTimeoutSec)
	}
	if c.KeepInputArtifact != providedConfigs["keep_input_artifact"] {
		t.Errorf("unexpected result: expected 'true' for 'keep_input_artifact', but returned 'false'")
	}
}

func getMinimalConfig() map[string]any {
	return map[string]any{
		"class_name":    "test-class",
		"storage_class": "test-storage",
	}
}

func getCompleteConfig(t *testing.T) map[string]any {
	// Use a valid kubeconfig file as we check the content in config.Prepare() function.
	validPath := getTestKubeconfigFile(t, "").Name()

	return map[string]any{
		"image_name":                "test-image",
		"class_name":                "test-class",
		"storage_class":             "test-storage",
		"supervisor_namespace":      "test-namespace",
		"source_name":               "test-source",
		"publish_location_name":     "test-publish-location",
		"publish_image_name":        "test-publish-image",
		"watch_source_timeout_sec":  60,
		"watch_publish_timeout_sec": 60,
		"keep_input_artifact":       true,
		"kubeconfig_path":           validPath,
	}
}
