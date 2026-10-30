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
	// see bind.go
	BindRequest   `ber:"application:0"`
	BindResponse  `ber:"application:1"`
	UnbindRequest `ber:"application:2"`

	// see search.go
	SearchRequest         `ber:"application:3"`
	SearchResultEntry     `ber:"application:4"`
	SearchResultDone      `ber:"application:5"`
	SearchResultReference `ber:"application:19"`

	// see modify.go
	ModifyRequest    `ber:"application:6"`
	ModifyResponse   `ber:"application:7"`
	AddRequest       `ber:"application:8"`
	AddResponse      `ber:"application:9"`
	DeleteRequest    `ber:"application:10"`
	DeleteResponse   `ber:"application:11"`
	ModifyDNRequest  `ber:"application:12"`
	ModifyDNResponse `ber:"application:13"`

	// see compare.go
	CompareRequest  `ber:"application:14"`
	CompareResponse `ber:"application:15"`

	// see workflow.go
	AbandonRequest       `ber:"application:16"`
	ExtendedRequest      `ber:"application:23"`
	ExtendedResponse     `ber:"application:24"`
	IntermediateResponse `ber:"application:25"`
}

var (
	_ ber.ChoiceOf[Operation] = BindRequest{}
	_ ber.ChoiceOf[Operation] = BindResponse{}
	_ ber.ChoiceOf[Operation] = UnbindRequest{}
	_ ber.ChoiceOf[Operation] = SearchRequest{}
	_ ber.ChoiceOf[Operation] = SearchResultEntry{}
	_ ber.ChoiceOf[Operation] = SearchResultDone{}
	_ ber.ChoiceOf[Operation] = SearchResultReference{}
	_ ber.ChoiceOf[Operation] = ModifyRequest{}
	_ ber.ChoiceOf[Operation] = ModifyResponse{}
	_ ber.ChoiceOf[Operation] = AddRequest{}
	_ ber.ChoiceOf[Operation] = AddResponse{}
	_ ber.ChoiceOf[Operation] = DeleteRequest("")
	_ ber.ChoiceOf[Operation] = DeleteResponse{}
	_ ber.ChoiceOf[Operation] = ModifyDNRequest{}
	_ ber.ChoiceOf[Operation] = ModifyDNResponse{}
	_ ber.ChoiceOf[Operation] = CompareRequest{}
	_ ber.ChoiceOf[Operation] = CompareResponse{}
	_ ber.ChoiceOf[Operation] = AbandonRequest(0)
	_ ber.ChoiceOf[Operation] = ExtendedRequest{}
	_ ber.ChoiceOf[Operation] = ExtendedResponse{}
	_ ber.ChoiceOf[Operation] = IntermediateResponse{}
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

// Into implements the [ber.ChoiceOf] interface.
func (r ModifyRequest) Into() Operation { return Operation{ModifyRequest: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r ModifyResponse) Into() Operation { return Operation{ModifyResponse: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r AddRequest) Into() Operation { return Operation{AddRequest: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r AddResponse) Into() Operation { return Operation{AddResponse: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r DeleteRequest) Into() Operation { return Operation{DeleteRequest: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r DeleteResponse) Into() Operation { return Operation{DeleteResponse: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r ModifyDNRequest) Into() Operation { return Operation{ModifyDNRequest: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r ModifyDNResponse) Into() Operation { return Operation{ModifyDNResponse: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r CompareRequest) Into() Operation { return Operation{CompareRequest: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r CompareResponse) Into() Operation { return Operation{CompareResponse: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r AbandonRequest) Into() Operation { return Operation{AbandonRequest: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r ExtendedRequest) Into() Operation { return Operation{ExtendedRequest: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r ExtendedResponse) Into() Operation { return Operation{ExtendedResponse: r} }

// Into implements the [ber.ChoiceOf] interface.
func (r IntermediateResponse) Into() Operation { return Operation{IntermediateResponse: r} }

// Control represents the Control type defined in [RFC 4511, 4.1.11].
// It appears in type [Message].
type Control struct {
	ControlType  OID
	Criticiality bool   `ber:",optional"` // NOTE: actually declared with "DEFAULT FALSE"
	ControlValue string `ber:",optional"`
}
