// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"errors"
	"fmt"
	"strings"
)

// TagError represents a structured error for tag operations.
type TagError struct {
	// Operation is the operation that failed (e.g., "create tag", "attach tag")
	Operation string
	// Context provides additional context about the error
	Context map[string]string
	// Err is the underlying error
	Err error
}

// Error implements the error interface.
func (e *TagError) Error() string {
	var parts []string

	// Add operation
	if e.Operation != "" {
		parts = append(parts, fmt.Sprintf("failed to %s", e.Operation))
	}

	// Add context
	if len(e.Context) > 0 {
		var contextParts []string
		for k, v := range e.Context {
			contextParts = append(contextParts, fmt.Sprintf("%s=%q", k, v))
		}
		parts = append(parts, fmt.Sprintf("(%s)", strings.Join(contextParts, ", ")))
	}

	// Add underlying error
	if e.Err != nil {
		parts = append(parts, e.Err.Error())
	}

	return strings.Join(parts, ": ")
}

// Unwrap returns the underlying error.
func (e *TagError) Unwrap() error {
	return e.Err
}

// NewTagError creates a new TagError.
func NewTagError(operation string, err error, context map[string]string) *TagError {
	return &TagError{
		Operation: operation,
		Context:   context,
		Err:       err,
	}
}

// PermissionError represents a permission-related error.
type PermissionError struct {
	// Operation is the operation that failed
	Operation string
	// RequiredPrivilege is the vSphere privilege required
	RequiredPrivilege string
	// Resource is the resource being accessed
	Resource string
	// Err is the underlying error
	Err error
}

// Error implements the error interface.
func (e *PermissionError) Error() string {
	msg := fmt.Sprintf("permission denied: failed to %s", e.Operation)

	if e.Resource != "" {
		msg += fmt.Sprintf(" on resource %q", e.Resource)
	}

	if e.RequiredPrivilege != "" {
		msg += fmt.Sprintf(". Required privilege: %s", e.RequiredPrivilege)
	}

	msg += ". Please ensure the user has the necessary vSphere privileges for tag operations."

	if e.Err != nil {
		msg += fmt.Sprintf(" (underlying error: %s)", e.Err.Error())
	}

	return msg
}

// Unwrap returns the underlying error.
func (e *PermissionError) Unwrap() error {
	return e.Err
}

// NewPermissionError creates a new PermissionError.
func NewPermissionError(operation, requiredPrivilege, resource string, err error) *PermissionError {
	return &PermissionError{
		Operation:         operation,
		RequiredPrivilege: requiredPrivilege,
		Resource:          resource,
		Err:               err,
	}
}

// NotFoundError represents a resource not found error.
type NotFoundError struct {
	// ResourceType is the type of resource (e.g., "tag", "category")
	ResourceType string
	// Identifier is the identifier used to look up the resource
	Identifier string
	// Suggestion provides guidance on how to resolve the issue
	Suggestion string
	// Err is the underlying error
	Err error
}

// Error implements the error interface.
func (e *NotFoundError) Error() string {
	msg := fmt.Sprintf("%s not found", e.ResourceType)

	if e.Identifier != "" {
		msg += fmt.Sprintf(": %q", e.Identifier)
	}

	if e.Suggestion != "" {
		msg += fmt.Sprintf(". %s", e.Suggestion)
	}

	if e.Err != nil {
		msg += fmt.Sprintf(" (underlying error: %s)", e.Err.Error())
	}

	return msg
}

// Unwrap returns the underlying error.
func (e *NotFoundError) Unwrap() error {
	return e.Err
}

// NewNotFoundError creates a new NotFoundError.
func NewNotFoundError(resourceType, identifier, suggestion string, err error) *NotFoundError {
	return &NotFoundError{
		ResourceType: resourceType,
		Identifier:   identifier,
		Suggestion:   suggestion,
		Err:          err,
	}
}

// ValidationError represents a configuration validation error.
type ValidationError struct {
	// Field is the configuration field that failed validation
	Field string
	// Value is the invalid value
	Value string
	// Reason explains why the validation failed
	Reason string
	// Suggestion provides guidance on how to fix the issue
	Suggestion string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	msg := "validation error"

	if e.Field != "" {
		msg += fmt.Sprintf(" for field %q", e.Field)
	}

	if e.Value != "" {
		msg += fmt.Sprintf(" with value %q", e.Value)
	}

	if e.Reason != "" {
		msg += fmt.Sprintf(": %s", e.Reason)
	}

	if e.Suggestion != "" {
		msg += fmt.Sprintf(". %s", e.Suggestion)
	}

	return msg
}

// NewValidationError creates a new ValidationError.
func NewValidationError(field, value, reason, suggestion string) *ValidationError {
	return &ValidationError{
		Field:      field,
		Value:      value,
		Reason:     reason,
		Suggestion: suggestion,
	}
}

// CategoryNotAssociableError represents an error when a category cannot be associated with VirtualMachine.
type CategoryNotAssociableError struct {
	// CategoryName is the name of the category
	CategoryName string
	// AssociableTypes are the types the category can be associated with
	AssociableTypes []string
}

// Error implements the error interface.
func (e *CategoryNotAssociableError) Error() string {
	msg := fmt.Sprintf("category %q is not associable with VirtualMachine", e.CategoryName)

	if len(e.AssociableTypes) > 0 {
		msg += fmt.Sprintf(". The category can only be associated with: %s", strings.Join(e.AssociableTypes, ", "))
	} else {
		msg += ". The category has no associable types configured"
	}

	msg += ". Please create a new category in vSphere that is associable with VirtualMachine, or modify the existing category's associable types."

	return msg
}

// NewCategoryNotAssociableError creates a new CategoryNotAssociableError.
func NewCategoryNotAssociableError(categoryName string, associableTypes []string) *CategoryNotAssociableError {
	return &CategoryNotAssociableError{
		CategoryName:    categoryName,
		AssociableTypes: associableTypes,
	}
}

// isPermissionError checks if an error is a permission-related error.
func isPermissionError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	permissionPatterns := []string{
		"permission denied",
		"unauthorized",
		"forbidden",
		"access denied",
		"insufficient privileges",
		"not authorized",
	}

	for _, pattern := range permissionPatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// isNotFoundError checks if an error is a not found error.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	notFoundPatterns := []string{
		"not found",
		"does not exist",
		"doesn't exist",
		"no such",
		"cannot find",
	}

	for _, pattern := range notFoundPatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// wrapTagOperationError wraps an error with appropriate context based on the error type.
func wrapTagOperationError(operation string, err error, context map[string]string) error {
	if err == nil {
		return nil
	}

	// Check if it's already a structured error
	var tagError *TagError
	var permissionError *PermissionError
	var notFoundError *NotFoundError
	var validationError *ValidationError
	var categoryNotAssociableError *CategoryNotAssociableError
	switch {
	case errors.As(err, &tagError), errors.As(err, &permissionError), errors.As(err, &notFoundError), errors.As(err, &validationError), errors.As(err, &categoryNotAssociableError):
		return err
	}

	// Wrap with appropriate error type based on error message
	if isPermissionError(err) {
		resource := ""
		if context != nil {
			if r, ok := context["resource"]; ok {
				resource = r
			}
		}

		// Determine required privilege based on operation
		var privilege string
		switch {
		case strings.Contains(operation, "create tag"):
			privilege = "com.vmware.cis.tagging.Tag.Create"
		case strings.Contains(operation, "attach tag"):
			privilege = "com.vmware.cis.tagging.TagAssociation.Attach"
		case strings.Contains(operation, "read") || strings.Contains(operation, "list") || strings.Contains(operation, "get"):
			privilege = "com.vmware.cis.tagging.Category.Read or com.vmware.cis.tagging.Tag.Read"
		}

		return NewPermissionError(operation, privilege, resource, err)
	}

	if isNotFoundError(err) {
		identifier := ""
		resourceType := "resource"
		suggestion := ""

		if context != nil {
			if id, ok := context["identifier"]; ok {
				identifier = id
			}
			if rt, ok := context["resource_type"]; ok {
				resourceType = rt
			}
			if s, ok := context["suggestion"]; ok {
				suggestion = s
			}
		}

		// Provide default suggestions based on resource type
		if suggestion == "" {
			switch resourceType {
			case "category":
				suggestion = "Please create the category in vSphere first and ensure it is associable with VirtualMachine"
			case "tag":
				suggestion = "Please verify the tag ID is correct or use category/name format to create the tag automatically"
			}
		}

		return NewNotFoundError(resourceType, identifier, suggestion, err)
	}

	// Default: wrap with TagError
	return NewTagError(operation, err, context)
}
