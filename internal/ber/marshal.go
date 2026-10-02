// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package ber

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"

	. "go.xyrillian.de/gg/option"
)

// Marshal encodes a structured data type into a BER message.
// This function is the exact reverse of [Unmarshal]:
//   - Only types that can be returned by [Unmarshal] may be given to this function.
//   - All inputs will be serialized in a way that [Unmarshal] is able to parse.
func Marshal(data any) ([]byte, error) {
	var buf bytes.Buffer
	err := marshalValue(&buf, reflect.ValueOf(data), None[tag]())
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), err
}

func marshalValue(buf *bytes.Buffer, v reflect.Value, t Option[tag]) error {
	switch v.Kind() {
	case reflect.Bool:
		return marshalBoolean(buf, v.Bool(), t.UnwrapOr(tagBoolean))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value := uint64(v.Int()) // intentional reinterpret_cast; binary.BigEndian can only encode unsigned and not signed integers
		return marshalInteger(buf, value, t.UnwrapOr(nativeTagForInteger(v.Type())))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return marshalInteger(buf, v.Uint(), t.UnwrapOr(nativeTagForInteger(v.Type())))
	case reflect.String:
		return marshalString(buf, v.String(), t.UnwrapOr(tagOctetString))
	case reflect.Slice:
		return marshalSlice(buf, v, t.UnwrapOr(nativeTagForSlice(v.Type())))
	case reflect.Struct:
		si := getStructInfo(v.Type())
		return marshalStruct(buf, v, si, t.UnwrapOr(nativeTagForStruct(si)))
	case reflect.Interface:
		return marshalChoice(buf, v)
	default:
		return fmt.Errorf("do not know how to encode %T", v.Interface())
	}
}

// marshalHeader writes identifier octets [X.690, 8.1.2] and length octets [X.690, 8.1.3] for a data value.
//
// If hdr.Length.IsNone(), the length octets will initially be in indefinite form,
// but this is always a transitory state: We never emit encodings with end-of-contents octets.
// Instead, the returned offset will point at the length octet, and once the length of the contents octets is known,
// finalizeLengthOctets() must be called to write the length octets, shifting the subsequent contents octets if necessary.
func marshalHeader(buf *bytes.Buffer, hdr header) (offset int, _ error) {
	// emit identifier octet [X.690, 8.1.2]
	if hdr.Tag.Type > 30 {
		return 0, errors.New("TODO: encoding of tag values > 30 is not implemented")
	}
	identifier := byte(hdr.Tag.Type) | (byte(hdr.Tag.Class)&0b0000_0011)<<6
	if hdr.IsConstructed {
		identifier |= 0b0010_0000
	}
	err := buf.WriteByte(identifier)
	if err != nil {
		return 0, err
	}

	// emit length octet [X.690, 8.1.3]
	offset = buf.Len()
	if length, ok := hdr.Length.Unpack(); ok {
		if length < 0 {
			panic(fmt.Sprintf("unexpected negative value for hdr.Length: %d", length))
		} else {
			var ibuf [9]byte
			payload := prepareLengthOctets(ibuf, uint64(length))
			_, err = buf.Write(payload)
			return offset, err
		}
	} else {
		// write indefinite form temporarily
		return offset, buf.WriteByte(0b1000_0000)
	}
}

// finalizeLengthOctets finalizes the length octets of a constructed encoding,
// where marshalHeader() was initially called without a known length.
//
// This call is made after all contents octets have been written,
// so everything past the given offset containing the length octet is considered
// to be part of the contents octets of the data value's constructed encoding.
//
// The length of this part of the buffer is written into the length octet.
// If the encoding of the length requires multiple length octets,
// the contents octets are shifted to make room.
func finalizeLengthOctets(buf *bytes.Buffer, offset int) error {
	// `offset` points at the length octets, where the 1-byte-long indefinite
	// form was written temporarily; everything in the buffer after that byte
	// is the contents bytes
	length := buf.Len() - (offset + 1)
	if length < 0 {
		panic(fmt.Sprintf("finalizeLengthOctets got invalid buffer state: buf.Len() = %d, offset = %d", buf.Len(), offset))
	}

	// prepare encoding of length octets
	var ibuf [9]byte
	payload := prepareLengthOctets(ibuf, uint64(length))

	// insert into prepared location if possible
	pbuf := buf.Bytes()
	if len(payload) == 1 {
		pbuf[offset] = payload[0]
		return nil
	}

	// shift contents octets to make room for the length octets
	oldLen := buf.Len()
	blank := make([]byte, len(payload)-1)
	_, err := buf.Write(blank)
	if err != nil {
		return err
	}
	newLen := buf.Len()
	copy(pbuf[oldLen-length:oldLen], pbuf[newLen-length:newLen])
	return nil
}

func marshalPrimitive(buf *bytes.Buffer, payload []byte, t tag) error {
	_, err := marshalHeader(buf, header{
		Tag:           t,
		IsConstructed: false,
		Length:        Some(len(payload)),
	})
	if err != nil {
		return err
	}
	_, err = buf.Write(payload)
	return err
}

func marshalBoolean(buf *bytes.Buffer, value bool, t tag) error {
	var payload [1]byte
	if value {
		payload[0] = 0xFF
	} else {
		payload[0] = 0x00
	}
	return marshalPrimitive(buf, payload[:], t)
}

func marshalInteger(buf *bytes.Buffer, value uint64, t tag) error {
	var ibuf [8]byte
	return marshalPrimitive(buf, prepareIntegerRepresentation(ibuf[:], value), t)
}

func prepareLengthOctets(buf [9]byte, length uint64) []byte {
	if length <= 127 {
		// definite short form [X.690, 8.1.3.4]
		buf[0] = byte(length)
		return buf[0:1]
	} else {
		// definite long form [X.690, 8.1.3.5]
		payload := prepareIntegerRepresentation(buf[1:], uint64(length))
		buf[0] = byte(len(payload))
		return buf[0 : 1+len(payload)]
	}
}

// `buf` must be at least 8 bytes long!
func prepareIntegerRepresentation(buf []byte, value uint64) []byte {
	binary.BigEndian.PutUint64(buf[:], value)
	result := buf[:]

	// truncate to shortest representation (this reverses the sign extension from unmarshalInteger())
	//
	// e.g. 0x00_00_00_00_00_8d_2f_25 -> 0x00_8d_2f_25
	// e.g. 0xff_ff_ff_ff_ff_ff_ff_fe -> 0xfe
	for len(result) > 1 {
		if result[0] == 0x00 && (result[1]&0b1000_0000) == 0 {
			result = result[1:]
		} else if result[0] == 0xFF && (result[1]&0b1000_0000) == 0b1000_0000 {
			result = result[1:]
		} else {
			break
		}
	}
	return result
}

func marshalString(buf *bytes.Buffer, value string, t tag) error {
	return marshalPrimitive(buf, []byte(value), t)
}

func marshalSlice(buf *bytes.Buffer, v reflect.Value, t tag) error {
	offset, err := marshalHeader(buf, header{Tag: t, IsConstructed: true})
	if err != nil {
		return err
	}
	for idx := range v.Len() {
		err := marshalValue(buf, v.Index(idx), None[tag]())
		if err != nil {
			// TODO: general remark for later, should we be providing error traces in all the marshalFoo() functions?
			return err
		}
	}
	return finalizeLengthOctets(buf, offset)
}

func marshalStruct(buf *bytes.Buffer, v reflect.Value, si structInfo, t tag) error {
	// marshal zero-field structs as NULL values [X.690, 8.8]
	if len(si.Fields) == 0 {
		_, err := marshalHeader(buf, header{
			Tag:           tagNull,
			IsConstructed: false,
			Length:        Some(0),
		})
		return err
	}

	// marshal all other sturcts as SEQUENCE values [X.690, 8.9]
	offset, err := marshalHeader(buf, header{Tag: tagSequence, IsConstructed: true})
	if err != nil {
		return err
	}

	for _, fi := range si.Fields {
		fv := v.FieldByIndex(fi.Index)
		if fi.IsOptional && fv.IsZero() {
			continue
		}
		if len(fi.TagChoices) == 0 {
			// for most types, force marshaling with the specific tag for this field
			// (which may have been read from a `ber:"..."` struct tag, or otherwise
			// is the native tag for the type of `fv`)
			err = marshalValue(buf, fv, Some(fi.Tag))
		} else {
			// for choice types, allow marshalChoice() to pick freely
			err = marshalValue(buf, fv, None[tag]())
		}
		if err != nil {
			return fmt.Errorf("while marshaling %T.%s: %w", v.Interface(), fi.Name, err)
		}
	}
	return finalizeLengthOctets(buf, offset)
}

func marshalChoice(buf *bytes.Buffer, v reflect.Value) error {
	// we get a ber.ChoiceOf[T], but we want the value in its concrete type
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	// wrap the concrete type implementing ber.ChoiceOf[T] into the struct type T
	cv, ok := castChoiceType(v)
	if !ok {
		return fmt.Errorf("invalid type %T given to marshalChoice: does not implement any ber.ChoiceOf", v.Interface())
	}

	// find tag to use for this concrete value
	// TODO: inefficient, should cache this lookup (in the correct direction) at buildStructInfo() time
	tagChoices, err := buildTagChoices(cv.Type())
	if err != nil {
		return fmt.Errorf("invalid type %T given to marshalChoice: %w", v.Interface(), err)
	}
	for tag, rtype := range tagChoices {
		if rtype == v.Type() {
			return marshalValue(buf, v, Some(tag))
		}
	}
	return fmt.Errorf("invalid type %T given to marshalChoice: not a valid choice for %T (no tag declared)", v.Interface(), cv.Interface())
}
