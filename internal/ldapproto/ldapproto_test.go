// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package ldapproto_test

import (
	"testing"

	"github.com/majewsky/portunus/internal/ber"
	"github.com/majewsky/portunus/internal/ldapproto"
	"go.xyrillian.de/gg/assert"
)

const (
	// This sequence of messages is a full request-response sequence for the command
	// `ldapsearch -H ldap://localhost:389 -D uid=admin,ou=users,dc=example,dc=com -W -b dc=example,dc=com '(objectclass=person)'`
	// on a minimal dev instance.
	msg1 = "04\x02\x01\x01`/\x02\x01\x03\x04$uid=admin,ou=users,dc=example,dc=com\x80\x041234"
	msg2 = "0\f\x02\x01\x01a\a\n\x01\x00\x04\x00\x04\x00"
	msg3 = "0@\x02\x01\x02c;\x04\x11dc=example,dc=com\n\x01\x02\n\x01\x00\x02\x01\x00\x02\x01\x00\x01\x01\x00\xa3\x15\x04\vobjectclass\x04\x06person0\x00"
	msg4 = "0\x82\x01}\x02\x01\x02d\x82\x01v\x04$uid=admin,ou=users,dc=example,dc=com0\x82\x01L0\x1d\x04\x02cn1\x17\x04\x15Initial Administrator0\x15\x04\x02sn1\x0f\x04\rAdministrator0\x16\x04\tgivenName1\t\x04\aInitial0b\x04\fuserPassword1R\x04P{CRYPT}$y$j9T$eq1Ue0u3wc35iB.Doc/Mo/$B65m87aCD8P9JOD9Pox.mCXPGxgESmZ.ks2WZYxyvv.05\x04\nisMemberOf1'\x04%cn=admins,ou=groups,dc=example,dc=com0Q\x04\vobjectClass1B\x04\x0eportunusPerson\x04\rinetOrgPerson\x04\x14organizationalPerson\x04\x06person\x04\x03top0\x0e\x04\x03uid1\a\x04\x05admin"
	msg5 = "0\f\x02\x01\x02e\a\n\x01\x00\x04\x00\x04\x00"
	msg6 = "0\x05\x02\x01\x03B\x00"

	// As a special case, this is the same SearchRequest as before, but with the filter wrapped in an additional NOT
	// (i.e. filtering for `(!(objectclass=person))` instead of `(objectclass=person)`.
	// This checks the special encoding for a Filter instantiated as a NotFilter containing another Filter.
	msg3Negated = "0B\x02\x01\x02c=\x04\x11dc=example,dc=com\n\x01\x02\n\x01\x00\x02\x01\x00\x02\x01\x00\x01\x01\x00\xa2\x17\xa3\x15\x04\vobjectclass\x04\x06person0\x00"
)

func expectParses(t *testing.T, input string, expected ldapproto.Message) {
	t.Helper()

	actual, err := ber.Unmarshal[ldapproto.Message]([]byte(input))
	if assert.ErrEqual(t, err, nil) {
		assert.Equal(t, actual, expected)
	}

	buf, err := ber.Marshal(expected)
	if assert.ErrEqual(t, err, nil) {
		assert.Equal(t, string(buf), input)
	}
}

func TestLDAPProtocolMessages(t *testing.T) {
	expectParses(t, msg1, ldapproto.Message{
		MessageID: 1,
		Operation: ldapproto.BindRequest{
			Version:        3,
			Name:           "uid=admin,ou=users,dc=example,dc=com",
			Authentication: ldapproto.SimpleAuthentication("1234"),
		},
	})
	expectParses(t, msg2, ldapproto.Message{
		MessageID: 1,
		Operation: ldapproto.BindResponse{
			Result: ldapproto.Result{
				ResultCode: ldapproto.ResultCodeSuccess,
			},
		},
	})
	expectParses(t, msg3, ldapproto.Message{
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
				AssertionValue:       ldapproto.AssertionValue("person"),
			},
			Attributes: nil,
		},
	})
	expectParses(t, msg4, ldapproto.Message{
		MessageID: 2,
		Operation: ldapproto.SearchResultEntry{
			ObjectName: "uid=admin,ou=users,dc=example,dc=com",
			Attributes: []ldapproto.Attribute{
				{Type: "cn", Values: attributeValues("Initial Administrator")},
				{Type: "sn", Values: attributeValues("Administrator")},
				{Type: "givenName", Values: attributeValues("Initial")},
				// NOTE: this does not leak a real password, the hashed value is just "1234"
				{Type: "userPassword", Values: attributeValues("{CRYPT}$y$j9T$eq1Ue0u3wc35iB.Doc/Mo/$B65m87aCD8P9JOD9Pox.mCXPGxgESmZ.ks2WZYxyvv.")},
				{Type: "isMemberOf", Values: attributeValues("cn=admins,ou=groups,dc=example,dc=com")},
				{Type: "objectClass", Values: attributeValues("portunusPerson", "inetOrgPerson", "organizationalPerson", "person", "top")},
				{Type: "uid", Values: attributeValues("admin")},
			},
		},
	})
	expectParses(t, msg5, ldapproto.Message{
		MessageID: 2,
		Operation: ldapproto.SearchResultDone{
			ResultCode: ldapproto.ResultCodeSuccess,
		},
	})
	expectParses(t, msg6, ldapproto.Message{
		MessageID: 3,
		Operation: ldapproto.UnbindRequest{},
	})

	expectParses(t, msg3Negated, ldapproto.Message{
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
					AssertionValue:       ldapproto.AssertionValue("person"),
				},
			},
			Attributes: nil,
		},
	})
}

func attributeValues(inputs ...string) (result ber.SetOf[ldapproto.AttributeValue]) {
	for _, input := range inputs {
		result = append(result, ldapproto.AttributeValue(input))
	}
	return
}

func FuzzUnmarshaling(f *testing.F) {
	f.Add([]byte(msg1))
	f.Add([]byte(msg2))
	f.Add([]byte(msg3))
	f.Add([]byte(msg3Negated))
	f.Add([]byte(msg4))
	f.Add([]byte(msg5))
	f.Add([]byte(msg6))
	f.Fuzz(func(t *testing.T, buf []byte) {
		_, _ = ber.Unmarshal[ldapproto.Message](buf)
	})
}
