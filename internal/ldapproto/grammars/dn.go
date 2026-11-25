// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package grammars

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	. "go.xyrillian.de/gg/option"

	"github.com/majewsky/portunus/internal/ldapproto"
)

// DN is a structured representation for a Distinguished Name [RFC 4514, section 2.1].
// Its constituent RDNs appear in the slice in serialization order.
type DN []RelativeDN

// RelativeDN is a structured representation for a Relative Distinguished Name [RFC 4514, section 2.2].
// Its constituent attributes appear in the slice in serialization order.
type RelativeDN []Attribute

// Attribute is a single pair of attribute type and value that appears in [RelativeDN].
type Attribute struct {
	Type ldapproto.String

	// EncodedValue contains the string representation of the [ldapproto.AttributeValue],
	// according to whatever string encoding rule is specified for this attribute type,
	// cf. e.g. [RFC 4517, section 3.3] for descriptions of common attribute types.
	//
	// It is only filled if the attribute value was represented as a <string>
	// (i.e. if the attribute type is given as its descriptor instead of an OID).
	EncodedValue string

	// RawValue is used instead of EncodedValue if the attribute value was represented as a <hexstring>
	// (i.e. if the attribute type is given as an OID instead of a descriptor).
	RawValue []byte
}

////////////////////////////////////////////////////////////////////////////////
// serialization

// String returns the string representation of a [DN].
func (dn DN) String() ldapproto.DN {
	var b strings.Builder
	for idx1, rdn := range dn {
		if idx1 > 0 {
			b.WriteByte(',')
		}
		for idx2, attr := range rdn {
			if idx2 > 0 {
				b.WriteByte('+')
			}
			attr.serializeTo(&b)
		}
	}
	return ldapproto.DN(b.String())
}

func (attr Attribute) serializeTo(b *strings.Builder) {
	// both OIDs and descriptors serialize without modification
	b.WriteString(attr.Type)
	b.WriteByte('=')

	// we have either an EncodedValue or a RawValue or neither; only the first case is tedious, so handle the others first
	if len(attr.RawValue) > 0 {
		b.WriteByte('#')
		hex.NewEncoder(b).Write(attr.RawValue) //nolint:errcheck // both b.Write() and hex.Encoder never produce errors
		return
	}
	if attr.EncodedValue == "" {
		return
	}

	// find the index of the last rune, we need this later to decide which characters must be escaped
	_, size := utf8.DecodeLastRuneInString(attr.EncodedValue)
	lastIndex := len(attr.EncodedValue) - size

	for idx, r := range attr.EncodedValue {
		if unicode.Is(unicode.C, r) {
			// ^ NOTE: [RFC 4514, section 2.4] only demands that we escape '\000' as `\00`.
			//         Escaping all other control characters, too, is a personal aesthetic choice.
			var buf [4]byte
			for idx2 := range utf8.EncodeRune(buf[:], r) {
				fmt.Fprintf(b, "\\%02x", buf[idx2])
			}
			continue
		}
		var needsEscaping bool
		switch r {
		case ' ':
			needsEscaping = idx == 0 || idx == lastIndex
		case '#':
			needsEscaping = idx == 0
		case '"', '+', ',', ';', '<', '>', '\\':
			needsEscaping = true
		}
		if needsEscaping {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
}

////////////////////////////////////////////////////////////////////////////////
// parsing

// ParseDN parses the string representation of a [DN].
func ParseDN(input ldapproto.DN) (DN, error) {
	if input == "" {
		// empty DNs are explicitly allowed (used in SearchRequest to query for the root DSE)
		return nil, nil
	}
	if !utf8.ValidString(string(input)) {
		return nil, errors.New("not a valid UTF-8 string")
	}
	return parseDN(string(input))
}

func parseDN(input string) (result DN, _ error) {
	rdn, remainder, err := parseRDN(input)
	if err != nil {
		return nil, err
	}
	result = append(result, rdn)
	for {
		var ok bool
		remainder, ok = strings.CutPrefix(remainder, ",")
		if !ok {
			break
		}
		rdn, remainder, err = parseRDN(remainder)
		if err != nil {
			return nil, err
		}
		result = append(result, rdn)
	}
	if remainder != "" {
		r, _ := utf8.DecodeRuneInString(remainder)
		return nil, fmt.Errorf("at %q: expected %q or end of string, but got %q", remainder, ",", r)
	}
	return result, nil
}

func parseRDN(input string) (result RelativeDN, remainder string, _ error) {
	attr, remainder, err := parseAttribute(input)
	if err != nil {
		return nil, "", err
	}
	result = append(result, attr)
	for {
		var ok bool
		remainder, ok = strings.CutPrefix(remainder, "+")
		if !ok {
			break
		}
		attr, remainder, err = parseAttribute(remainder)
		if err != nil {
			return nil, "", err
		}
		result = append(result, attr)
	}
	return result, remainder, nil
}

func parseAttribute(input string) (parsed Attribute, remainder string, _ error) {
	if input == "" {
		return Attribute{}, "", fmt.Errorf("at %q: missing <attributeTypeAndValue>", input)
	}
	r, _ := utf8.DecodeRuneInString(input)
	if isDigitChar(r) {
		parsedType, remainder, err := parseOID(input)
		if err != nil {
			return Attribute{}, "", err
		}
		remainder, err = consumeSeparator(remainder, "<oid> in <attributeType>", "=")
		if err != nil {
			return Attribute{}, "", fmt.Errorf("at %q: %w", input, err)
		}
		parsedValue, remainder, err := parseHexstring(remainder)
		if err != nil {
			return Attribute{}, "", err
		}
		return Attribute{Type: parsedType, RawValue: parsedValue}, remainder, nil
	} else {
		parsedType, remainder, err := parseDescriptor(input)
		if err != nil {
			return Attribute{}, "", err
		}
		remainder, err = consumeSeparator(remainder, "<descr> in <attributeType>", "=")
		if err != nil {
			return Attribute{}, "", fmt.Errorf("at %q: %w", input, err)
		}
		parsedValue, remainder, err := parseString(remainder)
		if err != nil {
			return Attribute{}, "", err
		}
		return Attribute{Type: parsedType, EncodedValue: parsedValue}, remainder, nil
	}
}

func parseOID(input string) (parsed ldapproto.String, remainder string, _ error) {
	var accepted int
	for accepted < len(input) {
		r, size := utf8.DecodeRuneInString(input[accepted:])
		if isDigitChar(r) || r == '.' {
			accepted += size
		} else {
			break
		}
	}
	parsed, remainder = input[0:accepted], input[accepted:]
	switch {
	case !strings.Contains(parsed, "."):
		return "", "", fmt.Errorf("at %q: <oid> in <attributeType> does not contain required character %q", input, '.')
	case strings.HasPrefix(parsed, "."):
		// defense in depth: caller has already checked `isDigitChar(r)`
		return "", "", fmt.Errorf("at %q: <oid> in <attributeType> starts with forbidden character %q", input, '.')
	case strings.HasSuffix(parsed, "."):
		return "", "", fmt.Errorf("at %q: <oid> in <attributeType> ends with forbidden character %q", input, '.')
	}
	for num := range strings.SplitSeq(parsed, ".") {
		if num == "" {
			return "", "", fmt.Errorf("at %q: <oid> in <attributeType> contains invalid sequence %q", input, "..")
		}
		if num != "0" && num[0] == '0' {
			return "", "", fmt.Errorf("at %q: <oid> in <attributeType> contains a <number> starting with a superfluous %q", input, '0')
		}
	}
	return
}

func parseDescriptor(input string) (parsed ldapproto.String, remainder string, _ error) {
	var accepted int
LOOP:
	for accepted < len(input) {
		r, size := utf8.DecodeRuneInString(input[accepted:])
		switch {
		case accepted == 0:
			if isAlphaChar(r) {
				accepted += size
			} else {
				return "", "", fmt.Errorf("at %q: <descr> in <attributeType> starts with forbidden character %q", input, r)
			}
		case isAlphaChar(r) || isDigitChar(r) || r == '-':
			accepted += size
		default:
			break LOOP
		}
	}
	return input[0:accepted], input[accepted:], nil
}

func parseString(input string) (parsed, remainder string, _ error) {
	var (
		lastUnquotedRune Option[rune]
		accepted         int
		b                strings.Builder
	)

LOOP:
	for accepted < len(input) {
		r, size := utf8.DecodeRuneInString(input[accepted:])
		switch r {
		case ' ', '#':
			if accepted == 0 {
				return "", "", fmt.Errorf("at %q: <string> in <attributeValue> starts with unescaped %q", input, r)
			}
		case '\x00', '"', '+', ',', ';', '<', '>':
			break LOOP
		}
		accepted += size
		if r == '\\' {
			parsedByte, length, err := parseEscaped(input[accepted:])
			if err != nil {
				return "", "", fmt.Errorf("at %q: %w", input, err)
			}
			accepted += length
			_ = b.WriteByte(parsedByte)
			lastUnquotedRune = None[rune]()
		} else {
			_, _ = b.WriteRune(r)
			lastUnquotedRune = Some(r)
		}
	}

	if lastUnquotedRune == Some(' ') {
		return "", "", fmt.Errorf("at %q: <string> in <attributeValue> ends with unescaped %q", input, ' ')
	}
	return b.String(), input[accepted:], nil
}

func parseEscaped(input string) (parsed byte, accepted int, _ error) {
	// NOTE: The initial '\\' was already accepted at this point. `input` starts right beyond it.
	if input == "" {
		return 0, 0, fmt.Errorf("<string> in <attributeValue> contains incomplete escape sequence %q", "\\")
	}
	r, size1 := utf8.DecodeRuneInString(input)
	switch {
	case isEscapableChar(r):
		return byte(r), size1, nil
	case !isHexChar(r):
		return 0, 0, fmt.Errorf("<string> in <attributeValue> contains malformed escape sequence %q", "\\"+input[0:size1])
	}

	if len(input) == size1 {
		return 0, 0, fmt.Errorf("<string> in <attributeValue> contains incomplete escape sequence %q", "\\"+input)
	}
	r, size2 := utf8.DecodeRuneInString(input[size1:])
	if !isHexChar(r) {
		return 0, 0, fmt.Errorf("<string> in <attributeValue> contains malformed escape sequence %q", "\\"+input[0:size1+size2])
	}
	decoded, err := hex.DecodeString(input[0 : size1+size2])
	if err != nil {
		return 0, 0, err // defense in depth: unreachable because of all the checks before
	}
	return decoded[0], size1 + size2, nil
}

func parseHexstring(fullInput string) (parsed []byte, remainder string, _ error) {
	input, ok := strings.CutPrefix(fullInput, "#")
	if !ok {
		return nil, "", fmt.Errorf("at %q: <hexstring> in <attributeValue> starts with %q", fullInput, '#')
	}

	var accepted int
	for accepted < len(input) {
		r, size := utf8.DecodeRuneInString(input[accepted:])
		if isHexChar(r) {
			accepted += size
		} else {
			break
		}
	}
	if accepted == 0 {
		return nil, "", fmt.Errorf("at %q: <hexstring> in <attributeValue> has length 0", fullInput)
	}
	if accepted%2 != 0 {
		return nil, "", fmt.Errorf("at %q: <hexstring> in <attributeValue> is of uneven length (%d)", fullInput, accepted)
	}
	parsed, _ = hex.DecodeString(input[0:accepted])
	return parsed, input[accepted:], nil
}

func consumeSeparator(input, prevProduction, separator string) (remainder string, _ error) {
	if input == "" {
		return "", fmt.Errorf("missing %q after %s", separator, prevProduction)
	}
	remainder, ok := strings.CutPrefix(input, "=")
	if ok {
		return remainder, nil
	} else {
		r, _ := utf8.DecodeRuneInString(input)
		return "", fmt.Errorf("expected %q after %s, but found %q", separator, prevProduction, r)
	}
}

func isEscapableChar(r rune) bool {
	switch r {
	case ' ', '"', '#', '+', ',', ';', '<', '=', '>', '\\':
		return true
	default:
		return false
	}
}

func isHexChar(r rune) bool {
	switch {
	case r >= '0' && r <= '9':
		return true
	case r >= 'A' && r <= 'F':
		return true
	case r >= 'a' && r <= 'f':
		return true
	default:
		return false
	}
}

func isDigitChar(r rune) bool {
	return r >= '0' && r <= '9'
}

func isAlphaChar(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}
