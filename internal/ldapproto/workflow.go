// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package ldapproto

// AbandonRequest represents the AbandonRequest type defined in [RFC 4511, 4.11].
// It is a choice for type [Operation].
//
// The contained integer value is the MessageID of the request that shall be abandoned.
type AbandonRequest int

// ExtendedRequest represents the ExtendedRequest type defined in [RFC 4511, 4.12].
// It is a choice for type [Operation].
type ExtendedRequest struct {
	RequestName  OID    `ber:"context-specific:0"`
	RequestValue []byte `ber:"context-specific:1,optional"`
}

// ExtendedResponse represents the ExtendedResponse type defined in [RFC 4511, 4.12].
// It is a choice for type [Operation].
type ExtendedResponse struct {
	Result
	RequestName  OID    `ber:"context-specific:10"`
	RequestValue []byte `ber:"context-specific:11,optional"`
}

// IntermediateResponse represents the IntermediateResponse type defined in [RFC 4511, 4.13].
// It is a choice for type [Operation].
type IntermediateResponse struct {
	RequestName  OID    `ber:"context-specific:0"`
	RequestValue []byte `ber:"context-specific:1,optional"`
}
