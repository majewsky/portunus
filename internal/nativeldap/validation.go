// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package nativeldap

import (
	"fmt"

	"github.com/majewsky/portunus/internal/ldapproto"
	. "go.xyrillian.de/gg/option"
)

func ValidateRequest(m ldapproto.Message) Option[ldapproto.Result] {
	if result := validateMessageID(m.MessageID); result.IsSome() {
		return result
	}

	for _, ctl := range m.Controls {
		if ctl.Criticality {
			return Some(ldapproto.ResultCodeUnavailableCriticalExtension.With("control %q is not supported", ctl.ControlType))
		}
	}

	switch op := m.Operation.(type) {
	case ldapproto.BindRequest:
		panic("TODO")
	case ldapproto.UnbindRequest:
		panic("TODO")
	case ldapproto.SearchRequest:
		panic("TODO")
	case ldapproto.CompareRequest:
		panic("TODO")
	case ldapproto.AbandonRequest:
		panic("TODO")
	case ldapproto.ModifyRequest, ldapproto.AddRequest, ldapproto.DeleteRequest, ldapproto.ModifyDNRequest:
		return Some(ldapproto.ResultCodeInsufficientAccessRights.With("forbidden"))
	case ldapproto.ExtendedRequest:
		return Some(ldapproto.ResultCodeProtocolError.With("extended operation %q is not supported", op.RequestName))
	default:
		return Some(ldapproto.ResultCodeOperationsError.With("%T is not a supported request type", op))
	}
}

func validateMessageID(id int) Option[ldapproto.Result] {
	if id == 0 {
		return Some(ldapproto.ResultCodeProtocolError.With("message ID may not be 0"))
	}
	return None[ldapproto.Result]()
}

func validateSearchScope(s ldapproto.SearchScope) error {
	switch s {
	case ldapproto.SearchScopeBaseObject, ldapproto.SearchScopeSingleLevel, ldapproto.SearchScopeWholeSubtree:
		return nil
	default:
		return fmt.Errorf("unacceptable value for SearchScope: %d", s)
	}
}

func validateDerefAliasesStrategy(s ldapproto.DerefAliasesStrategy) error {
	switch s {
	case ldapproto.NeverDerefAliases, ldapproto.DerefAliasesInSearching, ldapproto.DerefAliasesFindingBaseObject, ldapproto.DerefAliasesAlways:
		return nil
	default:
		return fmt.Errorf("unacceptable value for DerefAliasesStrategy: %d", s)
	}
}
