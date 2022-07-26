/*******************************************************************************
*
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
*
* This program is free software: you can redistribute it and/or modify it under
* the terms of the GNU General Public License as published by the Free Software
* Foundation, either version 3 of the License, or (at your option) any later
* version.
*
* This program is distributed in the hope that it will be useful, but WITHOUT ANY
* WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR
* A PARTICULAR PURPOSE. See the GNU General Public License for more details.
*
* You should have received a copy of the GNU General Public License along with
* this program. If not, see <http://www.gnu.org/licenses/>.
*
*******************************************************************************/

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/majewsky/portunus/internal/core"
	"github.com/sapcc/go-bits/logg"
	goldap "github.com/go-ldap/ldap/v3"
)

//LDAPWorker performs all the LDAP operations.
type LDAPWorker struct {
	DNSuffix      string //e.g. "dc=example,dc=org"
	UserDN        string //e.g. "cn=portunus,dc=example,dc=org"
	TLSDomainName string
	Password      string //for Portunus' service user
	conn          *goldap.Conn
	objects       map[string]core.LDAPObject //persisted objects, key = object DN
}

func newLDAPWorker() *LDAPWorker {
	//read config from environment (we don't do any further validation here
	//because portunus-orchestrator supplied these values and we trust in the
	//leadership of our glorious orchestrator)
	w := &LDAPWorker{
		DNSuffix:      os.Getenv("PORTUNUS_LDAP_SUFFIX"),
		Password:      os.Getenv("PORTUNUS_LDAP_PASSWORD"),
		TLSDomainName: os.Getenv("PORTUNUS_SLAPD_TLS_DOMAIN_NAME"),
		objects:       make(map[string]core.LDAPObject),
	}
	w.UserDN = "cn=portunus," + w.DNSuffix

	//portunus-server is started in parallel with slapd, and we don't know
	//when slapd is finished -> when initially connecting to LDAP, retry up to 10
	//times with exponential backoff (about 5-6 seconds in total) to give slapd
	//enough time to start up
	w.conn = w.getConn(0, 5*time.Millisecond)
	logg.Info("connected to LDAP server")

	//create main structure: domain-component objects
	suffixRDNs := strings.Split(w.DNSuffix, ",")
	dcName := strings.TrimPrefix(suffixRDNs[0], "dc=")
	err := w.addObject(core.LDAPObject{
		DN: w.DNSuffix,
		Attributes: map[string][]string{
			"dc":          {dcName},
			"o":           {dcName},
			"objectClass": {"dcObject", "organization", "top"},
		},
	})
	if err != nil {
		logg.Fatal(err.Error())
	}

	//create main structure: organizational units
	for _, ouName := range []string{"users", "groups", "posix-groups"} {
		err := w.addObject(core.LDAPObject{
			DN: fmt.Sprintf("ou=%s,%s", ouName, w.DNSuffix),
			Attributes: map[string][]string{
				"ou":          {ouName},
				"objectClass": {"organizationalUnit", "top"},
			},
		})
		if err != nil {
			logg.Fatal(err.Error())
		}
	}

	//create main structure: service user account
	err = w.addObject(core.LDAPObject{
		DN: w.UserDN,
		Attributes: map[string][]string{
			"cn":          {"portunus"},
			"description": {"Internal service user for Portunus"},
			"objectClass": {"organizationalRole", "top"},
		},
	})
	if err != nil {
		logg.Fatal(err.Error())
	}

	//create main structure: dummy user account for empty groups
	err = w.addObject(core.LDAPObject{
		DN: "cn=nobody," + w.DNSuffix,
		Attributes: map[string][]string{
			"cn":          {"nobody"},
			"description": {"Dummy user for empty groups (all groups need to have at least one member)"},
			"objectClass": {"organizationalRole", "top"},
		},
	})
	if err != nil {
		logg.Fatal(err.Error())
	}

	return w
}

//Does not return. Call with `go`.
func (w *LDAPWorker) processEvents(ldapUpdates <-chan []core.LDAPObject) {
	//process events (errors here are not fatal anymore; defects in single
	//entries should not compromise the availability of the overall service)
	for ldapDB := range ldapUpdates {
		isExistingDN := make(map[string]bool)

		for _, newObj := range ldapDB {
			isExistingDN[newObj.DN] = true
			oldObj, exists := w.objects[newObj.DN]
			if exists {
				updated, err := w.modifyObject(newObj.DN, oldObj.Attributes, newObj.Attributes)
				if err == nil {
					if updated {
						logg.Info("LDAP object %s updated", newObj.DN)
					}
				} else {
					logg.Error("cannot update LDAP object %s: %s", newObj.DN, err.Error())
				}
			} else {
				err := w.addObject(newObj)
				if err == nil {
					logg.Info("LDAP object %s created", newObj.DN)
				} else {
					logg.Error("cannot create LDAP object %s: %s", newObj.DN, err.Error())
				}
			}
			w.objects[newObj.DN] = newObj
		}

		for dn := range w.objects {
			if isExistingDN[dn] {
				continue
			}
			err := w.deleteObject(dn)
			if err == nil {
				logg.Info("LDAP object %s deleted", dn)
			} else {
				logg.Error("cannot delete LDAP object %s: %s", dn, err.Error())
			}
			delete(w.objects, dn)
		}
	}
}

func (w LDAPWorker) getConn(retryCounter int, sleepInterval time.Duration) *goldap.Conn {
	if retryCounter == 10 {
		logg.Fatal("giving up on LDAP server after 10 connection attempts")
	}
	time.Sleep(sleepInterval)

	var (
		conn *goldap.Conn
		err  error
	)
	if w.TLSDomainName != "" {
		conn, err = goldap.DialTLS("tcp", w.TLSDomainName+":ldaps", nil)
	} else {
		conn, err = goldap.Dial("tcp", ":ldap")
	}
	if err == nil {
		err = conn.Bind(w.UserDN, w.Password)
	}
	if err != nil {
		logg.Info("cannot connect to LDAP server (attempt %d/10): %s", retryCounter+1, err.Error())
		return w.getConn(retryCounter+1, sleepInterval*2)
	}
	return conn
}

func mkAttr(typeName string, values ...string) goldap.Attribute {
	return goldap.Attribute{Type: typeName, Vals: values}
}

func (w LDAPWorker) addObject(obj core.LDAPObject) error {
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
	return w.conn.Add(&req)
}

func (w LDAPWorker) deleteObject(dn string) error {
	return w.conn.Del(&goldap.DelRequest{DN: dn})
}

func (w LDAPWorker) modifyObject(dn string, oldAttrs, newAttrs map[string][]string) (updated bool, err error) {
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
		return false, nil
	}
	return true, w.conn.Modify(&req)
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
