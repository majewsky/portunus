// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package grammars_test

import (
	"cmp"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/majewsky/portunus/internal/ldapproto"
	"github.com/majewsky/portunus/internal/ldapproto/grammars"
	"go.xyrillian.de/gg/assert"
)

// shorthands for use in the testcase constructions
func attr(typeStr, value string) grammars.Attribute {
	if strings.Contains(typeStr, ".") {
		return grammars.Attribute{Type: typeStr, RawValue: []byte(value)}
	} else {
		return grammars.Attribute{Type: typeStr, EncodedValue: value}
	}
}
func rdn(attrs ...grammars.Attribute) grammars.RelativeDN {
	return grammars.RelativeDN(attrs)
}

var dnTestCases = []struct {
	Raw        ldapproto.DN
	Structured grammars.DN
	Normalized ldapproto.DN // contains expected output of Structured.String() if different from Raw
	Error      any
}{
	// test the examples from [RFC 4514, section 4]
	{
		Raw: `UID=jsmith,DC=example,DC=net`,
		Structured: grammars.DN{
			rdn(attr("UID", "jsmith")),
			rdn(attr("DC", "example")),
			rdn(attr("DC", "net")),
		},
	},
	{
		Raw: `OU=Sales+CN=J.  Smith,DC=example,DC=net`,
		Structured: grammars.DN{
			rdn(attr("OU", "Sales"), attr("CN", "J.  Smith")),
			rdn(attr("DC", "example")),
			rdn(attr("DC", "net")),
		},
	},
	{
		Raw: `CN=James \"Jim\" Smith\, III,DC=example,DC=net`,
		Structured: grammars.DN{
			rdn(attr("CN", `James "Jim" Smith, III`)),
			rdn(attr("DC", "example")),
			rdn(attr("DC", "net")),
		},
	},
	{
		Raw: `CN=Before\0dAfter,DC=example,DC=net`,
		Structured: grammars.DN{
			rdn(attr("CN", "Before\rAfter")),
			rdn(attr("DC", "example")),
			rdn(attr("DC", "net")),
		},
	},
	{
		Raw: `1.3.6.1.4.1.1466.0=#04024869`,
		Structured: grammars.DN{
			rdn(attr("1.3.6.1.4.1.1466.0", "\x04\x02\x48\x69")),
		},
	},
	{
		Raw: `CN=Lu\C4\8Di\C4\87`,
		Structured: grammars.DN{
			rdn(attr("CN", "Lučić")),
		},
		Normalized: `CN=Lučić`,
	},
	// our own test cases (for coverage)
	{
		Raw:        "",
		Structured: nil,
	},
	{
		Raw:   "example.org",
		Error: `at "example.org": expected "=" after <descr> in <attributeType>, but found '.'`,
	},
	// previous fuzzing failures
	{
		Raw:   "=0",
		Error: `at "=0": <descr> in <attributeType> starts with forbidden character '='`,
	},
	{
		Raw: "A=",
		Structured: grammars.DN{
			rdn(attr("A", "")),
		},
	},
	{
		Raw:   "A=\x00",
		Error: `at "\x00": expected "," or end of string, but got '\x00'`,
	},
	{
		Raw:   "000.0=",
		Error: `at "000.0=": <oid> in <attributeType> contains a <number> starting with a superfluous '0'`,
	},
	{
		Raw:   "0=",
		Error: `at "0=": <oid> in <attributeType> does not contain required character '.'`,
	},
	{
		Raw:   "0.0=#",
		Error: `at "#": <hexstring> in <attributeValue> has length 0`,
	},
	{
		Raw:   "1.3.6.1.4.1.6.0=#01466.0=#04024869",
		Error: `at "#01466.0=#04024869": <hexstring> in <attributeValue> is of uneven length (5)`,
	},
	{
		Raw:   "0..0=#00",
		Error: `at "0..0=#00": <oid> in <attributeType> contains invalid sequence ".."`,
	},
	{
		Raw: "100.0=#00",
		Structured: grammars.DN{
			rdn(attr("100.0", "\x00")),
		},
	},
}

func TestDN(t *testing.T) {
	for _, tc := range dnTestCases {
		t.Run(string(tc.Raw), func(t *testing.T) {
			actual, err := grammars.ParseDN(tc.Raw)
			if !assert.ErrEqual(t, err, tc.Error) {
				t.FailNow()
			}
			if tc.Error == nil {
				assert.Equal(t, actual, tc.Structured)
				assert.Equal(t, actual.String(), cmp.Or(tc.Normalized, tc.Raw))
			}
		})
	}
}

func FuzzDNAcceptance(f *testing.F) {
	for _, tc := range dnTestCases {
		f.Add(string(tc.Raw))
	}

	// ground truth: a regex built from the ABNF in [RFC 4514, section 3]
	// (each variable is named after the respective ABNF production)
	var (
		hexpairPattern    = `[0-9A-Fa-f][0-9A-Fa-f]`
		hexstringPattern  = fmt.Sprintf(`#(?:%s)+`, hexpairPattern)
		specialPattern    = `["+,;<> #=]`
		pairPattern       = fmt.Sprintf(`\\(?:\\|%s|%s)`, specialPattern, hexpairPattern)
		stringcharPattern = `[^\x00\x22\x2B\x2C\x3B\x3C\x3E\x5C]`
		trailcharPattern  = `[^\x00\x20\x22\x2B\x2C\x3B\x3C\x3E\x5C]`
		leadcharPattern   = `[^\x00\x20\x22\x23\x2B\x2C\x3B\x3C\x3E\x5C]`
		stringPattern     = fmt.Sprintf(`(?:(?:%[1]s|%[4]s)(?:(?:%[2]s|%[4]s)*(?:%[3]s|%[4]s))?)?`, leadcharPattern, stringcharPattern, trailcharPattern, pairPattern)

		numberPattern     = `(?:[0-9]|[1-9][0-9]+)`
		numericoidPattern = fmt.Sprintf(`%[1]s(?:\.%[1]s)+`, numberPattern)
		keystringPattern  = `[A-Za-z][A-Za-z0-9-]*`

		// variable names abbreviated here for readability (atav = attributeTypeAndValue)
		atavPattern = fmt.Sprintf(`(?:%[1]s=%[2]s|%[3]s=%[4]s)`, numericoidPattern, hexstringPattern, keystringPattern, stringPattern)
		rdnPattern  = fmt.Sprintf(`%[1]s(?:\+%[1]s)*`, atavPattern)
		dnPattern   = fmt.Sprintf(`%[1]s(?:,%[1]s)*`, rdnPattern)
		rx          = regexp.MustCompile(fmt.Sprintf(`^(%s)?$`, dnPattern))
	)

	f.Fuzz(func(t *testing.T, raw string) {
		if !utf8.ValidString(raw) {
			return // not interesting
		}

		// check that `raw` only parses if and only if the regexp also accepts it
		dn, err := grammars.ParseDN(ldapproto.DN(raw))
		matches := rx.MatchString(raw)
		t.Logf("input = %q", raw)
		if matches && err != nil {
			t.Fatalf("regexp matches, but ParseDN says: %s", err.Error())
		} else if !matches && err == nil {
			t.Fatalf("regexp does not match, but ParseDN produces: %#v", dn)
		}

		if !matches {
			return // subsequent test only applies to DNs that parse successfully
		}

		// for coverage of DN.String(), we cannot just check if dn.String() == raw, because this roundtrip is not idempotent;
		// however, parsing the output of dn.String() produces a DN that ought to roundtrip
		raw2 := dn.String()
		dn2, err := grammars.ParseDN(raw2)
		if !assert.ErrEqual(t, err, nil) {
			assert.Equal(t, dn2.String(), raw2)
		}
	})
}
