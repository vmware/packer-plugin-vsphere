// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"context"
	"errors"
	"fmt"

	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

// TagMatcher checks whether inventory objects have all required tags.
// Login to the REST API happens once in NewTagMatcher.
type TagMatcher struct {
	ctx      context.Context
	manager  *tags.Manager
	required []Tag
}

// NewTagMatcher logs into the REST API once when required tags are non-empty.
func NewTagMatcher(d *driver.VCenterDriver, required []Tag) (*TagMatcher, error) {
	m := &TagMatcher{
		ctx:      d.Ctx,
		required: required,
	}
	if len(required) == 0 {
		return m, nil
	}

	if err := d.RestClient.Login(d.Ctx); err != nil {
		return nil, fmt.Errorf("failed to login to REST API: %w", err)
	}
	m.manager = tags.NewManager(d.RestClient.Client())
	return m, nil
}

// HasAll reports whether the object has all required tags attached.
func (m *TagMatcher) HasAll(ref types.ManagedObjectReference) (bool, error) {
	if len(m.required) == 0 {
		return true, nil
	}
	if m.manager == nil {
		return false, errors.New("tag matcher is not initialized")
	}

	attached, err := m.manager.GetAttachedTags(m.ctx, ref)
	if err != nil {
		return false, fmt.Errorf("failed to return tags for the object: %w", err)
	}

	matchedTagsCount := 0
	for _, configTag := range m.required {
		configTagMatched := false
		for _, realTag := range attached {
			if configTag.Name != realTag.Name {
				continue
			}
			category, err := m.manager.GetCategory(m.ctx, realTag.CategoryID)
			if err != nil {
				return false, fmt.Errorf("failed to return tag category for tag: %w", err)
			}
			if configTag.Category == category.Name {
				configTagMatched = true
				break
			}
		}
		if configTagMatched {
			matchedTagsCount++
		} else {
			break
		}
	}
	return matchedTagsCount == len(m.required), nil
}
