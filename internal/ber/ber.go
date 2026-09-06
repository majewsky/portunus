// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

// Package ber implements just enough of [ASN.1] [BER] to cover [LDAP].
//
// [ASN.1]: https://www.itu.int/rec/T-REC-X.680/en
// [BER]: https://www.itu.int/rec/T-REC-X.690/en
// [LDAP]: https://datatracker.ietf.org/doc/html/rfc4511
package ber

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	. "go.xyrillian.de/gg/option"
)

// ChoiceOf is an interface for types that can be encoded in BER using the tag "CHOICE" in the universal class.
//
// To document which choices exist, and which tags are used to encode them, the type argument T must be a struct type containing one field for each choice:
//
//	type Message struct { // a SEQUENCE type
//		Operation ber.ChoiceOf[Operation]
//	}
//
//	type Operation struct { // declaration for a CHOICE type (this type is never actually instantiated)
//		FooOperation `ber:"application:1"`
//		BarOperation `ber:"application:2"`
//	}
//
//	type FooOperation struct { ... }
//	func (o FooOperation) Into() Operation { return Operation{FooOperation: o} }
//
//	type BarOperation struct { ... }
//	func (o BarOperation) Into() Operation { return Operation{BarOperation: o} }
//
// After unmarshaling a message, its Operation field will hold either an instance of FooOperation or BarOperation.
// This can be analyzed using type assertions when processing the message.
type ChoiceOf[T any] interface {
	Into() T
}

// detectChoiceType takes a type that might be a ChoiceOf[T], and returns T if it is.
func detectChoiceType(in reflect.Type) (out reflect.Type, ok bool) {
	for idx := range in.NumMethod() {
		m := in.Method(idx)
		if m.Name == "Into" && m.Type.NumIn() == 0 && m.Type.NumOut() == 1 {
			return m.Type.Out(0), true
		}
	}

	ok = false
	return
}

// castChoiceType converts a ChoiceOf[T] value into a T.
func castChoiceType(v reflect.Value) (out reflect.Value, ok bool) {
	mv := v.MethodByName("Into")
	if mv.Kind() == reflect.Func && mv.Type().NumIn() == 0 && mv.Type().NumOut() == 1 {
		outs := mv.Call(nil)
		return outs[0], true
	}

	ok = false
	return
}

// Enum is an interface for integer types that allow only a predefined set of enumerated values.
//
// If this interface is implemented on a type, [Unmarshal] will expect values of this type to be encoded with tag ENUMERATED instead of INTEGER; see documentation over there.
type Enum interface {
	IsEnum() bool
	Validatable
}

// SetOf wraps slice types that must be encoded with the tag SET OF instead of SEQUENCE OF.
// See documentation on [Unmarshal] for details.
type SetOf[T any] []T

type encodesWithSetOf interface {
	isSetOf()
}

func (SetOf[T]) isSetOf() {}

var _ encodesWithSetOf = SetOf[int]{}

// Validatable is an interface for types that support validation during unmarshaling.
//
// When [Unmarshal] unmarshals into a type that implements this interface, it will call the Validate() method and abort if it returns an error.
type Validatable interface {
	Validate() error
}

// structInfo contains structural information about a struct type that is relevant for marshaling.
type structInfo struct {
	Fields []fieldInfo
}

// fieldInfo appears in type [structInfo].
type fieldInfo struct {
	Index      []int
	Name       string
	Tag        tag
	TagChoices map[tag]reflect.Type // if the field is a ChoiceOf[T], the set of realization types with their respective tags
	IsOptional bool
}

// acceptsTag returns whether this field can be represented as an encoded value with the given tag.
func (fi fieldInfo) acceptsTag(t tag) bool {
	if len(fi.TagChoices) == 0 {
		return fi.Tag == t
	} else {
		_, ok := fi.TagChoices[t]
		return ok
	}
}

var (
	structInfoCache      = map[reflect.Type]structInfo{}
	structInfoCacheMutex sync.Mutex
)

// getStructInfo is a memoized version of buildStructInfo.
func getStructInfo(t reflect.Type) structInfo {
	si, ok := tryLoadStructInfo(t)
	if !ok {
		var err error
		si, err = buildStructInfo(t)
		if err != nil {
			panic(err.Error())
		}
		si = storeStructInfo(t, si)
	}
	return si
}

func tryLoadStructInfo(t reflect.Type) (structInfo, bool) {
	structInfoCacheMutex.Lock()
	defer structInfoCacheMutex.Unlock()
	si, ok := structInfoCache[t]
	return si, ok
}

func storeStructInfo(t reflect.Type, si structInfo) structInfo {
	structInfoCacheMutex.Lock()
	defer structInfoCacheMutex.Unlock()
	if actual, ok := structInfoCache[t]; ok {
		return actual
	} else {
		structInfoCache[t] = si
		return si
	}
}

// buildStructInfo analyzes a struct type to build a structInfo.
// This should not be called directly; use func getStructInfo instead.
func buildStructInfo(t reflect.Type) (si structInfo, err error) {
	if t.Kind() != reflect.Struct {
		zero := reflect.New(t).Elem()
		return si, fmt.Errorf("buildStructInfo called for non-struct type %T", zero.Interface())
	}

	for _, field := range reflect.VisibleFields(t) {
		// ignore unexported fields where field.Interface() does not work
		if !field.IsExported() {
			continue
		}
		// ignore embedded fields (we will consider the fields of the embedded type instead)
		if field.Anonymous {
			continue
		}

		fi := fieldInfo{
			Index: field.Index,
			Name:  field.Name,
		}
		if choiceType, ok := detectChoiceType(field.Type); ok {
			fi.TagChoices, err = buildTagChoices(choiceType)
			if err != nil {
				return si, err
			}
		}
		tagOrNone := nativeTagFor(field.Type) // may be None if there is no native tag (esp. for ChoiceOf[T] fields)
		for idx, value := range strings.Split(field.Tag.Get("ber"), ",") {
			if idx == 0 {
				if value == "" {
					// acceptable, used when no tag spec is needed
					continue
				}
				if numStr, ok := strings.CutPrefix(value, "context-specific:"); ok {
					if num, err := strconv.ParseUint(numStr, 10, 32); err == nil {
						tagOrNone = Some(tag{tagClassContextSpecific, uint(num)})
						continue
					}
				}
				if numStr, ok := strings.CutPrefix(value, "application:"); ok {
					if num, err := strconv.ParseUint(numStr, 10, 32); err == nil {
						tagOrNone = Some(tag{tagClassApplication, uint(num)})
						continue
					}
				}
			} else {
				if value == "optional" {
					fi.IsOptional = true
					continue
				}
			}
			return si, fmt.Errorf(
				"unrecognizable struct field tag in position %d on field %s.%s: %q",
				idx, t.Name(), fi.Name, value,
			)
		}

		if chosenTag, ok := tagOrNone.Unpack(); ok {
			fi.Tag = chosenTag
		} else if len(fi.TagChoices) == 0 {
			return si, fmt.Errorf(
				"type of struct field %s.%s has no native tag choice (must be specified explicitly in the struct field tag)",
				t.Name(), fi.Name,
			)
		}

		si.Fields = append(si.Fields, fi)
	}
	return si, nil
}

// buildTagChoices analyzes the underlying type T for a ber.ChoiceOf[T].
func buildTagChoices(t reflect.Type) (result map[tag]reflect.Type, _ error) {
	if t.Kind() != reflect.Struct {
		zero := reflect.New(t).Elem()
		return result, fmt.Errorf("buildTagChoices called for non-struct type %T", zero.Interface())
	}

	result = make(map[tag]reflect.Type)
	for idx := range t.NumField() {
		fi := t.Field(idx)
		if !fi.Anonymous {
			return result, fmt.Errorf("choice declaration type %s contains non-embedded field %s", t.Name(), fi.Name)
		}

		tagValue := fi.Tag.Get("ber")
		if tagValue == "" {
			return result, fmt.Errorf("choice declaration %s.%s does not have a `ber:\"...\"` tag", t.Name(), fi.Name)
		}

		tagOrNone := None[tag]()
		if numStr, ok := strings.CutPrefix(tagValue, "context-specific:"); ok {
			if num, err := strconv.ParseUint(numStr, 10, 32); err == nil {
				tagOrNone = Some(tag{tagClassContextSpecific, uint(num)})
			}
		} else if numStr, ok := strings.CutPrefix(tagValue, "application:"); ok {
			if num, err := strconv.ParseUint(numStr, 10, 32); err == nil {
				tagOrNone = Some(tag{tagClassApplication, uint(num)})
			}
		}
		if chosenTag, ok := tagOrNone.Unpack(); ok {
			result[chosenTag] = fi.Type
		} else {
			return result, fmt.Errorf(
				"unrecognizable struct field tag on choice declaration %s.%s: %q",
				t.Name(), fi.Name, tagValue,
			)
		}
	}
	return result, nil
}
