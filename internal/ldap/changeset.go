/*******************************************************************************
* Copyright 2019-2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package ldap

import (
	goldap "github.com/go-ldap/ldap/v3"
	"github.com/majewsky/portunus/internal/core"
)

// A sum type of all possible requests that we can send to the server.
type operation struct {
	//Exactly one of these must be non-nil.
	AddRequest    *goldap.AddRequest
	ModifyRequest *goldap.ModifyRequest
	DeleteRequest *goldap.DelRequest
}

// ExecuteOn dispatches into the respective method call on the `conn` interface.
func (op operation) ExecuteOn(conn Connection) error {
	switch {
	case op.AddRequest != nil:
		return conn.Add(*op.AddRequest)
	case op.ModifyRequest != nil:
		return conn.Modify(*op.ModifyRequest)
	case op.DeleteRequest != nil:
		return conn.Delete(*op.DeleteRequest)
	default:
		panic("operation had no non-nil member field!")
	}
}

// Computes a minimal changeset (i.e. a set of LDAP write operations) by
// diffing two sets of LDAP objects.
func computeUpdates(oldObjects, newObjects []core.LDAPObject, operations chan<- operation) {
	oldObjectsByDN := make(map[string]core.LDAPObject, len(oldObjects))
	for _, oldObj := range oldObjects {
		oldObjectsByDN[oldObj.DN] = oldObj
	}

	isExistingDN := make(map[string]bool)
	for _, newObj := range newObjects {
		isExistingDN[newObj.DN] = true
		oldObj, exists := oldObjectsByDN[newObj.DN]
		if exists {
			buildModifyRequest(newObj.DN, oldObj.Attributes, newObj.Attributes, operations)
		} else {
			buildAddRequest(newObj, operations)
		}
	}

	for _, oldObj := range oldObjects {
		if !isExistingDN[oldObj.DN] {
			req := goldap.DelRequest{DN: oldObj.DN}
			operations <- operation{DeleteRequest: &req}
		}
	}
}

func buildAddRequest(obj core.LDAPObject, operations chan<- operation) {
	req := goldap.AddRequest{
		DN:         obj.DN,
		Attributes: make([]goldap.Attribute, 0, len(obj.Attributes)),
	}
	for key, values := range obj.Attributes {
		if len(values) > 0 {
			attr := goldap.Attribute{Type: key, Vals: values}
			req.Attributes = append(req.Attributes, attr)
		}
	}
	operations <- operation{AddRequest: &req}
}

func buildModifyRequest(dn string, oldAttrs, newAttrs map[string][]string, operations chan<- operation) {
	req := goldap.ModifyRequest{DN: dn}
	keepAttribute := make(map[string]bool, len(newAttrs))

	for key, newValues := range newAttrs {
		keepAttribute[key] = true
		oldValues := oldAttrs[key]
		if !stringListsAreEqual(oldValues, newValues) {
			req.Replace(key, newValues)
		}
	}

	for key := range oldAttrs {
		if !keepAttribute[key] {
			req.Delete(key, nil)
		}
	}

	if len(req.Changes) == 0 {
		return
	}
	operations <- operation{ModifyRequest: &req}
}

func stringListsAreEqual(lhs, rhs []string) bool {
	if len(lhs) != len(rhs) {
		return false
	}
	for idx, left := range lhs {
		right := rhs[idx]
		if left != right {
			return false
		}
	}
	return true
}
