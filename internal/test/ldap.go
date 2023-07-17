/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package test

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	goldap "github.com/go-ldap/ldap/v3"
)

// LDAPConnectionDouble is a test double for the ldap.Connection interface.
// It will only accept requests that are sent while a call to its Expect()
// method is in progress.
type LDAPConnectionDouble struct {
	dnSuffix               string
	expectedAddRequests    []goldap.AddRequest
	expectedModifyRequests []goldap.ModifyRequest
	expectedDeleteRequests []goldap.DelRequest
}

// NewLDAPConnectionDouble builds an LDAPConnectionDouble.
func NewLDAPConnectionDouble(dnSuffix string) *LDAPConnectionDouble {
	return &LDAPConnectionDouble{dnSuffix: dnSuffix}
}

// DNSuffix implements the ldap.Connection interface.
func (d *LDAPConnectionDouble) DNSuffix() string {
	return d.dnSuffix
}

// Add implements the ldap.Connection interface.
func (d *LDAPConnectionDouble) Add(req goldap.AddRequest) error {
	return removeIfExpected[goldap.AddRequest](&d.expectedAddRequests, normalizeAddRequest(req))
}

// Modify implements the ldap.Connection interface.
func (d *LDAPConnectionDouble) Modify(req goldap.ModifyRequest) error {
	return removeIfExpected[goldap.ModifyRequest](&d.expectedModifyRequests, req)
}

// Delete implements the ldap.Connection interface.
func (d *LDAPConnectionDouble) Delete(req goldap.DelRequest) error {
	return removeIfExpected[goldap.DelRequest](&d.expectedDeleteRequests, req)
}

func removeIfExpected[R any](pool *[]R, req R) error {
	for idx := range *pool {
		if reflect.DeepEqual((*pool)[idx], req) {
			//this request was expected - remove it from the pool of expected requests
			*pool = append(append([]R(nil), (*pool)[0:idx]...), (*pool)[idx+1:]...)
			return nil
		}
	}
	return fmt.Errorf("unexpected LDAP request:\n\t%#v", req)
}

// ExpectAdd records that we expect an AddRequest to be executed via this
// double after this call returns.
func (d *LDAPConnectionDouble) ExpectAdd(req goldap.AddRequest) {
	d.expectedAddRequests = append(d.expectedAddRequests, normalizeAddRequest(req))
}

// ExpectModify records that we expect an ModifyRequest to be executed via this
// double after this call returns.
func (d *LDAPConnectionDouble) ExpectModify(req goldap.ModifyRequest) {
	d.expectedModifyRequests = append(d.expectedModifyRequests, req)
}

// ExpectDelete records that we expect an DeleteRequest to be executed via this
// double after this call returns.
func (d *LDAPConnectionDouble) ExpectDelete(req goldap.DelRequest) {
	d.expectedDeleteRequests = append(d.expectedDeleteRequests, req)
}

// CheckAllExecuted fails the test if any of the expected requests that were
// enqueued with ExpectAdd, ExpectModify or ExpectDelete were not sent before
// this call.
func (d *LDAPConnectionDouble) CheckAllExecuted(t *testing.T) {
	t.Helper()
	for _, req := range d.expectedAddRequests {
		t.Errorf("did not observe as expected:\n\t%#v", req)
	}
	d.expectedAddRequests = nil
	for _, req := range d.expectedModifyRequests {
		t.Errorf("did not observe as expected:\n\t%#v", req)
	}
	d.expectedModifyRequests = nil
	for _, req := range d.expectedDeleteRequests {
		t.Errorf("did not observe as expected:\n\t%#v", req)
	}
	d.expectedDeleteRequests = nil
}

func normalizeAddRequest(req goldap.AddRequest) goldap.AddRequest {
	//normalize the request to enable deterministic matching
	attrs := req.Attributes
	sort.Slice(attrs, func(i, j int) bool { return attrs[i].Type < attrs[j].Type })
	return req
}
