/*******************************************************************************
* Copyright 2019-2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package ldap

import goldap "github.com/go-ldap/ldap/v3"

// A sum type of all possible requests that we can send to the server.
type operation struct {
	// Exactly one of these must be non-nil.
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
func computeUpdates(oldObjects, newObjects []Object) (result []operation) {
	oldObjectsByDN := make(map[string]Object, len(oldObjects))
	for _, oldObj := range oldObjects {
		oldObjectsByDN[oldObj.DN] = oldObj
	}

	isExistingDN := make(map[string]bool)
	for _, newObj := range newObjects {
		isExistingDN[newObj.DN] = true
		oldObj, exists := oldObjectsByDN[newObj.DN]
		if exists {
			result = append(result, buildModifyRequest(newObj.DN, oldObj.Attributes, newObj.Attributes)...)
		} else {
			result = append(result, buildAddRequest(newObj))
		}
	}

	for _, oldObj := range oldObjects {
		if !isExistingDN[oldObj.DN] {
			req := goldap.DelRequest{DN: oldObj.DN}
			result = append(result, operation{DeleteRequest: &req})
		}
	}

	return result
}

func buildAddRequest(obj Object) operation {
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
	return operation{AddRequest: &req}
}

func buildModifyRequest(dn string, oldAttrs, newAttrs map[string][]string) []operation {
	req := goldap.ModifyRequest{DN: dn}
	keepAttribute := make(map[string]bool, len(newAttrs))

	for key, newValues := range newAttrs {
		if len(newValues) == 0 {
			continue
		}
		keepAttribute[key] = true
		oldValues := oldAttrs[key]
		if !stringListsAreEqual(oldValues, newValues) {
			req.Replace(key, newValues)
		}
	}

	for key, oldValues := range oldAttrs {
		if len(oldValues) == 0 {
			continue
		}
		if !keepAttribute[key] {
			req.Delete(key, nil)
		}
	}

	if len(req.Changes) == 0 {
		return nil
	}
	return []operation{{ModifyRequest: &req}}
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
