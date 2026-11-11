// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package ldapproto

import "github.com/majewsky/portunus/internal/ber"

// BindRequest represents the BindRequest type defined in [RFC 4511, 4.2].
// It is a choice for type [Operation].
type BindRequest struct {
	Version        uint8
	Name           DN
	Authentication ber.ChoiceOf[AuthenticationChoice]
}

// AuthenticationChoice appears in type [BindRequest].
type AuthenticationChoice struct {
	SimpleAuthentication `ber:"context-specific:0"`
	SASLAuthentication   `ber:"context-specific:3"`
}

var (
	_ ber.ChoiceOf[AuthenticationChoice] = SimpleAuthentication("")
	_ ber.ChoiceOf[AuthenticationChoice] = SASLAuthentication{}
)

// Into implements the [ber.ChoiceOf] interface.
func (a SimpleAuthentication) Into() AuthenticationChoice {
	return AuthenticationChoice{SimpleAuthentication: a}
}

// Into implements the [ber.ChoiceOf] interface.
func (a SASLAuthentication) Into() AuthenticationChoice {
	return AuthenticationChoice{SASLAuthentication: a}
}

// SimpleAuthentication is a choice for type [AuthenticationChoice].
type SimpleAuthentication string

// SASLAuthentication is a choice for type [AuthenticationChoice].
type SASLAuthentication struct {
	Mechanism   String
	Credentials string `ber:",optional"`
}

// BindResponse represents the BindResponse type defined in [RFC 4511, 4.2].
// It is a choice for type [Operation].
type BindResponse struct {
	Result                       // COMPONENTS OF...
	ServerSASLCredentials string `ber:"context-specific:7,optional"`
}

// UnbindRequest represents the UnbindRequest type defined in [RFC 4511, 4.2].
// It is a choice for type [Operation].
type UnbindRequest struct{}
