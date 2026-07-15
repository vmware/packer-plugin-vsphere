// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"strings"
	"testing"
)

func TestExactlyOne(t *testing.T) {
	t.Parallel()

	got, err := ExactlyOne([]string{"only"}, "none", "many")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "only" {
		t.Fatalf("expected only, got %q", got)
	}

	_, err = ExactlyOne([]string{}, "none", "many")
	if err == nil || err.Error() != "none" {
		t.Fatalf("expected none error, got %v", err)
	}

	_, err = ExactlyOne([]string{"a", "b"}, "none", "many")
	if err == nil || err.Error() != "many" {
		t.Fatalf("expected many error, got %v", err)
	}
}

func TestSelectMax(t *testing.T) {
	t.Parallel()

	type item struct {
		name  string
		value int64
	}
	items := []item{
		{name: "low", value: 1},
		{name: "high", value: 10},
		{name: "mid", value: 5},
	}

	got, err := SelectMax(items, func(i item) (int64, error) {
		return i.value, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.name != "high" {
		t.Fatalf("expected high, got %q", got.name)
	}

	_, err = SelectMax([]item{}, func(i item) (int64, error) {
		return i.value, nil
	})
	if err == nil {
		t.Fatal("expected error for empty list")
	}
}

func TestResolveOne(t *testing.T) {
	t.Parallel()

	got, err := ResolveOne([]int{7}, false, nil, "none", "many")
	if err != nil || got != 7 {
		t.Fatalf("expected 7, got %d, err %v", got, err)
	}

	_, err = ResolveOne([]int{}, false, nil, "none", "many")
	if err == nil || err.Error() != "none" {
		t.Fatalf("expected none, got %v", err)
	}

	_, err = ResolveOne([]int{1, 2}, false, nil, "none", "many")
	if err == nil || err.Error() != "many" {
		t.Fatalf("expected many, got %v", err)
	}

	got, err = ResolveOne([]int{1, 9, 3}, true, func(items []int) (int, error) {
		return SelectMax(items, func(v int) (int64, error) {
			return int64(v), nil
		})
	}, "none", "many")
	if err != nil || got != 9 {
		t.Fatalf("expected 9 from selectAlt, got %d, err %v", got, err)
	}
}

func TestValidateTags(t *testing.T) {
	t.Parallel()

	if err := ValidateTags([]Tag{{Name: "a", Category: "b"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateTags([]Tag{{Name: "", Category: "b"}}); err == nil {
		t.Fatal("expected error for empty name")
	}
	if err := ValidateTags([]Tag{{Name: "a", Category: ""}}); err == nil {
		t.Fatal("expected error for empty category")
	}

	err := ValidateTags([]Tag{
		{Name: "", Category: "b"},
		{Name: "a", Category: ""},
	})
	if err == nil {
		t.Fatal("expected errors for multiple invalid tags")
	}
	needle := "both name and category are required for tag"
	if n := strings.Count(err.Error(), needle); n < 2 {
		t.Fatalf("expected two validation messages, got %q", err)
	}
}
