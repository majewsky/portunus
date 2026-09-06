// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package ber

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"

	. "go.xyrillian.de/gg/option"
)

// Unmarshal decodes a BER message into a structured data type.
//
// Primitive encodings of the following tags in the universal class may be unmarshaled:
//   - BOOLEAN into boolean types (T ~bool)
//   - INTEGER into integer types (T interface { ~int | ~int8 | ... | ~uint64 }) that do not implement [Enum]
//   - ENUMERATED into integer types (T interface { ~int | ~int8 | ... | ~uint64 }) that do implement [Enum]
//   - OCTET STRING into string types (T ~string)
//   - NULL into empty struct types (T ~struct{})
//
// Constructed encodings with the tag SEQUENCE OF in the universal class may be unmarshaled into slice types that are not an instance of [SetOf].
//
// Constructed encodings with the tag SET OF in the universal class may be unmarshaled into [SetOf] types.
//
// Constructed encodings with the tag SEQUENCE in the universal class may be unmarshaled into struct types with at least one exported field.
// The exported struct fields must be ordered like the corresponding types in the definition of the SEQUENCE type.
// Embedded fields that have a struct type behave as if the sequence of fields within that struct type was embedded into the parent type (in the same way as a COMPONENTS OF directive in the ASN.1 declaration of a SEQUENCE).
// The following tags are recognized on struct fields:
//   - `ber:"application:N"` with N a uint literal: expect a tag of N in the application class for this field (corresponds to [APPLICATION N] in the ASN.1 declaration)
//   - `ber:"context-specific:N"` with N a uint literal: expect a tag of N in the context-specific class for this field (corresponds to [N] in the ASN.1 declaration)
//   - `ber:",optional"`: allow this field to be absent (corresponds to OPTIONAL or a zero-valued DEFAULT in the ASN.1 declaration)
//
// Constructed encodings with the tag CHOICE may be unmarshaled into instances of the generic interface type [ChoiceOf], as documented over there.
//
// Any tags not documented here (as well as any pairings of types and tags not documented here) are not supported and will result in an error.
func Unmarshal[T any](message []byte) (T, error) {
	var (
		result    T
		remainder = message
	)
	err := unmarshalFullValue(&remainder, reflect.ValueOf(&result).Elem())
	if err == nil && len(remainder) != 0 {
		err = fmt.Errorf("%d unexpected bytes after end of message", len(remainder))
	}
	if err != nil {
		return result, fmt.Errorf("while unmarshaling %T: error at byte %d: %w", result, len(message)-len(remainder), err)
	}
	return result, nil
}

func unmarshalFullValue(pbuf *[]byte, target reflect.Value) error {
	hdr, err := unmarshalHeader(pbuf)
	if err != nil {
		return err
	}
	return unmarshalValue(pbuf, hdr, target)
}

func unmarshalValue(pbuf *[]byte, hdr header, target reflect.Value) error {
	switch target.Kind() {
	case reflect.Bool:
		return unmarshalBoolean(pbuf, hdr, target)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return unmarshalInteger(pbuf, hdr, target, true)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return unmarshalInteger(pbuf, hdr, target, false)
	case reflect.String:
		return unmarshalString(pbuf, hdr, target)
	case reflect.Slice:
		return unmarshalSlice(pbuf, hdr, target)
	case reflect.Struct:
		si := getStructInfo(target.Type())
		if len(si.Fields) == 0 {
			return unmarshalStructFromNull(hdr, target)
		} else {
			return unmarshalStructFromSequence(pbuf, hdr, target, si)
		}
	default:
		return fmt.Errorf("do not know how to decode into %T", target.Interface())
	}
}

func unmarshalHeader(pbuf *[]byte) (header, error) {
	buf := *pbuf
	defer func() {
		*pbuf = buf
	}()

	// parse first identifier octet [X.690, 8.1.2]
	var hdr header
	if len(buf) == 0 {
		return header{}, errors.New("expected first identifier octet, but got EOF")
	}
	hdr.Tag.Class = tagClass((buf[0] & 0b1100_0000) >> 6)
	hdr.IsConstructed = (buf[0] & 0b0010_0000) == 0b0010_0000
	hdr.Tag.Type = uint(buf[0] & 0b0001_1111)
	buf = buf[1:]

	// parse additional identifier octets, if any
	if hdr.Tag.Type == 31 {
		return header{}, errors.New("TODO: parsing of tag values > 30 is not implemented")
	}

	// parse length octets [X.690, 8.1.3]
	if len(buf) == 0 {
		return header{}, errors.New("expected first length octet, but got EOF")
	}
	firstLengthOctet := buf[0]
	buf = buf[1:]

	switch {
	case firstLengthOctet == 0b1000_0000:
		// indefinite form [X.690, 8.1.3.6]
		if !hdr.IsConstructed {
			return header{}, errors.New("length is specified in indefinite form, which is not allowed for primitive encodings")
		}
		hdr.Length = None[int]()
	case (firstLengthOctet & 0b1000_0000) == 0:
		// definite short form [X.690, 8.1.3.4]
		hdr.Length = Some(int(firstLengthOctet & 0b0111_1111))
	default:
		// definite long form [X.690, 8.1.3.5]
		numSubsequentOctets := int(firstLengthOctet & 0b0111_1111)
		if len(buf) < numSubsequentOctets {
			return header{}, fmt.Errorf("long-form length requires %d subsequent octets, but got only %d octets left", numSubsequentOctets, len(buf))
		}
		length := int(0)
		for _, b := range buf[0:numSubsequentOctets] {
			if ((length << 8) >> 8) != length {
				return header{}, fmt.Errorf("long-form length with %d subsequent octets exceeds int range", numSubsequentOctets)
			}
			length = (length << 8) | int(b)
		}
		buf = buf[numSubsequentOctets:]
		hdr.Length = Some(length)
	}

	if length, ok := hdr.Length.Unpack(); ok {
		if length > len(buf) {
			return header{}, fmt.Errorf("header with tag %s declares %d bytes of content, but only %d bytes remain to be unmarshaled", hdr.Tag, length, len(buf))
		}
	}
	return hdr, nil
}

// Unmarshals a BOOLEAN value [X.690, 8.2] into any boolean type.
func unmarshalBoolean(pbuf *[]byte, hdr header, target reflect.Value) error {
	buf := *pbuf
	defer func() {
		*pbuf = buf
	}()

	err := hdr.ExpectTagFor(target, tagBoolean)
	if err != nil {
		return err
	}
	content, err := hdr.ExpectPrimitiveAndGetContent(&buf)
	if err != nil {
		return err
	}

	if len(content) != 1 {
		return fmt.Errorf("cannot decode boolean value from %d bytes of content", len(content))
	}
	target.SetBool(content[0] != 0)
	return nil
}

// Unmarshals an INTEGER value [X.690, 8.3] into any plain signed or unsigned integer type.
func unmarshalInteger(pbuf *[]byte, hdr header, target reflect.Value, isSigned bool) error {
	buf := *pbuf
	defer func() {
		*pbuf = buf
	}()

	expectedTag := tagInteger
	if target.Type().Implements(reflect.TypeFor[Enum]()) {
		expectedTag = tagEnumerated
	}
	err := hdr.ExpectTagFor(target, expectedTag)
	if err != nil {
		return err
	}
	content, err := hdr.ExpectPrimitiveAndGetContent(&buf)
	if err != nil {
		return err
	}

	// NOTE: [X.690, 8.3.3] requires integer values to be two's complement binary numbers,
	//       so in other words, they are always signed.
	if len(content) == 0 {
		return errors.New("cannot decode integer value from 0 bytes of content")
	}
	if len(content) > 8 {
		return fmt.Errorf("value 0x%s is longer than 8 bytes and overflows int64", hex.EncodeToString(content))
	}
	if len(content) < 8 {
		// expand `content` to 8 bytes using sign extension (Uint64() panics on less than 8 bytes)
		filler := byte(0b0000_0000)
		if (content[0] & 0b1000_0000) == 0b1000_0000 {
			filler = 0b1111_1111
		}
		content = append(bytes.Repeat([]byte{filler}, 8-len(content)), content...)
	}
	value := int64(binary.BigEndian.Uint64(content))
	// NOTE: This looks like an overflow risk, but isn't.
	// BigEndian does not have a method for reading int64.
	// The intended approach is to read uint64 and then do an intentional reinterpret_cast.

	// try to put this value into `target`
	if isSigned {
		if target.OverflowInt(value) {
			return fmt.Errorf("value %d overflows %T", value, target.Interface())
		}
		target.SetInt(value)
	} else {
		if value < 0 {
			return fmt.Errorf("value %d overflows %T", value, target.Interface())
		}
		unsignedValue := uint64(value)
		if target.OverflowUint(unsignedValue) {
			return fmt.Errorf("value %d overflows %T", value, target.Interface())
		}
		target.SetUint(unsignedValue)
	}

	if expectedTag == tagEnumerated {
		targetAsEnum := target.Interface().(Enum) // this cast cannot fail because we checked Implements() earlier
		return targetAsEnum.Validate()
	} else {
		return nil
	}
}

// Unmarshals an OCTET STRING value [X.690, 8.7] into a string type.
func unmarshalString(pbuf *[]byte, hdr header, target reflect.Value) error {
	buf := *pbuf
	defer func() {
		*pbuf = buf
	}()

	err := hdr.ExpectTagFor(target, tagOctetString)
	if err != nil {
		return err
	}
	content, err := hdr.ExpectPrimitiveAndGetContent(&buf)
	if err != nil {
		return err
	}

	target.SetString(string(content))
	return nil
}

// Unmarshals a SEQUENCE OF value [X.690, 8.10] or SET OF value [X.690, 8.12] into a slice type.
func unmarshalSlice(pbuf *[]byte, hdr header, target reflect.Value) error {
	nativeTag := tagSequence
	if target.Type().Implements(reflect.TypeFor[encodesWithSetOf]()) {
		nativeTag = tagSet
	}
	err := hdr.ExpectTagFor(target, nativeTag)
	if err != nil {
		return err
	}

	return unmarshalConstructedSequence(pbuf, hdr, func(pbuf *[]byte, hdr header) error {
		targetVal := reflect.New(target.Type().Elem()).Elem()
		err := unmarshalValue(pbuf, hdr, targetVal)
		if err != nil {
			return err
		}
		target.Set(reflect.Append(target, targetVal))
		return nil
	})
}

// Unmarshals a NULL value [X.690, 8.8] into a fieldless struct.
func unmarshalStructFromNull(hdr header, target reflect.Value) error {
	err := hdr.ExpectTagFor(target, tagNull)
	if err != nil {
		return err
	}
	err = hdr.ExpectPrimitive()
	if err != nil {
		return err
	}
	if hdr.Length != Some(0) {
		return fmt.Errorf("expected 0 bytes, but got %d bytes of content for NULL value", hdr.Length.UnwrapOr(0))
	}
	return nil
}

// Unmarshals a SEQUENCE value [X.690, 8.9] into a struct.
func unmarshalStructFromSequence(pbuf *[]byte, hdr header, target reflect.Value, si structInfo) error {
	err := hdr.ExpectTagFor(target, tagSequence)
	if err != nil {
		return err
	}

	nextField := 0 // index into `si.Fields`
	err = unmarshalConstructedSequence(pbuf, hdr, func(pbuf *[]byte, hdr header) error {
		if nextField >= len(si.Fields) {
			return fmt.Errorf("while unmarshaling %T: %d bytes of content remain unused after having unmarshaled all fields", target.Interface(), len(*pbuf))
		}

		// try to match with next field;
		// if fields are optional and there is no match, we skip to the next match [X.690, 8.9.3]
		var skippedFields []string
		for {
			fi := si.Fields[nextField]
			if fi.acceptsTag(hdr.Tag) || !fi.IsOptional {
				break
			}
			skippedFields = append(skippedFields, fi.Name)
			nextField++
			if nextField == len(si.Fields) {
				return fmt.Errorf("while unmarshaling %T: no suitable target fields remain for tag %s after skipping %v",
					target.Interface(), hdr.Tag, skippedFields)
			}
		}
		fi := si.Fields[nextField]
		if !fi.acceptsTag(hdr.Tag) {
			if len(skippedFields) > 0 {
				return fmt.Errorf("while unmarshaling %T: cannot decode value with tag %s into field %s after skipping %v",
					target.Interface(), hdr.Tag, fi.Name, skippedFields)
			} else {
				return fmt.Errorf("while unmarshaling %T: cannot decode value with tag %s into field %s",
					target.Interface(), hdr.Tag, fi.Name)
			}
		}

		// consume value
		var err error
		if len(fi.TagChoices) > 0 {
			err = unmarshalChoice(pbuf, hdr, target.FieldByIndex(fi.Index), fi.TagChoices[hdr.Tag])
		} else {
			err = unmarshalValue(pbuf, hdr, target.FieldByIndex(fi.Index))
		}
		if err != nil {
			return fmt.Errorf("while unmarshaling field %s of %T: %w", fi.Name, target.Interface(), err)
		}
		nextField++
		return nil
	})
	if err != nil {
		return err
	}

	for _, fi := range si.Fields[nextField:] {
		if !fi.IsOptional {
			return fmt.Errorf("while unmarshaling %T: missing required field %q", target.Interface(), fi.Name)
		}
	}
	return nil
}

// Helper for unmarshaling a constructed encoding that contains a sequence of encodings
// (used for SEQUENCE, SEQUENCE OF and SET OF values).
func unmarshalConstructedSequence(pbuf *[]byte, hdr header, unmarshalElement func(*[]byte, header) error) error {
	err := hdr.ExpectConstructed()
	if err != nil {
		return err
	}

	// while reading data values within the SEQUENCE/SET, ensure that we do not read over its length boundary
	var (
		bufFull  = *pbuf // full remainder (incl. encoded values that come after our SEQUENCE value)
		buf      []byte  // prefix of `bufFull`, limited to what can be part of the SEQUENCE value
		needsEOC bool    // whether we expect an end-of-contents marker
	)
	if length, ok := hdr.Length.Unpack(); ok {
		buf = bufFull[:length]
		defer func() {
			*pbuf = bufFull[length-len(buf):]
		}()
		needsEOC = false
	} else {
		buf = bufFull
		defer func() {
			*pbuf = buf
		}()
		needsEOC = true
	}

	// NOTE: In a struct (SEQUENCE), all fields may be optional, so `len(buf) == 0` can legitimately happen.
	//       And similarly, a slice (SEQUENCE OF/SET OF) may have zero elements.
	for len(buf) > 0 {
		// unmarshal header of next data value in sequence [X.690, 8.9.2]
		hdr, err := unmarshalHeader(&buf)
		if err != nil {
			return err
		}
		if hdr.Tag == tagEndOfContents && needsEOC {
			break
		}

		// unmarshal value with this header
		err = unmarshalElement(&buf, hdr)
		if err != nil {
			return err
		}
	}

	return nil
}

// `target` is the interface type, i.e. ber.ChoiceOf[T] for some T.
// `chosenType` is a type that implements T.
func unmarshalChoice(pbuf *[]byte, hdr header, target reflect.Value, chosenType reflect.Type) error {
	// Within a CHOICE value, the tag of the choice replaces the native tag of the payload type [X.690, 8.13].
	// This will confuse unmarshalValue(), so we are substituting the native tag before dispatching.
	if nativeTag, ok := nativeTagFor(chosenType).Unpack(); ok {
		hdr.Tag = nativeTag
	}

	value := reflect.New(chosenType).Elem()
	err := unmarshalValue(pbuf, hdr, value)
	if err != nil {
		return err
	}
	target.Set(value)
	return nil
}
