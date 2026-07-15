// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"errors"
	"fmt"
)

// ExactlyOne returns the sole element of items, or an error if the slice is
// empty or contains more than one element.
func ExactlyOne[T any](items []T, noneMsg, manyMsg string) (T, error) {
	var zero T
	switch len(items) {
	case 0:
		return zero, errors.New(noneMsg)
	case 1:
		return items[0], nil
	default:
		return zero, errors.New(manyMsg)
	}
}

// SelectMax returns the element with the greatest score. items must be
// non-empty. Equal scores keep the first-seen maximum.
func SelectMax[T any](items []T, score func(T) (int64, error)) (T, error) {
	var zero T
	if len(items) == 0 {
		return zero, errors.New("cannot select from an empty list")
	}

	best := items[0]
	bestScore, err := score(best)
	if err != nil {
		return zero, err
	}
	for _, item := range items[1:] {
		s, err := score(item)
		if err != nil {
			return zero, err
		}
		if s > bestScore {
			best = item
			bestScore = s
		}
	}
	return best, nil
}

// ResolveOne returns the only matching item.
// When multiple items match and selectAlt is true, selectFn picks one.
// When selectAlt is false, multiple matches return manyMsg.
func ResolveOne[T any](items []T, selectAlt bool, selectFn func([]T) (T, error), noneMsg, manyMsg string) (T, error) {
	var zero T
	switch len(items) {
	case 0:
		return zero, errors.New(noneMsg)
	case 1:
		return items[0], nil
	default:
		if !selectAlt {
			return zero, errors.New(manyMsg)
		}
		if selectFn == nil {
			return zero, fmt.Errorf("multiple matches require a selection function: %s", manyMsg)
		}
		return selectFn(items)
	}
}
