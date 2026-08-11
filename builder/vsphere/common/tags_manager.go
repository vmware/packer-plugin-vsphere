// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/types"
)

// TagManager handles all tag operations.
type TagManager interface {
	// ValidateConfig validates the tag configuration.
	ValidateConfig() error

	// ResolveTagIDs resolves all tags to their IDs.
	ResolveTagIDs(ctx context.Context) ([]string, error)

	// ApplyTags attaches tags to a virtual machine.
	ApplyTags(ctx context.Context, vmRef types.ManagedObjectReference) error
}

// tagManager implements TagManager.
type tagManager struct {
	restClient    *rest.Client
	config        *TagsConfig
	ctx           interpolate.Context
	tagCache      map[string]*Tag         // Cache for tag lookups
	categoryCache map[string]*TagCategory // Cache for category lookups
	retryConfig   *RetryConfig            // Retry configuration for tag operations
}

// Tag represents a vSphere tag.
type Tag struct {
	ID          string
	Name        string
	Description string
	CategoryID  string
}

// TagCategory represents a vSphere tag category.
type TagCategory struct {
	ID              string
	Name            string
	Description     string
	Cardinality     string
	AssociableTypes []string
}

// NewTagManager creates a new tag manager.
func NewTagManager(restClient *rest.Client, config *TagsConfig, ctx interpolate.Context) TagManager {
	return &tagManager{
		restClient:    restClient,
		config:        config,
		ctx:           ctx,
		tagCache:      make(map[string]*Tag),
		categoryCache: make(map[string]*TagCategory),
		retryConfig:   DefaultRetryConfig(),
	}
}

// ValidateConfig validates the tag configuration.
func (tm *tagManager) ValidateConfig() error {
	if tm.config == nil {
		return nil // No tags configured, nothing to validate
	}

	// Track seen tags for deduplication
	seenIDs := make(map[string]bool)
	seenCategoryName := make(map[string]bool)

	// Validate tags list (tag IDs)
	for i, tagID := range tm.config.Tags {
		if tagID == "" {
			return NewValidationError(
				fmt.Sprintf("tags[%d]", i),
				"",
				"tag ID is empty",
				"Provide a valid vSphere tag URN (e.g., urn:vmomi:InventoryServiceTag:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx:GLOBAL)",
			)
		}

		// Validate URN format: urn:vmomi:InventoryServiceTag:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx:GLOBAL
		if !isValidTagURN(tagID) {
			return NewValidationError(
				fmt.Sprintf("tags[%d]", i),
				tagID,
				"invalid tag ID URN format",
				"Tag ID must be in the format: urn:vmomi:InventoryServiceTag:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx:GLOBAL",
			)
		}

		// Check for duplicates
		if seenIDs[tagID] {
			return NewValidationError(
				"tags",
				tagID,
				"duplicate tag ID",
				"Remove duplicate tag IDs from the configuration",
			)
		}
		seenIDs[tagID] = true
	}

	// Validate tag blocks (category/name pairs)
	for i, tag := range tm.config.Tag {
		// Check mutual exclusivity: either ID or (category + name), not both
		hasID := tag.ID != ""
		hasCategoryName := tag.Category != "" || tag.Name != ""

		if hasID && hasCategoryName {
			return NewValidationError(
				fmt.Sprintf("tag[%d]", i),
				"",
				"cannot specify both 'id' and 'category/name'",
				"Use either 'id' for existing tags or 'category' and 'name' to create/reference tags",
			)
		}

		if hasID {
			// Validate ID format
			if !isValidTagURN(tag.ID) {
				return NewValidationError(
					fmt.Sprintf("tag[%d].id", i),
					tag.ID,
					"invalid tag ID URN format",
					"Tag ID must be in the format: urn:vmomi:InventoryServiceTag:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx:GLOBAL",
				)
			}

			// Check for duplicates
			if seenIDs[tag.ID] {
				return NewValidationError(
					"tag",
					tag.ID,
					"duplicate tag ID",
					"Remove duplicate tag IDs from the configuration",
				)
			}
			seenIDs[tag.ID] = true
		} else if hasCategoryName {
			// Both category and name must be specified
			if tag.Category == "" {
				return NewValidationError(
					fmt.Sprintf("tag[%d].category", i),
					"",
					"category is required when using category/name format",
					"Specify a category name (e.g., 'environment', 'os', 'team')",
				)
			}
			if tag.Name == "" {
				return NewValidationError(
					fmt.Sprintf("tag[%d].name", i),
					"",
					"name is required when using category/name format",
					"Specify a tag name within the category (e.g., 'production', 'ubuntu', 'platform')",
				)
			}

			// Check for duplicates
			key := tag.Category + ":" + tag.Name
			if seenCategoryName[key] {
				return NewValidationError(
					"tag",
					fmt.Sprintf("%s/%s", tag.Category, tag.Name),
					"duplicate tag category/name pair",
					"Remove duplicate category/name combinations from the configuration",
				)
			}
			seenCategoryName[key] = true
		} else {
			return NewValidationError(
				fmt.Sprintf("tag[%d]", i),
				"",
				"must specify either 'id' or both 'category' and 'name'",
				"Provide either an 'id' field or both 'category' and 'name' fields",
			)
		}
	}

	return nil
}

// isValidTagURN validates that a string is a valid vSphere tag URN.
// Format: urn:vmomi:InventoryServiceTag:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx:GLOBAL
func isValidTagURN(urn string) bool {
	// Basic validation: must start with urn:vmomi:InventoryServiceTag:
	// and contain a UUID-like pattern
	if !strings.HasPrefix(urn, "urn:vmomi:InventoryServiceTag:") {
		return false
	}

	// Extract the UUID part (after the prefix and before :GLOBAL)
	parts := strings.Split(urn, ":")
	if len(parts) != 5 {
		return false
	}

	// Validate UUID format (basic check for 8-4-4-4-12 hex pattern)
	uuid := parts[3]
	uuidPattern := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	if !uuidPattern.MatchString(uuid) {
		return false
	}

	// Last part should be GLOBAL
	if parts[4] != "GLOBAL" {
		return false
	}

	return true
}

// ResolveTagIDs resolves all tags to their IDs.
func (tm *tagManager) ResolveTagIDs(ctx context.Context) ([]string, error) {
	if tm.config == nil {
		return nil, nil
	}

	var resolvedIDs []string
	seenIDs := make(map[string]bool)

	// Create tags manager from REST client
	tagsManager := tags.NewManager(tm.restClient)

	// Process tags list (direct tag IDs)
	for _, tagID := range tm.config.Tags {
		// Interpolate template variables
		interpolated, err := interpolate.Render(tagID, &tm.ctx)
		if err != nil {
			return nil, wrapTagOperationError("interpolate tag ID", err, map[string]string{
				"tag_id": tagID,
			})
		}

		// Verify tag exists
		tag, err := tm.getTagByID(ctx, tagsManager, interpolated)
		if err != nil {
			return nil, wrapTagOperationError("lookup tag by ID", err, map[string]string{
				"identifier":    interpolated,
				"resource_type": "tag",
				"suggestion":    "Please verify the tag ID is correct or use category/name format to create the tag automatically",
			})
		}

		if !seenIDs[tag.ID] {
			resolvedIDs = append(resolvedIDs, tag.ID)
			seenIDs[tag.ID] = true
		}
	}

	// Process tag blocks (category/name pairs or IDs)
	for i, tagConfig := range tm.config.Tag {
		if tagConfig.ID != "" {
			// Handle ID in tag block
			interpolated, err := interpolate.Render(tagConfig.ID, &tm.ctx)
			if err != nil {
				return nil, wrapTagOperationError("interpolate tag ID", err, map[string]string{
					"tag_id": tagConfig.ID,
					"block":  fmt.Sprintf("%d", i),
				})
			}

			tag, err := tm.getTagByID(ctx, tagsManager, interpolated)
			if err != nil {
				return nil, wrapTagOperationError("lookup tag by ID", err, map[string]string{
					"identifier":    interpolated,
					"resource_type": "tag",
					"block":         fmt.Sprintf("%d", i),
					"suggestion":    "Please verify the tag ID is correct or use category/name format to create the tag automatically",
				})
			}

			if !seenIDs[tag.ID] {
				resolvedIDs = append(resolvedIDs, tag.ID)
				seenIDs[tag.ID] = true
			}
		} else {
			// Handle category/name pair
			interpolatedCategory, err := interpolate.Render(tagConfig.Category, &tm.ctx)
			if err != nil {
				return nil, wrapTagOperationError("interpolate category", err, map[string]string{
					"category": tagConfig.Category,
					"block":    fmt.Sprintf("%d", i),
				})
			}

			interpolatedName, err := interpolate.Render(tagConfig.Name, &tm.ctx)
			if err != nil {
				return nil, wrapTagOperationError("interpolate tag name", err, map[string]string{
					"tag_name": tagConfig.Name,
					"category": interpolatedCategory,
					"block":    fmt.Sprintf("%d", i),
				})
			}

			// Resolve or create tag
			tag, err := tm.resolveOrCreateTag(ctx, tagsManager, interpolatedCategory, interpolatedName)
			if err != nil {
				return nil, wrapTagOperationError("resolve or create tag", err, map[string]string{
					"category": interpolatedCategory,
					"tag_name": interpolatedName,
					"block":    fmt.Sprintf("%d", i),
				})
			}

			if !seenIDs[tag.ID] {
				resolvedIDs = append(resolvedIDs, tag.ID)
				seenIDs[tag.ID] = true
			}
		}
	}

	return resolvedIDs, nil
}

// getTagByID retrieves a tag by its ID, using cache if available.
func (tm *tagManager) getTagByID(ctx context.Context, tagsManager *tags.Manager, tagID string) (*Tag, error) {
	// Check cache first
	if cached, ok := tm.tagCache[tagID]; ok {
		return cached, nil
	}

	var tag *Tag

	// Lookup tag via API with retry
	err := withRetry(ctx, tm.retryConfig, func() error {
		tagInfo, err := tagsManager.GetTag(ctx, tagID)
		if err != nil {
			return err
		}

		tag = &Tag{
			ID:          tagInfo.ID,
			Name:        tagInfo.Name,
			Description: tagInfo.Description,
			CategoryID:  tagInfo.CategoryID,
		}

		return nil
	})

	if err != nil {
		return nil, wrapTagOperationError("get tag by ID", err, map[string]string{
			"identifier":    tagID,
			"resource_type": "tag",
		})
	}

	// Cache the result
	tm.tagCache[tagID] = tag

	return tag, nil
}

// getCategoryByName retrieves a category by name, using cache if available.
func (tm *tagManager) getCategoryByName(ctx context.Context, tagsManager *tags.Manager, categoryName string) (*TagCategory, error) {
	// Check cache first
	if cached, ok := tm.categoryCache[categoryName]; ok {
		return cached, nil
	}

	var category *TagCategory

	// List all categories and find by name with retry
	err := withRetry(ctx, tm.retryConfig, func() error {
		categoryIDs, err := tagsManager.ListCategories(ctx)
		if err != nil {
			return err
		}

		for _, categoryID := range categoryIDs {
			categoryInfo, err := tagsManager.GetCategory(ctx, categoryID)
			if err != nil {
				continue
			}

			if categoryInfo.Name == categoryName {
				category = &TagCategory{
					ID:              categoryInfo.ID,
					Name:            categoryInfo.Name,
					Description:     categoryInfo.Description,
					Cardinality:     categoryInfo.Cardinality,
					AssociableTypes: categoryInfo.AssociableTypes,
				}

				return nil
			}
		}

		return NewNotFoundError(
			"category",
			categoryName,
			"Please create the category in vSphere first and ensure it is associable with VirtualMachine",
			nil,
		)
	})

	if err != nil {
		return nil, wrapTagOperationError("get category by name", err, map[string]string{
			"identifier":    categoryName,
			"resource_type": "category",
		})
	}

	// Cache the result
	tm.categoryCache[categoryName] = category

	return category, nil
}

// getTagByCategoryAndName retrieves a tag by category and name.
func (tm *tagManager) getTagByCategoryAndName(ctx context.Context, tagsManager *tags.Manager, categoryID, tagName string) (*Tag, error) {
	var tag *Tag

	// List tags in category with retry
	err := withRetry(ctx, tm.retryConfig, func() error {
		tagIDs, err := tagsManager.ListTagsForCategory(ctx, categoryID)
		if err != nil {
			return err
		}

		for _, tagID := range tagIDs {
			tagInfo, err := tagsManager.GetTag(ctx, tagID)
			if err != nil {
				continue
			}

			if tagInfo.Name == tagName {
				tag = &Tag{
					ID:          tagInfo.ID,
					Name:        tagInfo.Name,
					Description: tagInfo.Description,
					CategoryID:  tagInfo.CategoryID,
				}

				return nil
			}
		}

		// Tag not found, but not an error - don't retry
		return nil
	})

	if err != nil {
		return nil, wrapTagOperationError("list tags for category", err, map[string]string{
			"category_id": categoryID,
			"tag_name":    tagName,
		})
	}

	// Cache the result if found
	if tag != nil {
		tm.tagCache[tag.ID] = tag
	}

	return tag, nil
}

// resolveOrCreateTag resolves a tag by category/name, creating it if it doesn't exist.
func (tm *tagManager) resolveOrCreateTag(ctx context.Context, tagsManager *tags.Manager, categoryName, tagName string) (*Tag, error) {
	// Get category
	category, err := tm.getCategoryByName(ctx, tagsManager, categoryName)
	if err != nil {
		return nil, err
	}

	// Validate category is associable with VirtualMachine
	if !isAssociableWithVirtualMachine(category.AssociableTypes) {
		return nil, NewCategoryNotAssociableError(categoryName, category.AssociableTypes)
	}

	// Try to find existing tag
	tag, err := tm.getTagByCategoryAndName(ctx, tagsManager, category.ID, tagName)
	if err != nil {
		return nil, err
	}

	if tag != nil {
		return tag, nil
	}

	// Tag doesn't exist, create it with retry
	var createdTag *Tag
	err = withRetry(ctx, tm.retryConfig, func() error {
		tagID, err := tagsManager.CreateTag(ctx, &tags.Tag{
			Name:       tagName,
			CategoryID: category.ID,
		})
		if err != nil {
			return err
		}

		// Fetch the created tag
		tagInfo, err := tagsManager.GetTag(ctx, tagID)
		if err != nil {
			return err
		}

		createdTag = &Tag{
			ID:          tagInfo.ID,
			Name:        tagInfo.Name,
			Description: tagInfo.Description,
			CategoryID:  tagInfo.CategoryID,
		}

		return nil
	})

	if err != nil {
		return nil, wrapTagOperationError("create tag", err, map[string]string{
			"category": categoryName,
			"tag_name": tagName,
		})
	}

	// Cache the result
	tm.tagCache[createdTag.ID] = createdTag

	return createdTag, nil
}

// isAssociableWithVirtualMachine checks if a category can be associated with VirtualMachine objects.
func isAssociableWithVirtualMachine(associableTypes []string) bool {
	return slices.Contains(associableTypes, "VirtualMachine")
}

// ApplyTags attaches tags to a virtual machine.
func (tm *tagManager) ApplyTags(ctx context.Context, vmRef types.ManagedObjectReference) error {
	// Resolve all tag IDs
	tagIDs, err := tm.ResolveTagIDs(ctx)
	if err != nil {
		return err
	}

	if len(tagIDs) == 0 {
		return nil // No tags to apply
	}

	// Create tags manager
	tagsManager := tags.NewManager(tm.restClient)

	// Get currently attached tags to implement idempotency with retry
	var attachedSet map[string]bool
	err = withRetry(ctx, tm.retryConfig, func() error {
		attachedTags, err := tagsManager.ListAttachedTags(ctx, vmRef)
		if err != nil {
			return err
		}

		// Create a set of already attached tag IDs
		attachedSet = make(map[string]bool)
		for _, tagID := range attachedTags {
			attachedSet[tagID] = true
		}

		return nil
	})

	if err != nil {
		return wrapTagOperationError("list attached tags", err, map[string]string{
			"resource": vmRef.String(),
		})
	}

	// Attach each tag that isn't already attached
	for _, tagID := range tagIDs {
		if attachedSet[tagID] {
			// Tag already attached, skip (idempotent)
			continue
		}

		// Attach tag with retry
		err := withRetry(ctx, tm.retryConfig, func() error {
			return tagsManager.AttachTag(ctx, tagID, vmRef)
		})

		if err != nil {
			return wrapTagOperationError("attach tag", err, map[string]string{
				"tag_id":   tagID,
				"resource": vmRef.String(),
			})
		}
	}

	return nil
}
