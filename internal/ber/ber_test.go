// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package ber_test

import (
	"fmt"
	"testing"

	"go.xyrillian.de/gg/assert"

	"github.com/majewsky/portunus/internal/ber"
	"github.com/majewsky/portunus/internal/ldapproto"
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

func TestPrimitiveValues(t *testing.T) {
	// boolean [X.690, 8.2] [X.690, 11.1]
	expectParses(t, false, "\x01\x01\x00")
	expectParses(t, true, "\x01\x01\xFF", "\x01\x01\x01")
	expectParseError[bool](t, "\x01\x00",
		"error at byte 2: cannot decode boolean value from 0 bytes of content")
	expectParseError[bool](t, "\x01\x02\x00\x00",
		"error at byte 4: cannot decode boolean value from 2 bytes of content")

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

	// octet string [X.690, 8.7]
	expectParses(t, "", "\x04\x00")
	expectParses(t, "Hello", "\x04\x05Hello")

	// null [X.690, 8.8]
	expectParses(t, struct{}{}, "\x05\x00")
	expectParseError[struct{}](t, "\x05\x01\x00",
		"error at byte 2: expected 0 bytes, but got 1 bytes of content for NULL value")

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

func TestLDAPProtocolMessages(t *testing.T) {
	// This sequence of messages is a full request-response sequence for the command
	// `ldapsearch -H ldap://localhost:389 -D uid=admin,ou=users,dc=example,dc=com -W -b dc=example,dc=com '(objectclass=person)'`
	// on a minimal dev instance.
	expectParses(t,
		ldapproto.Message{
			MessageID: 1,
			Operation: ldapproto.BindRequest{
				Version:        3,
				Name:           "uid=admin,ou=users,dc=example,dc=com",
				Authentication: ldapproto.SimpleAuthentication("1234"),
			},
		},
		"04\x02\x01\x01`/\x02\x01\x03\x04$uid=admin,ou=users,dc=example,dc=com\x80\x041234",
	)
	expectParses(t,
		ldapproto.Message{
			MessageID: 1,
			Operation: ldapproto.BindResponse{
				Result: ldapproto.Result{
					ResultCode: ldapproto.ResultCodeSuccess,
				},
			},
		},
		"0\f\x02\x01\x01a\a\n\x01\x00\x04\x00\x04\x00",
	)
	expectParses(t,
		ldapproto.Message{
			MessageID: 2,
			Operation: ldapproto.SearchRequest{
				BaseObject:   "dc=example,dc=com",
				Scope:        ldapproto.SearchScopeWholeSubtree,
				DerefAliases: ldapproto.NeverDerefAliases,
				SizeLimit:    0,
				TimeLimit:    0,
				TypesOnly:    false,
				Filter: ldapproto.EqualityMatchFilter{
					AttributeDescription: "objectclass",
					AssertionValue:       "person",
				},
				Attributes: nil,
			},
		},
		"0@\x02\x01\x02c;\x04\x11dc=example,dc=com\n\x01\x02\n\x01\x00\x02\x01\x00\x02\x01\x00\x01\x01\x00\xa3\x15\x04\vobjectclass\x04\x06person0\x00",
	)
	expectParses(t,
		ldapproto.Message{
			MessageID: 2,
			Operation: ldapproto.SearchResultEntry{
				ObjectName: "uid=admin,ou=users,dc=example,dc=com",
				Attributes: []ldapproto.Attribute{
					{Type: "cn", Values: []string{"Initial Administrator"}},
					{Type: "sn", Values: []string{"Administrator"}},
					{Type: "givenName", Values: []string{"Initial"}},
					// NOTE: this does not leak a real password, the hashed value is just "1234"
					{Type: "userPassword", Values: []string{"{CRYPT}$y$j9T$eq1Ue0u3wc35iB.Doc/Mo/$B65m87aCD8P9JOD9Pox.mCXPGxgESmZ.ks2WZYxyvv."}},
					{Type: "isMemberOf", Values: []string{"cn=admins,ou=groups,dc=example,dc=com"}},
					{Type: "objectClass", Values: []string{"portunusPerson", "inetOrgPerson", "organizationalPerson", "person", "top"}},
					{Type: "uid", Values: []string{"admin"}},
				},
			},
		},
		"0\x82\x01}\x02\x01\x02d\x82\x01v\x04$uid=admin,ou=users,dc=example,dc=com0\x82\x01L0\x1d\x04\x02cn1\x17\x04\x15Initial Administrator0\x15\x04\x02sn1\x0f\x04\rAdministrator0\x16\x04\tgivenName1\t\x04\aInitial0b\x04\fuserPassword1R\x04P{CRYPT}$y$j9T$eq1Ue0u3wc35iB.Doc/Mo/$B65m87aCD8P9JOD9Pox.mCXPGxgESmZ.ks2WZYxyvv.05\x04\nisMemberOf1'\x04%cn=admins,ou=groups,dc=example,dc=com0Q\x04\vobjectClass1B\x04\x0eportunusPerson\x04\rinetOrgPerson\x04\x14organizationalPerson\x04\x06person\x04\x03top0\x0e\x04\x03uid1\a\x04\x05admin",
	)
	expectParses(t,
		ldapproto.Message{
			MessageID: 2,
			Operation: ldapproto.SearchResultDone{
				ResultCode: ldapproto.ResultCodeSuccess,
			},
		},
		"0\f\x02\x01\x02e\a\n\x01\x00\x04\x00\x04\x00",
	)
	expectParses(t,
		ldapproto.Message{
			MessageID: 3,
			Operation: ldapproto.UnbindRequest{},
		},
		"0\x05\x02\x01\x03B\x00",
	)

	// As a special case, this is the same SearchRequest as before, but with the filter wrapped in an additional NOT
	// (i.e. filtering for `(!(objectclass=person))` instead of `(objectclass=person`).
	// This checks the special encoding for a Filter instantiated as a NotFilter containing another Filter.
	expectParses(t,
		ldapproto.Message{
			MessageID: 2,
			Operation: ldapproto.SearchRequest{
				BaseObject:   "dc=example,dc=com",
				Scope:        ldapproto.SearchScopeWholeSubtree,
				DerefAliases: ldapproto.NeverDerefAliases,
				SizeLimit:    0,
				TimeLimit:    0,
				TypesOnly:    false,
				Filter: ldapproto.NotFilter{
					Inner: ldapproto.EqualityMatchFilter{
						AttributeDescription: "objectclass",
						AssertionValue:       "person",
					},
				},
				Attributes: nil,
			},
		},
		"0B\x02\x01\x02c=\x04\x11dc=example,dc=com\n\x01\x02\n\x01\x00\x02\x01\x00\x02\x01\x00\x01\x01\x00\xa2\x17\xa3\x15\x04\vobjectclass\x04\x06person0\x00",
	)
}
