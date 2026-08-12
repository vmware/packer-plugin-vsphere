// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package acceptance

import (
	"fmt"
	"testing"

	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
	"github.com/zclconf/go-cty/cty"
)

// AssertTagsShape verifies that the datasource `tags` output is null or a list
// of objects with non-empty name and category strings.
func AssertTagsShape(t *testing.T, output cty.Value) {
	t.Helper()
	tagsAttr := output.GetAttr("tags")
	if tagsAttr.IsNull() {
		return
	}
	for i, tagVal := range tagsAttr.AsValueSlice() {
		if tagVal.IsNull() {
			t.Fatalf("tags[%d] is null", i)
		}
		name := tagVal.GetAttr("name")
		category := tagVal.GetAttr("category")
		if name.IsNull() || name.AsString() == "" {
			t.Fatalf("tags[%d].name is empty", i)
		}
		if category.IsNull() || category.AsString() == "" {
			t.Fatalf("tags[%d].category is empty", i)
		}
	}
}

// AssertContainsTag fails unless output.tags includes the given category/name
// pair.
func AssertContainsTag(t *testing.T, output cty.Value, category, name string) {
	t.Helper()
	tagsAttr := output.GetAttr("tags")
	if tagsAttr.IsNull() {
		t.Fatalf("expected tag %q/%q, but tags is null", category, name)
	}
	for _, tagVal := range tagsAttr.AsValueSlice() {
		if tagVal.GetAttr("name").AsString() == name && tagVal.GetAttr("category").AsString() == category {
			return
		}
	}
	t.Fatalf("expected tag category=%q name=%q in tags output", category, name)
}

// AttachTagsTemporarily ensures the category and tags exist, attaches any that
// are not already on ref, and registers t.Cleanup to detach only the tags this
// helper attached.
func AttachTagsTemporarily(t *testing.T, ref types.ManagedObjectReference, categoryName string, tagNames ...string) {
	t.Helper()
	if len(tagNames) == 0 {
		return
	}

	d, err := TestConn()
	if err != nil {
		t.Fatalf("connect for tagging: %v", err)
	}
	vd, ok := d.(*driver.VCenterDriver)
	if !ok {
		t.Fatalf("unexpected driver type %T", d)
	}
	if err := vd.RestClient.Login(vd.Ctx); err != nil {
		t.Fatalf("REST login for tagging: %v", err)
	}

	tm := tags.NewManager(vd.GetRestClient())
	catID, err := ensureTagCategory(vd, tm, categoryName)
	if err != nil {
		t.Fatalf("ensure tag category %q: %v", categoryName, err)
	}

	attachedIDs, err := tm.ListAttachedTags(vd.Ctx, ref)
	if err != nil {
		t.Fatalf("list attached tags: %v", err)
	}
	attached := make(map[string]bool, len(attachedIDs))
	for _, id := range attachedIDs {
		attached[id] = true
	}

	var attachedByUs []string
	for _, tagName := range tagNames {
		tagID, err := ensureTag(vd, tm, catID, tagName)
		if err != nil {
			t.Fatalf("ensure tag %q/%q: %v", categoryName, tagName, err)
		}
		if attached[tagID] {
			continue
		}
		if err := tm.AttachTag(vd.Ctx, tagID, ref); err != nil {
			t.Fatalf("attach tag %q/%q: %v", categoryName, tagName, err)
		}
		attachedByUs = append(attachedByUs, tagID)
	}

	t.Cleanup(func() {
		d, err := TestConn()
		if err != nil {
			t.Errorf("cleanup connect for tagging: %v", err)
			return
		}
		vd, ok := d.(*driver.VCenterDriver)
		if !ok {
			t.Errorf("cleanup unexpected driver type %T", d)
			return
		}
		if err := vd.RestClient.Login(vd.Ctx); err != nil {
			t.Errorf("cleanup REST login for tagging: %v", err)
			return
		}
		tm := tags.NewManager(vd.GetRestClient())
		for _, tagID := range attachedByUs {
			if err := tm.DetachTag(vd.Ctx, tagID, ref); err != nil {
				t.Errorf("detach tag %s: %v", tagID, err)
			}
		}
	})
}

func ensureTagCategory(d *driver.VCenterDriver, tm *tags.Manager, categoryName string) (string, error) {
	categories, err := tm.GetCategories(d.Ctx)
	if err != nil {
		return "", fmt.Errorf("list categories: %w", err)
	}
	for i := range categories {
		cat := categories[i]
		if cat.Name != categoryName {
			continue
		}
		changed := false
		if cat.Cardinality != "MULTIPLE" {
			cat.Cardinality = "MULTIPLE"
			changed = true
		}
		// Empty associable types means the category applies to all object types,
		// which covers both VirtualMachine and content library items.
		if len(cat.AssociableTypes) > 0 {
			cat.AssociableTypes = nil
			changed = true
		}
		if changed {
			if err := tm.UpdateCategory(d.Ctx, &cat); err != nil {
				return "", fmt.Errorf("update category %q: %w", categoryName, err)
			}
		}
		return cat.ID, nil
	}

	id, err := tm.CreateCategory(d.Ctx, &tags.Category{
		Name:        categoryName,
		Cardinality: "MULTIPLE",
	})
	if err != nil {
		return "", fmt.Errorf("create category %q: %w", categoryName, err)
	}
	return id, nil
}

func ensureTag(d *driver.VCenterDriver, tm *tags.Manager, categoryID, tagName string) (string, error) {
	existing, err := tm.GetTagsForCategory(d.Ctx, categoryID)
	if err != nil {
		return "", fmt.Errorf("list tags for category: %w", err)
	}
	for _, tag := range existing {
		if tag.Name == tagName {
			return tag.ID, nil
		}
	}
	id, err := tm.CreateTag(d.Ctx, &tags.Tag{Name: tagName, CategoryID: categoryID})
	if err != nil {
		return "", fmt.Errorf("create tag %q: %w", tagName, err)
	}
	return id, nil
}
