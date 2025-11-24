/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"errors"
	"slices"
)

// Object is a trait for types that can be stored in type List.
type Object[Self any] interface {
	// Returns a field from this struct that uniquely identifies it within the List.
	Key() string
	// Returns a deep copy of this struct.
	Cloned() Self
}

// ObjectList adds convenience methods for working with lists of users and groups.
type ObjectList[T Object[T]] []T

// Cloned returns a deep copy of this list.
func (list ObjectList[T]) Cloned() ObjectList[T] {
	result := make(ObjectList[T], len(list))
	for idx, obj := range list {
		result[idx] = obj.Cloned()
	}
	return result
}

// Find returns a copy of the first object matching the given predicate.
// If no match is found, false is returned in the second return value
// (like for a two-valued map lookup).
func (list ObjectList[T]) Find(predicate func(T) bool) (T, bool) {
	for _, obj := range list {
		if predicate(obj) {
			return obj.Cloned(), true
		}
	}
	var empty T
	return empty, false
}

var errNoSuchObject = errors.New("no such object")

// Update replaces an existing object with the same key in the list.
// If no such object exists, an error is returned.
func (list *ObjectList[T]) Update(newObject T) error {
	key := newObject.Key()
	for idx, oldObject := range *list {
		if oldObject.Key() == key {
			(*list)[idx] = newObject.Cloned()
			return nil
		}
	}
	return errNoSuchObject
}

// Delete removes the object with the given key from the list.
// If no such object exists, an error is returned.
func (list *ObjectList[T]) Delete(key string) error {
	for idx, obj := range *list {
		if obj.Key() == key {
			*list = slices.Delete(*list, idx, idx+1)
			return nil
		}
	}
	return errNoSuchObject
}
