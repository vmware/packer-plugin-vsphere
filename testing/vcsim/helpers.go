// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package vcsim

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vapi/tags"
)

// markSimulatedVmAsTemplate powers off the virtual machine and converts it to a template.
func markSimulatedVmAsTemplate(ctx context.Context, vm *object.VirtualMachine) error {
	task, err := vm.PowerOff(ctx)
	if err != nil {
		return fmt.Errorf("failed to issue powering off command to the machine: %w", err)
	}
	err = task.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed to power off the machine: %w", err)
	}
	err = vm.MarkAsTemplate(ctx)
	if err != nil {
		return fmt.Errorf("failed to mark virtual machine as a template: %w", err)
	}
	return nil
}

// ensureCategory ensures a tag category exists by name and returns its ID.
func ensureCategory(ctx context.Context, man *tags.Manager, catName string) (string, error) {
	categoryList, err := man.GetCategories(ctx)
	if err != nil {
		return "", fmt.Errorf("cannot return categories from cluster: %w", err)
	}
	for _, category := range categoryList {
		if category.Name == catName {
			return category.ID, nil
		}
	}
	newCategoryID, err := man.CreateCategory(ctx, &tags.Category{Name: catName, Cardinality: "MULTIPLE"})
	if err != nil {
		return "", fmt.Errorf("cannot create category: %w", err)
	}
	return newCategoryID, nil
}

// ensureTag ensures a tag exists within the specified category and returns its ID.
func ensureTag(ctx context.Context, man *tags.Manager, catID string, tagName string) (string, error) {
	tagsInCategory, err := man.GetTagsForCategory(ctx, catID)
	if err != nil {
		return "", fmt.Errorf("cannot return tags for category: %w", err)
	}
	for _, tag := range tagsInCategory {
		if tag.Name == tagName {
			return tag.ID, nil
		}
	}
	newTagID, err := man.CreateTag(ctx, &tags.Tag{Name: tagName, CategoryID: catID})
	if err != nil {
		return "", fmt.Errorf("cannot create tag: %w", err)
	}
	return newTagID, nil
}
