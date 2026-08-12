// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestTagError(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		context   map[string]string
		err       error
		want      string
	}{
		{
			name:      "basic error",
			operation: "create tag",
			err:       errors.New("underlying error"),
			want:      "failed to create tag: underlying error",
		},
		{
			name:      "error with context",
			operation: "attach tag",
			context: map[string]string{
				"tag_id":   "urn:vmomi:InventoryServiceTag:123:GLOBAL",
				"resource": "vm-123",
			},
			err:  errors.New("permission denied"),
			want: "failed to attach tag",
		},
		{
			name:      "error without underlying error",
			operation: "validate config",
			context: map[string]string{
				"field": "tags",
			},
			want: "failed to validate config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTagError(tt.operation, tt.err, tt.context)
			got := err.Error()

			if !strings.Contains(got, tt.want) {
				t.Errorf("TagError.Error() = %q, want to contain %q", got, tt.want)
			}

			// Test Unwrap
			if tt.err != nil && !errors.Is(err, tt.err) {
				t.Errorf("TagError.Unwrap() should return underlying error")
			}
		})
	}
}

func TestPermissionError(t *testing.T) {
	tests := []struct {
		name              string
		operation         string
		requiredPrivilege string
		resource          string
		err               error
		wantContains      []string
	}{
		{
			name:              "create tag permission error",
			operation:         "create tag",
			requiredPrivilege: "com.vmware.cis.tagging.Tag.Create",
			resource:          "category-123",
			err:               errors.New("unauthorized"),
			wantContains: []string{
				"permission denied",
				"create tag",
				"com.vmware.cis.tagging.Tag.Create",
				"category-123",
				"vSphere privileges",
			},
		},
		{
			name:              "attach tag permission error",
			operation:         "attach tag",
			requiredPrivilege: "com.vmware.cis.tagging.TagAssociation.Attach",
			wantContains: []string{
				"permission denied",
				"attach tag",
				"com.vmware.cis.tagging.TagAssociation.Attach",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewPermissionError(tt.operation, tt.requiredPrivilege, tt.resource, tt.err)
			got := err.Error()

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("PermissionError.Error() = %q, want to contain %q", got, want)
				}
			}

			// Test Unwrap
			if tt.err != nil && !errors.Is(err, tt.err) {
				t.Errorf("PermissionError.Unwrap() should return underlying error")
			}
		})
	}
}

func TestNotFoundError(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		identifier   string
		suggestion   string
		err          error
		wantContains []string
	}{
		{
			name:         "category not found",
			resourceType: "category",
			identifier:   "environment",
			suggestion:   "Please create the category in vSphere first",
			wantContains: []string{
				"category not found",
				"environment",
				"Please create the category in vSphere first",
			},
		},
		{
			name:         "tag not found",
			resourceType: "tag",
			identifier:   "urn:vmomi:InventoryServiceTag:123:GLOBAL",
			suggestion:   "Please verify the tag ID is correct",
			wantContains: []string{
				"tag not found",
				"urn:vmomi:InventoryServiceTag:123:GLOBAL",
				"Please verify the tag ID is correct",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewNotFoundError(tt.resourceType, tt.identifier, tt.suggestion, tt.err)
			got := err.Error()

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("NotFoundError.Error() = %q, want to contain %q", got, want)
				}
			}

			// Test Unwrap
			if tt.err != nil && !errors.Is(err, tt.err) {
				t.Errorf("NotFoundError.Unwrap() should return underlying error")
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	tests := []struct {
		name         string
		field        string
		value        string
		reason       string
		suggestion   string
		wantContains []string
	}{
		{
			name:       "empty tag ID",
			field:      "tags[0]",
			value:      "",
			reason:     "tag ID is empty",
			suggestion: "Provide a valid vSphere tag URN",
			wantContains: []string{
				"validation error",
				"tags[0]",
				"tag ID is empty",
				"Provide a valid vSphere tag URN",
			},
		},
		{
			name:       "invalid URN format",
			field:      "tags[1]",
			value:      "invalid-urn",
			reason:     "invalid tag ID URN format",
			suggestion: "Tag ID must be in the format: urn:vmomi:InventoryServiceTag:...",
			wantContains: []string{
				"validation error",
				"tags[1]",
				"invalid-urn",
				"invalid tag ID URN format",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError(tt.field, tt.value, tt.reason, tt.suggestion)
			got := err.Error()

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("ValidationError.Error() = %q, want to contain %q", got, want)
				}
			}
		})
	}
}

func TestCategoryNotAssociableError(t *testing.T) {
	tests := []struct {
		name            string
		categoryName    string
		associableTypes []string
		wantContains    []string
	}{
		{
			name:            "category with other types",
			categoryName:    "environment",
			associableTypes: []string{"Datacenter", "Cluster"},
			wantContains: []string{
				"environment",
				"not associable with VirtualMachine",
				"Datacenter, Cluster",
				"create a new category",
			},
		},
		{
			name:            "category with no types",
			categoryName:    "test",
			associableTypes: []string{},
			wantContains: []string{
				"test",
				"not associable with VirtualMachine",
				"no associable types configured",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewCategoryNotAssociableError(tt.categoryName, tt.associableTypes)
			got := err.Error()

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("CategoryNotAssociableError.Error() = %q, want to contain %q", got, want)
				}
			}
		})
	}
}

func TestIsPermissionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "permission denied",
			err:  errors.New("permission denied"),
			want: true,
		},
		{
			name: "unauthorized",
			err:  errors.New("unauthorized access"),
			want: true,
		},
		{
			name: "forbidden",
			err:  errors.New("forbidden resource"),
			want: true,
		},
		{
			name: "not a permission error",
			err:  errors.New("not found"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPermissionError(tt.err)
			if got != tt.want {
				t.Errorf("isPermissionError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "not found",
			err:  errors.New("resource not found"),
			want: true,
		},
		{
			name: "does not exist",
			err:  errors.New("category does not exist"),
			want: true,
		},
		{
			name: "cannot find",
			err:  errors.New("cannot find tag"),
			want: true,
		},
		{
			name: "not a not found error",
			err:  errors.New("permission denied"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNotFoundError(tt.err)
			if got != tt.want {
				t.Errorf("isNotFoundError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWrapTagOperationError(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		err       error
		context   map[string]string
		wantType  string
	}{
		{
			name:      "permission error",
			operation: "create tag",
			err:       errors.New("permission denied"),
			wantType:  "*common.PermissionError",
		},
		{
			name:      "not found error",
			operation: "get tag",
			err:       errors.New("tag not found"),
			context: map[string]string{
				"identifier":    "test-tag",
				"resource_type": "tag",
			},
			wantType: "*common.NotFoundError",
		},
		{
			name:      "generic error",
			operation: "attach tag",
			err:       errors.New("network timeout"),
			wantType:  "*common.TagError",
		},
		{
			name:      "nil error",
			operation: "test",
			err:       nil,
			wantType:  "<nil>",
		},
		{
			name:      "already wrapped error",
			operation: "test",
			err:       NewValidationError("field", "value", "reason", "suggestion"),
			wantType:  "*common.ValidationError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapTagOperationError(tt.operation, tt.err, tt.context)
			gotType := "<nil>"
			if got != nil {
				gotType = fmt.Sprintf("%T", got)
			}

			if gotType != tt.wantType {
				t.Errorf("wrapTagOperationError() type = %v, want %v", gotType, tt.wantType)
			}
		})
	}
}
