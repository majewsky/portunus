// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package ber

import (
	"fmt"
	"reflect"

	. "go.xyrillian.de/gg/option"
)

// header holds the contents of the identifier octets and length octets of an encoding.
type header struct {
	Tag           tag
	IsConstructed bool        // false = primitive encoding, true = constructed encoding
	Length        Option[int] // None = indefinite form (content must be followed by end-of-contents octets)
}

// ExpectTagFor is a helper for checking that an encoded value uses the right tag.
func (h header) ExpectTagFor(target reflect.Value, tag tag) error {
	if h.Tag != tag {
		return fmt.Errorf("expected %T to be encoded with tag %s, but got tag %s", target.Interface(), tag, h.Tag)
	}
	return nil
}

// ExpectConstructed is a helper for checking that a value uses a constructed encoding.
func (h header) ExpectConstructed() error {
	if !h.IsConstructed {
		return fmt.Errorf("expected constructed encoding for tag %s, but got primitive encoding", h.Tag)
	}
	return nil
}

// ExpectPrimitive is a helper for checking that a value uses a primitive encoding.
func (h header) ExpectPrimitive() error {
	if h.IsConstructed {
		return fmt.Errorf("expected primitive encoding for tag %s, but got constructed encoding", h.Tag)
	}
	return nil
}

// ExpectPrimitiveAndGetContent is like ExpectPrimitive, but also clips the content from `pbuf`.
// This is always possible because primitive encodings must declare a definite length.
func (h header) ExpectPrimitiveAndGetContent(pbuf *[]byte) ([]byte, error) {
	err := h.ExpectPrimitive()
	if err != nil {
		return nil, err
	}
	contentLength := h.Length.UnwrapOrPanic("length cannot be indefinite in primitive encodings")
	content := (*pbuf)[0:contentLength]
	*pbuf = (*pbuf)[contentLength:]
	return content, nil
}

// tag holds tag values. It appears in type [header] and type [fieldInfo].
type tag struct {
	Class tagClass
	Type  uint
}

var (
	tagEndOfContents = tag{tagClassUniversal, 0}
	tagBoolean       = tag{tagClassUniversal, 1}
	tagInteger       = tag{tagClassUniversal, 2}
	tagOctetString   = tag{tagClassUniversal, 4}
	tagNull          = tag{tagClassUniversal, 5}
	tagEnumerated    = tag{tagClassUniversal, 10}
	tagSequence      = tag{tagClassUniversal, 16}
	tagSet           = tag{tagClassUniversal, 17}
)

// String implements the [fmt.Stringer] interface.
func (t tag) String() string {
	return fmt.Sprintf("%s:%d", t.Class, t.Type)
}

// nativeTagFor returns the default tag for the given type.
func nativeTagFor(t reflect.Type) Option[tag] {
	switch t.Kind() {
	case reflect.Bool:
		return Some(tagBoolean)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return Some(nativeTagForInteger(t))
	case reflect.String:
		return Some(tagOctetString)
	case reflect.Slice:
		return Some(nativeTagForSlice(t))
	case reflect.Struct:
		si := getStructInfo(t)
		return Some(nativeTagForStruct(si))
	default:
		return None[tag]()
	}
}

func nativeTagForInteger(t reflect.Type) tag {
	if t.Implements(reflect.TypeFor[Enum]()) {
		return tagEnumerated
	} else {
		return tagInteger
	}
}

func nativeTagForSlice(t reflect.Type) tag {
	if t.Implements(reflect.TypeFor[encodesWithSetOf]()) {
		return tagSet
	} else {
		return tagSequence
	}
}

func nativeTagForStruct(si structInfo) tag {
	if len(si.Fields) == 0 {
		return tagNull
	} else {
		return tagSequence
	}
}

// tagClass is an enum. It appears in type [tag].
type tagClass byte

const (
	// The values of these enum variants match their encoding in identifier octets. [X.690, 8.1.2.2]
	tagClassUniversal       tagClass = 0b00
	tagClassApplication     tagClass = 0b01
	tagClassContextSpecific tagClass = 0b10
	tagClassPrivate         tagClass = 0b11
)

// String implements the [fmt.Stringer] interface.
func (c tagClass) String() string {
	switch c {
	case tagClassUniversal:
		return "universal"
	case tagClassApplication:
		return "application"
	case tagClassContextSpecific:
		return "context-specific"
	case tagClassPrivate:
		return "private"
	default:
		panic(fmt.Sprintf("unknown tag class: %d", c))
	}
}
