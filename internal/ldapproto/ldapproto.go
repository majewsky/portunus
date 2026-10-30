// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

// Package ldapproto contains type declarations for the messages defined in LDAP [RFC 4511].
// Full LDAP messages map onto type [Message].
package ldapproto

import "github.com/majewsky/portunus/internal/ber"

// TODO: replace string with []byte where the spec does not demand UTF-8
// TODO: Validate() everywhere, but remove enforcement during unmarshaling (some validations require specific ResultCode responses)

type (
	// String represents the LDAPSTRING type defined in [RFC 4511, 4.1.2].
	String string

	// OID represents the LDAPOID type defined in [RFC 4511, 4.1.2].
	OID string

	// DN represents the LDAPDN type defined in [RFC 4511, 4.1.3].
	DN string

	// RelativeDN represents the LDAPDN type defined in [RFC 4511, 4.1.3].
	RelativeDN string

	// AttributeDescription represents the AttributeDescription type defined in [RFC 4511, 4.1.6].
	AttributeDescription string

	// AssertionValue represents the AssertionValue type defined in [RFC 4511, 4.1.6].
	AssertionValue string

	// MatchingRuleID represents the MatchingRuleId type defined in [RFC 4511, 4.1.6].
	MatchingRuleID string

	// URI represents the URI type defined in [RFC 4511, 4.1.10].
	URI string
)

// AttributeValueAssertion represents the AttributeValueAssertion type defined in [RFC 4511, 4.1.6].
// It appears in several variants of type [Filter] and in type [CompareRequest].
type AttributeValueAssertion struct {
	AttributeDescription AttributeDescription
	AssertionValue       AssertionValue
}

// Attribute represents both the PartialAttribute and Attribute types defined in [RFC 4511, 4.1.7].
// It appears in types [AddRequest], [Change] and [SearchResultEntry].
type Attribute struct {
	Type   String
	Values ber.SetOf[string]
}
