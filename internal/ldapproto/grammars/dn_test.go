// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package grammars_test

import (
	"fmt"
	"regexp"
	"testing"
	"unicode/utf8"

	"github.com/majewsky/portunus/internal/ldapproto"
	"github.com/majewsky/portunus/internal/ldapproto/grammars"
	"go.xyrillian.de/gg/assert"
)

func TestDN(t *testing.T) {
	// shorthands for use in the testcase constructions
	attr := func(typeStr, valueStr string) grammars.Attribute {
		return grammars.Attribute{typeStr, ldapproto.AttributeValue(valueStr)}
	}
	rdn := func(attrs ...grammars.Attribute) grammars.RelativeDN {
		return grammars.RelativeDN(attrs)
	}

	testCases := []struct {
		Raw        ldapproto.DN
		Structured grammars.DN
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
		},
		// our own test cases
		{
			Raw:        "",
			Structured: nil,
		},
		{
			Raw:   "example.org",
			Error: `malformed relativeDistinguishedName "example.org": malformed attributeTypeAndValue "example.org": not of the form "type=value"`,
		},
	}

	for _, tc := range testCases {
		t.Run(string(tc.Raw), func(t *testing.T) {
			actual, err := grammars.ParseDN(tc.Raw)
			if !assert.ErrEqual(t, err, tc.Error) {
				t.FailNow()
			}
			if tc.Error == nil {
				assert.Equal(t, actual, tc.Structured)
				// TODO: test roundtripping once DN.String() implemented
			}
		})
	}
}

func FuzzDNAcceptance(f *testing.F) {
	// TODO: use the full list of testcases from TestDN as seed corpus
	f.Add(`UID=jsmith,DC=example,DC=net`)
	f.Add(`OU=Sales+CN=J.  Smith,DC=example,DC=net`)
	f.Add(`CN=James \"Jim\" Smith\, III,DC=example,DC=net`)
	f.Add(`CN=Before\0dAfter,DC=example,DC=net`)
	f.Add(`1.3.6.1.4.1.1466.0=#04024869`)
	f.Add(`CN=Lu\C4\8Di\C4\87`)

	// ground truth: a regex built from the ABNF in [RFC 4514, section 3]
	// (each variable is named after the respective ABNF production)
	var (
		hexpairPattern        = `[0-9A-Fa-f][0-9A-Fa-f]`
		hexstringPattern      = fmt.Sprintf(`#(?:%s)+`, hexpairPattern)
		specialPattern        = `["+,;<> #=]`
		pairPattern           = fmt.Sprintf(`\\(?:\\|%s|%s)`, specialPattern, hexpairPattern)
		stringcharPattern     = `[^\x00\x22\x2B\x2C\x3B\x3C\x3E\x5C]`
		trailcharPattern      = `[^\x00\x20\x22\x2B\x2C\x3B\x3C\x3E\x5C]`
		leadcharPattern       = `[^\x00\x20\x22\x23\x2B\x2C\x3B\x3C\x3E\x5C]`
		stringPattern         = fmt.Sprintf(`(?:%[1]s|%[4]s)(?:(?:%[2]s|%[4]s)*(?:%[3]s|%[4]s))?`, leadcharPattern, stringcharPattern, trailcharPattern, pairPattern)
		attributeValuePattern = fmt.Sprintf(`(?:%s|%s)`, stringPattern, hexstringPattern)

		numberPattern        = `(?:[0-9]|[0-9][1-9]+)`
		numericoidPattern    = fmt.Sprintf(`%[1]s(?:\.%[1]s)+`, numberPattern)
		keystringPattern     = `[A-Za-z][A-Za-z0-9-]*`
		attributeTypePattern = fmt.Sprintf(`(?:%s|%s)`, keystringPattern, numericoidPattern)

		// variable names abbreviated here for readability
		rdnPattern = fmt.Sprintf(`%[1]s=%[2]s(?:\+%[1]s=%[2]s)*`, attributeTypePattern, attributeValuePattern)
		dnPattern  = fmt.Sprintf(`%[1]s(?:,%[1]s)*`, rdnPattern)
		rx         = regexp.MustCompile(fmt.Sprintf(`^%s$`, dnPattern))
	)

	f.Fuzz(func(t *testing.T, raw string) {
		if !utf8.ValidString(raw) {
			return // not interesting
		}
		dn, err := grammars.ParseDN(ldapproto.DN(raw))
		matches := rx.MatchString(raw)
		t.Logf("input = %q", raw)
		if matches && err != nil {
			t.Fatalf("regexp matches, but ParseDN says: %s", err.Error())
		} else if !matches && err == nil {
			t.Fatalf("regexp does not match, but ParseDN produces: %#v", dn)
		}
	})
}
