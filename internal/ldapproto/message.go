// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package ldapproto

import "github.com/majewsky/portunus/internal/ber"

// Message contains a full LDAP message, corresponding to the LDAPMessage type defined in [RFC 4511, 4.1.1].
type Message struct {
	MessageID int
	Operation ber.ChoiceOf[Operation]
	Controls  []Control `ber:"context-specific:0,optional"`
}

// Operation appears in type [Message].
type Operation struct {
	BindRequest   `ber:"application:0"`
	BindResponse  `ber:"application:1"`
	UnbindRequest `ber:"application:2"`

	SearchRequest         `ber:"application:3"`
	SearchResultEntry     `ber:"application:4"`
	SearchResultDone      `ber:"application:5"`
	SearchResultReference `ber:"application:19"`
}

var (
	_ ber.ChoiceOf[Operation] = BindRequest{}
	_ ber.ChoiceOf[Operation] = BindResponse{}
	_ ber.ChoiceOf[Operation] = UnbindRequest{}
	_ ber.ChoiceOf[Operation] = SearchRequest{}
	_ ber.ChoiceOf[Operation] = SearchResultEntry{}
	_ ber.ChoiceOf[Operation] = SearchResultDone{}
	_ ber.ChoiceOf[Operation] = SearchResultReference{}
)

// Into implements the [ber.ChoiceOf] interface.
func (r BindRequest) Into() Operation { return Operation{BindRequest: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r BindResponse) Into() Operation { return Operation{BindResponse: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r UnbindRequest) Into() Operation { return Operation{UnbindRequest: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r SearchRequest) Into() Operation { return Operation{SearchRequest: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r SearchResultEntry) Into() Operation { return Operation{SearchResultEntry: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r SearchResultDone) Into() Operation { return Operation{SearchResultDone: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r SearchResultReference) Into() Operation { return Operation{SearchResultReference: r} }

// Control represents the Control type defined in [RFC 4511, 4.1.11].
// It appears in type [Message].
type Control struct {
	ControlType  OID
	Criticiality bool   `ber:",optional"` // NOTE: actually declared with "DEFAULT FALSE"
	ControlValue string `ber:",optional"`
}
