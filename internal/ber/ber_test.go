// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package ber_test

import (
	"fmt"
	"strings"
	"testing"

	"go.xyrillian.de/gg/assert"

	"github.com/majewsky/portunus/internal/ber"
)

func expectParses[T any](t *testing.T, value T, encodings ...string) {
	t.Helper()

	// all encodings must be valid representations of `value`
	for idx, encoding := range encodings {
		actual, err := ber.Unmarshal[T]([]byte(encoding))
		if assert.ErrEqual(t, err, nil) {
			if !assert.Equal(t, actual, value) {
				t.Logf("^ this discrepancy was from ber.Unmarshal() of encoding %d: %q", idx, encoding)
			}
		} else {
			t.Logf("^ this error was from ber.Unmarshal() of encoding %d: %q", idx, encoding)
		}
	}

	// the first encoding must roundtrip (i.e. be the DER encoding)
	if len(encodings) > 0 {
		actual, err := ber.Marshal(value)
		if assert.ErrEqual(t, err, nil) {
			if !assert.Equal(t, string(actual), encodings[0]) {
				t.Log("^ this discrepancy was from ber.Marshal()")
			}
		} else {
			t.Log("^ this error was from ber.Marshal()")
		}
	}
}

func expectParseError[T any](t *testing.T, encoding, expectedError string) {
	t.Helper()
	_, err := ber.Unmarshal[T]([]byte(encoding))
	var zero T
	assert.ErrEqual(t, err, fmt.Sprintf("while unmarshaling %T: %s", zero, expectedError))
}

type mockEnum int

const (
	mockEnumOne mockEnum = 1
	mockEnumTwo mockEnum = 2
)

// IsEnum implements the [ber.Enum] interface.
func (mockEnum) IsEnum() bool { return true }

// Validate implements the [ber.Enum] interface.
func (e mockEnum) Validate() error {
	if e == mockEnumOne || e == mockEnumTwo {
		return nil
	}
	return fmt.Errorf("expected mockEnum to have value 1 or 2, but got %d", e)
}

type (
	mockChoice struct {
		mockChoiceString `ber:"application:0"`
		mockChoiceInt    `ber:"application:1"`
		mockChoiceStruct `ber:"application:2"`
	}

	mockChoiceString string
	mockChoiceInt    int
	mockChoiceStruct struct {
		ID   int
		Name string
	}
)

var (
	_ ber.ChoiceOf[mockChoice] = mockChoiceString("")
	_ ber.ChoiceOf[mockChoice] = mockChoiceInt(0)
	_ ber.ChoiceOf[mockChoice] = mockChoiceStruct{}
)

// Into implements the [ber.ChoiceOf] interface.
func (c mockChoiceString) Into() mockChoice { return mockChoice{mockChoiceString: c} }

// Into implements the [ber.ChoiceOf] interface.
func (c mockChoiceInt) Into() mockChoice { return mockChoice{mockChoiceInt: c} }

// Into implements the [ber.ChoiceOf] interface.
func (c mockChoiceStruct) Into() mockChoice { return mockChoice{mockChoiceStruct: c} }

func TestPrimitiveValues(t *testing.T) {
	// boolean [X.690, 8.2] [X.690, 11.1]
	expectParses(t, false, "\x01\x01\x00")
	expectParses(t, true, "\x01\x01\xFF", "\x01\x01\x01")
	expectParseError[bool](t, "\x01\x00",
		"error at byte 2: cannot decode boolean value from 0 bytes of content")
	expectParseError[bool](t, "\x01\x02\x00\x00",
		"error at byte 4: cannot decode boolean value from 2 bytes of content")
	expectParseError[bool](t, "\x21\x02\x00\x00",
		"error at byte 2: expected primitive encoding for tag universal:1, but got constructed encoding")

	// integer [X.690, 8.3]
	expectParses(t, 0,
		// on each line, first shortest form of contents octets, then overlong contents octets with sign extension
		"\x02\x01\x00", "\x02\x02\x00\x00", // length in definite short form
		"\x02\x81\x01\x00", "\x02\x81\x02\x00\x00", // length in definite long form
		"\x02\x82\x00\x01\x00", "\x02\x82\x00\x02\x00\x00", // length in overlong definite long form
		"\x02\x89\x00\x00\x00\x00\x00\x00\x00\x00\x01\x00", // absurdly overlong definite long form (more than 8 bytes, but the encoded value fits in int)
	)
	expectParseError[int](t, "\x02\x00", // [X.590, 8.3.1] requires at least one byte of content
		"error at byte 2: cannot decode integer value from 0 bytes of content")
	expectParseError[int](t, "\x22\x01\x00",
		"error at byte 2: expected primitive encoding for tag universal:2, but got constructed encoding")
	expectParses(t, 1000, // 1000 = 0x03e8
		"\x02\x02\x03\xE8", "\x02\x04\x00\x00\x03\xE8",
		"\x02\x81\x02\x03\xE8", "\x02\x81\x04\x00\x00\x03\xE8",
		"\x02\x82\x00\x02\x03\xE8", "\x02\x82\x00\x04\x00\x00\x03\xE8",
	)
	expectParseError[uint8](t, "\x02\x02\x03\xE8",
		"error at byte 4: value 1000 overflows uint8")
	expectParses(t, -1000, // -1000 = 0xfc18
		"\x02\x02\xFC\x18", "\x02\x04\xFF\xFF\xFC\x18",
		"\x02\x81\x02\xFC\x18", "\x02\x81\x04\xFF\xFF\xFC\x18",
		"\x02\x82\x00\x02\xFC\x18", "\x02\x82\x00\x04\xFF\xFF\xFC\x18",
	)
	expectParseError[uint](t, "\x02\x02\xFC\x18",
		"error at byte 4: value -1000 overflows uint")
	expectParseError[int8](t, "\x02\x02\xFC\x18",
		"error at byte 4: value -1000 overflows int8")
	expectParseError[uint64](t, "\x02\x09\x11\x11\x11\x11\x11\x11\x11\x11\x11",
		"error at byte 11: value 0x111111111111111111 is longer than 8 bytes and overflows int64")

	// as an exception, extremely long encodings are allowed if removing the sign extension makes the value fit
	expectParses(t, uint64(1000),
		"\x02\x02\x03\xE8",
		"\x02\x09\x00\x00\x00\x00\x00\x00\x00\x03\xE8",
	)
	expectParses(t, int64(-1000),
		"\x02\x02\xFC\x18",
		"\x02\x09\xFF\xFF\xFF\xFF\xFF\xFF\xFF\xFC\x18",
	)

	// NOTE: Beyond this point, we do not test all the different encodings of length octets again and again, and only use DER-compliant length octets.
	//       This is acceptable because decoding for all types shares the same header parsing logic.

	// enumerated integers [X.690, 8.4]
	expectParses(t, mockEnumOne, "\x0A\x01\x01")
	expectParses(t, mockEnumTwo, "\x0A\x01\x02")
	expectParseError[mockEnum](t, "\x0A\x01\x03", "error at byte 3: expected mockEnum to have value 1 or 2, but got 3")

	// octet string [X.690, 8.7.2]
	expectParses(t, "", "\x04\x00")
	expectParses(t, []byte(""), "\x04\x00")
	expectParses(t, "Hello", "\x04\x05Hello")
	expectParses(t, []byte("Hello"), "\x04\x05Hello")

	// beyond 127 bytes of payload, length octets must be in definite long form even for DER encoding
	text := strings.Repeat("=", 127)
	expectParses(t, text, "\x04\x7F"+text, "\x04\x81\x7F"+text, "\x04\x82\x00\x7F"+text)
	expectParses(t, []byte(text), "\x04\x7F"+text, "\x04\x81\x7F"+text, "\x04\x82\x00\x7F"+text)
	text = strings.Repeat("=", 128)
	expectParses(t, text, "\x04\x81\x80"+text, "\x04\x82\x00\x80"+text)
	expectParses(t, []byte(text), "\x04\x81\x80"+text, "\x04\x82\x00\x80"+text)
	text = strings.Repeat("=", 129)
	expectParses(t, text, "\x04\x81\x81"+text, "\x04\x82\x00\x81"+text)
	expectParses(t, []byte(text), "\x04\x81\x81"+text, "\x04\x82\x00\x81"+text)

	// null [X.690, 8.8]
	expectParses(t, struct{}{}, "\x05\x00")
	expectParseError[struct{}](t, "\x05\x01\x00",
		"error at byte 2: expected 0 bytes, but got 1 bytes of content for NULL value")
	expectParseError[struct{}](t, "\x25\x00",
		"error at byte 2: expected primitive encoding for tag universal:5, but got constructed encoding")

	// type-independent parse errors
	expectParseError[string](t, "\x04\x05Hello\xFF",
		"error at byte 7: 1 unexpected bytes after end of message")
	expectParseError[string](t, "\x04\x05Hi",
		"error at byte 2: header with tag universal:4 declares 5 bytes of content, but only 2 bytes remain to be unmarshaled")
	expectParseError[string](t, "\x02\x05Hello",
		"error at byte 2: expected string to be encoded with tag universal:4, but got tag universal:2")
	expectParseError[string](t, "",
		"error at byte 0: expected first identifier octet, but got EOF")
	expectParseError[string](t, "\x04",
		"error at byte 1: expected first length octet, but got EOF")
	expectParseError[string](t, "\x04\x82\x00",
		"error at byte 2: long-form length requires 2 subsequent octets, but got only 1 octets left")
	expectParseError[string](t, "\x04\x89\x11\x11\x11\x11\x11\x11\x11\x11\x11",
		"error at byte 2: long-form length value 0x111111111111111111 exceeds int range")
	expectParseError[string](t, "\x04\xFF",
		"error at byte 2: length is specified as 0xFF which is an invalid encoding")
	expectParseError[string](t, "\x04\x80Hello\x00\x00",
		"error at byte 2: length is specified in indefinite form, which is not allowed for primitive encodings")
	expectParseError[chan string](t, "\x01\x00",
		"error at byte 2: do not know how to decode into chan string")
}

func TestConstructedValues(t *testing.T) {
	// octet string [X.690, 8.7.3]
	expectParses(t, "abcd",
		// DER never uses constructed encoding for octet strings
		"\x04\x04abcd",
		// "a" + "bcd" with definite form on length octets of constructed encoding
		"\x24\x08"+"\x04\x01a"+"\x04\x03bcd",
		// "a" + "bc" + "d" with indefinite form on length octets of constructed encoding
		"\x24\x80"+"\x04\x01a"+"\x04\x02bc"+"\x04\x01d"+"\x00\x00",
		// "a" + ("b" + "cd") with nested constructed encoding
		"\x24\x80"+"\x04\x01a"+"\x24\x07"+"\x04\x01b"+"\x04\x02cd"+"\x00\x00",
	)

	// sequence-of [X.690, 8.10]
	sequenceEncodings := []string{
		// DER encoding
		"\x30\x0F" + "\x04\x03foo" + "\x04\x03bar" + "\x04\x03baz",
		// nested constructed encoding: "bar" is encoded as "bar"+"" (an objectively insane, yet valid construction)
		"\x30\x13" + "\x04\x03foo" + "\x24\x07\x04\x03bar\x04\x00" + "\x04\x03baz",
		// indefinite form on length octets of outer constructed encoding
		"\x30\x80" + "\x04\x03foo" + "\x24\x07\x04\x03bar\x04\x00" + "\x04\x03baz" + "\x00\x00",
		// indefinite form on length octets of both constructed encodings
		"\x30\x80" + "\x04\x03foo" + "\x24\x80\x04\x03bar\x04\x00\x00\x00" + "\x04\x03baz" + "\x00\x00",
	}
	expectParses(t, []string{"foo", "bar", "baz"}, sequenceEncodings...)
	expectParseError[[]string](t,
		// same as DER encoding above, but first encoding is primitive
		"\x10\x0F"+"\x04\x03foo"+"\x04\x03bar"+"\x04\x03baz",
		"error at byte 2: expected constructed encoding for tag universal:16, but got primitive encoding",
	)

	// set-of [X.690, 8.12] using the same examples as above, just a different tag value (0x11 instead of 0x10)
	setEncodings := make([]string, len(sequenceEncodings))
	for idx, value := range sequenceEncodings {
		setEncodings[idx] = "\x31" + strings.TrimPrefix(value, "\x30")
	}
	expectParses(t, ber.SetOf[string]{"foo", "bar", "baz"}, setEncodings...)

	// NOTE: Beyond this point, we do not test all the different encodings of length octets again and again, and only use DER-compliant length octets.
	//       This is acceptable because decoding for all types shares the same header parsing logic.

	// sequence [X.690, 8.9]
	type simpleRecord struct {
		ID       int
		Category int    `ber:",optional"`
		Name     string `ber:",optional"`
	}
	expectParses(t, simpleRecord{42, 5, "Alice"},
		"\x30\x0D"+"\x02\x01\x2A"+"\x02\x01\x05"+"\x04\x05Alice",
	)
	expectParses(t, simpleRecord{23, 0, "Bob"},
		// omitted zero value for optional field
		"\x30\x08"+"\x02\x01\x17"+"\x04\x03Bob",
		// explicit zero value for optional field
		"\x30\x0B"+"\x02\x01\x17"+"\x02\x01\x00"+"\x04\x03Bob",
	)
	expectParses(t, simpleRecord{1337, 0, ""},
		// multiple trailing optional fields omitted
		"\x30\x04"+"\x02\x02\x05\x39",
		// explicit zero values for all optional fields
		"\x30\x09"+"\x02\x02\x05\x39"+"\x02\x01\x00"+"\x04\x00",
	)
	expectParseError[simpleRecord](t,
		// first example again, but with the fields rearranged into the wrong order
		"\x30\x0D"+"\x04\x05Alice"+"\x02\x01\x2A"+"\x02\x01\x05",
		"error at byte 4: in field ID of ber_test.simpleRecord: cannot decode value with tag universal:4",
	)
	expectParseError[simpleRecord](t,
		// first example again, but with an unexpected custom tag
		"\x30\x0D"+"\x02\x01\x2A"+"\x02\x01\x05"+"\xC4\x05Alice",
		"error at byte 10: in ber_test.simpleRecord: no suitable target fields remain for tag private:4 after skipping [Name]",
	)
	expectParseError[simpleRecord](t,
		// unexpected end of sequence
		"\x30\x00",
		`error at byte 2: in ber_test.simpleRecord: missing required field "ID"`,
	)
	expectParseError[simpleRecord](t,
		// unexpected additional fields
		"\x30\x16"+"\x02\x01\x2A"+"\x02\x01\x05"+"\x04\x05Alice"+"\x04\x07Allison",
		`error at byte 17: in ber_test.simpleRecord: 7 bytes of content remain unused after having unmarshaled all fields`,
	)
	expectParseError[simpleRecord](t,
		// unexpected primitive encoding
		"\x10\x0D"+"\x02\x01\x2A"+"\x02\x01\x05"+"\x04\x05Alice",
		"error at byte 2: expected constructed encoding for tag universal:16, but got primitive encoding",
	)

	type simpleRecordWithRequiredName struct {
		ID       int
		Category int `ber:",optional"`
		Name     string
	}
	expectParses(t, simpleRecordWithRequiredName{42, 5, "Alice"},
		// same as above: all fields populated
		"\x30\x0D"+"\x02\x01\x2A"+"\x02\x01\x05"+"\x04\x05Alice",
	)
	expectParses(t, simpleRecord{23, 0, "Bob"},
		// same as above: "Category" field omitted or explicitly set to zero
		"\x30\x08"+"\x02\x01\x17"+"\x04\x03Bob",
		"\x30\x0B"+"\x02\x01\x17"+"\x02\x01\x00"+"\x04\x03Bob",
	)
	expectParseError[simpleRecordWithRequiredName](t,
		// encoded value `true` cannot be matched with field even after skipping optional fields
		"\x30\x0B"+"\x02\x01\x17"+"\x01\x01\x00"+"\x04\x03Bob",
		`error at byte 7: in field Name of ber_test.simpleRecordWithRequiredName (after skipping [Category]): cannot decode value with tag universal:1`,
	)

	// choice [X.690, 8.13]: the declared tag on the choice option overrides the native tag of the underlying type
	type recordWithChoice struct {
		// ^ this outer type is to force coercion of `mockChoiceFoo` values into the `ber.ChoiceOf[mockChoice]` interface type
		Choice ber.ChoiceOf[mockChoice]
	}
	expectParses(t, recordWithChoice{mockChoiceString("Alice")},
		"\x30\x07"+"\x40\x05Alice",
	)
	expectParses(t, recordWithChoice{mockChoiceInt(42)},
		"\x30\x03"+"\x41\x01\x2A",
	)
	expectParses(t, recordWithChoice{mockChoiceStruct{42, "Alice"}},
		"\x30\x0C"+"\x62\x0A"+"\x02\x01\x2A"+"\x04\x05Alice",
	)
	expectParseError[recordWithChoice](t,
		"\x30\x03"+"\x81\x01\x2A", // Choice has unknown tag "context-specific:1"
		"error at byte 4: in field Choice of ber_test.recordWithChoice: cannot decode value with tag context-specific:1",
	)

	// large nested constructed encodings that require definite long form for length octets
	var (
		str1 = strings.Repeat("a", 50)
		str2 = strings.Repeat("b", 50)
		str3 = strings.Repeat("c", 50)
	)
	type recordWithSet struct {
		Data ber.SetOf[string]
	}
	expectParses(t, recordWithSet{ber.SetOf[string]{str1, str2, str3}},
		"\x30\x81\x9F"+"\x31\x81\x9C"+("\x04\x32"+str1)+("\x04\x32"+str2)+("\x04\x32"+str3),
	)

	// test coverage for nested error reporting
	type level3 struct {
		Value int
	}
	type level2 struct {
		Children []level3
	}
	type level1 struct {
		Content level2
	}
	expectParses(t, level1{level2{[]level3{{42}}}},
		"\x30\x09"+"\x30\x07"+"\x30\x05"+"\x30\x03"+"\x02\x01\x2A")
	expectParseError[level1](t,
		"\x30\x09"+"\x30\x07"+"\x30\x05"+"\x30\x03"+"\x01\x01\x00",
		`error at byte 10: in field Content of ber_test.level1: in field Children of ber_test.level2: in element 0: in field Value of ber_test.level3: cannot decode value with tag universal:1`,
	)

	// TODO: test coverage for error paths
}

func TestUnmarshalMany(t *testing.T) {
	type record struct {
		ID   int
		Name string
	}
	const (
		msg1 = "\x30\x0A" + "\x02\x01\x01" + "\x04\x05Alice"
		msg2 = "\x30\x08" + "\x02\x01\x02" + "\x04\x03Bob"
		msg3 = "\x30\x0A" + "\x02\x01\x03" + "\x04\x05Carol"
	)

	result, err := ber.UnmarshalMany[record]([]byte(msg1 + msg2 + msg3))
	if assert.ErrEqual(t, err, nil) {
		assert.Equal(t, result, []record{
			{1, "Alice"},
			{2, "Bob"},
			{3, "Carol"},
		})
	}

	_, err = ber.UnmarshalMany[record]([]byte(msg1 + msg2 + msg3)[:len(msg1)+len(msg2)+len(msg3)-1])
	assert.ErrEqual(t, err, `while unmarshaling []ber_test.record[0]: error at byte 24: header with tag universal:16 declares 10 bytes of content, but only 9 bytes remain to be unmarshaled`)
}
