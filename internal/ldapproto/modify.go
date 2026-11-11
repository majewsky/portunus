// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package ldapproto

import "github.com/majewsky/portunus/internal/ber"

// ModifyRequest represents the ModifyRequest type defined in [RFC 4511, 4.6].
// It is a choice for type [Operation].
type ModifyRequest struct {
	ObjectName DN
	Changes    []Change
}

// Change appears in type [ModifyRequest].
type Change struct {
	Operation    ChangeOperation
	Modification Attribute
}

// ChangeOperation is an enum appearing in type [Change].
type ChangeOperation int

var _ ber.Enum = ChangeOperationAdd

const (
	// ChangeOperationAdd adds values listed to the modification attribute, creating the attribute if necessary.
	ChangeOperationAdd SearchScope = 0
	// ChangeOperationDelete deletes values listed from the modification attribute.
	// If no values are listed, or if all current values of the attribute are listed, the entire attribute is removed.
	ChangeOperationDelete SearchScope = 1
	// ChangeOperationReplace replaces all existing values of the modification attribute with the new values listed,
	// creating the attribute if it did not already exist (or deleting the attribute if no values are listed).
	ChangeOperationReplace SearchScope = 2
)

// ModifyResponse represents the ModifyResponse type defined in [RFC 4511, 4.6].
// It is a choice for type [Operation].
type ModifyResponse Result

// AddRequest represents the AddRequest type defined in [RFC 4511, 4.7].
// It is a choice for type [Operation].
type AddRequest struct {
	ObjectName DN
	Attributes []Attribute
}

// AddResponse represents the ModifyResponse type defined in [RFC 4511, 4.7].
// It is a choice for type [Operation].
type AddResponse Result

// DeleteRequest represents the DeleteRequest type defined in [RFC 4511, 4.8].
// It is a choice for type [Operation].
type DeleteRequest DN

// DeleteResponse represents the ModifyResponse type defined in [RFC 4511, 4.8].
// It is a choice for type [Operation].
type DeleteResponse Result

// ModifyDNRequest represents the ModifyDNRequest type defined in [RFC 4511, 4.9].
// It is a choice for type [Operation].
type ModifyDNRequest struct {
	ObjectName          DN
	NewRelativeDN       RelativeDN
	DeleteOldRelativeDN bool
	NewSuperior         DN `ber:"context-specific:0,optional"`
}

// ModifyDNResponse represents the ModifyResponse type defined in [RFC 4511, 4.9].
// It is a choice for type [Operation].
type ModifyDNResponse Result
