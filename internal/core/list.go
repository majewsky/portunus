/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import "slices"

// Object is a trait for types that can be stored in type List.
type Object[Self any] interface {
	// List of permitted types. This is required for type inference, as explained here:
	// <https://stackoverflow.com/a/73851453>
	User | Group

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
			return obj, true
		}
	}
	var empty T
	return empty, false
}

// InsertOrUpdate adds the object to the list. If an object with the same key
// already exists in the list, it will be replaced.
func (list *ObjectList[T]) InsertOrUpdate(newObject T) {
	key := newObject.Key()

	//try updating an existing object
	for idx, oldObject := range *list {
		if oldObject.Key() == key {
			(*list)[idx] = newObject
			return
		}
	}

	//if no existing object found, insert a new one
	*list = append(*list, newObject)
}

// Delete removes the object with the given key from the list.
// If no such object exists, false is returned.
func (list *ObjectList[T]) Delete(key string) bool {
	for idx, obj := range *list {
		if obj.Key() == key {
			*list = slices.Delete(*list, idx, idx+1)
			return true
		}
	}
	return false
}
