// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package clone

import (
	"slices"
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

// TestSysprepFieldsMutuallyExclusive validates an error message is thrown
// when both windows_sysprep_text and windows_sysprep_file are included in the config
func TestSysprepFieldsMutuallyExclusive(t *testing.T) {
	// Customize config with windows sysprep file and windows sysprep text set.
	config := &CustomizeConfig{
		WindowsSysPrepFile: "path-to-file",
		WindowsSysPrepText: "text",
		NetworkInterfaces: []NetworkInterface{
			{},
		},
	}

	// Expected error message
	expectedError := errCustomizeOptionMutualExclusive
	_, errors := config.Prepare()

	// Make sure we only received on error
	expectedErrorLength := 1
	if len(errors) != expectedErrorLength {
		t.Fatalf("unexpected result: expected '%d', but returned: '%d'", expectedErrorLength, len(errors))
	}

	// Validate the error messages are what we expect.
	if errors[0] != expectedError {
		t.Fatalf("unexpected error: expected '%s', but returned: '%s'", expectedError, errors[0])
	}
}

// TestWindowsSysprepFilePrintsWarning tests a warning about
// the field being deprecated is returned when windows_sysprep_file field is set.
func TestWindowsSysprepFilePrintsWarning(t *testing.T) {
	// Customize config with windows sysprep file and windows sysprep text set.
	config := &CustomizeConfig{
		WindowsSysPrepFile: "path-to-file",
		NetworkInterfaces: []NetworkInterface{
			{},
		},
	}

	// Expected warning message
	expectedWarning := windowsSysprepFileDeprecatedMessage
	warnings, errors := config.Prepare()

	// Fail if there were errors
	if len(errors) > 0 {
		t.Fatalf("unexpected error: %s", errors)
	}

	// Search warnings array for the warning message
	found := slices.Contains(warnings, expectedWarning)

	// If we didn't find the expect warning message fail.
	if found == false {
		t.Fatalf("unexpected result: expected '%s' to be in warnings", expectedWarning)
	}

}

// TestWindowsSysprepTextSetsContent validates that when WindowSyrepText is set
// that it sets the value for the vSphere customization spec.
func TestWindowsSysprepTextSetsContent(t *testing.T) {
	// Expected text
	text := "xml customization spec"
	config := &CustomizeConfig{
		WindowsSysPrepText: text,
		NetworkInterfaces: []NetworkInterface{
			{},
		},
	}

	// Create a step customize object with the given config
	stepCustomize := &StepCustomize{Config: config}

	// Get the identity settings for the customize step
	baseCustomizationSettings, err := stepCustomize.identitySettings()
	if err != nil {
		t.Fatalf("unexpected error: '%v'", err)
	}

	// Cast the result to the vsphere govmomi type
	sysprepText, ok := baseCustomizationSettings.(*types.CustomizationSysprepText)
	if !ok {
		t.Fatalf("unexpected result: expected '%s', but returned %s", text, sysprepText)
	}

	// Make sure the sysprep text value is equal to what was passed in via the config.
	if sysprepText.Value != text {
		t.Fatalf("unexpected result: expected '%s', but returned '%s'", text, sysprepText.Value)
	}
}
