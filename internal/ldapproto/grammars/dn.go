// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package grammars

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

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
	Type  ldapproto.String
	Value ldapproto.AttributeValue
}

// ParseDN parses the string representation of a [DN].
func ParseDN(input ldapproto.DN) (DN, error) {
	if input == "" {
		// empty DNs are explicitly allowed (used in SearchRequest to query for the root DSE)
		return nil, nil
	}
	if !utf8.ValidString(string(input)) {
		return nil, errors.New("not a valid UTF-8 string")
	}
	var result DN
	for raw := range strings.SplitSeq(string(input), ",") {
		rdn, err := parseRDN(raw)
		if err != nil {
			return nil, fmt.Errorf("malformed relativeDistinguishedName %q: %w", raw, err)
		}
		result = append(result, rdn)
	}
	return result, nil
}

func parseRDN(input string) (RelativeDN, error) {
	if input == "" {
		return nil, errors.New("RDN may not be empty")
	}

	var result RelativeDN
	for raw := range strings.SplitSeq(string(input), "+") {
		rdn, err := parseRDNAttribute(raw)
		if err != nil {
			return nil, fmt.Errorf("malformed attributeTypeAndValue %q: %w", raw, err)
		}
		result = append(result, rdn)
	}
	return result, nil
}

func parseRDNAttribute(input string) (Attribute, error) {
	// TODO: incomplete, we will build the fuzzer first
	left, right, ok := strings.Cut(input, "=")
	if !ok {
		return Attribute{}, errors.New(`not of the form "type=value"`)
	}
	return Attribute{Type: left, Value: ldapproto.AttributeValue(right)}, nil
}
