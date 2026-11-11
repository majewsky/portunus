// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

// Package ldapproto contains type declarations for the messages defined in LDAP [RFC 4511].
// Full LDAP messages map onto type [Message].
//
// As a general note, OCTET STRING and other ASN.1 types derived from it are mapped to the Go type []byte by default,
// and to the Go type string only if the relevant specifications explicitly restrict valid values to UTF-8 or ASCII.
package ldapproto

import "github.com/majewsky/portunus/internal/ber"

// TODO: Validate() everywhere, but remove enforcement during unmarshaling (some validations require specific ResultCode responses)
// TODO: allow catching open-ended choices to handle them in a special way (e.g. AuthenticationChoice requires an authMethodNotSupported response)

type (
	// String represents the LDAPSTRING type defined in [RFC 4511, section 4.1.2].
	String string

	// OID represents the LDAPOID type defined in [RFC 4511, section 4.1.2],
	// corresponding to the <numericoid> production of [RFC 4512].
	OID string

	// DN represents the LDAPDN type defined in [RFC 4511, section 4.1.3],
	// corresponding to the <distinguishedName> production of [RFC 4514].
	DN string

	// RelativeDN represents the RelativeLDAPDN type defined in [RFC 4511, section 4.1.3],
	// corresponding to the <name-component> production of [RFC 4514].
	RelativeDN string

	// AttributeDescription represents the AttributeDescription type defined in [RFC 4511, section 4.1.4],
	// corresponding to the <attributedescription> production of [RFC 4512].
	AttributeDescription string

	// AttributeValue represents the AttributeValue type defined in [RFC 4511, section 4.1.5].
	AttributeValue []byte

	// AssertionValue represents the AssertionValue type defined in [RFC 4511, section 4.1.6].
	AssertionValue []byte

	// MatchingRuleID represents the MatchingRuleId type defined in [RFC 4511, section 4.1.8].
	MatchingRuleID string

	// URI represents the URI type defined in [RFC 4511, section 4.1.10].
	URI string

	// AttributeSelector represents elements of the AttributeSelection type defined in [RFC 4511, section 4.5.1],
	// corresponding to the <attributeSelector> production in [RFC 4511, section 4.5.1.8].
	AttributeSelector string
)

// AttributeValueAssertion represents the AttributeValueAssertion type defined in [RFC 4511, section 4.1.6].
// It appears in several variants of type [Filter] and in type [CompareRequest].
type AttributeValueAssertion struct {
	AttributeDescription AttributeDescription
	AssertionValue       AssertionValue
}

// Attribute represents both the PartialAttribute and Attribute types defined in [RFC 4511, section 4.1.7].
// It appears in types [AddRequest], [Change] and [SearchResultEntry].
type Attribute struct {
	Type   String
	Values ber.SetOf[AttributeValue]
}
