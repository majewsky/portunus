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
	goldap "gopkg.in/ldap.v3"
)

//LDAPWorker performs all the LDAP operations.
type LDAPWorker struct {
	DNSuffix string //e.g. "dc=example,dc=org"
	UserDN   string //e.g. "cn=portunus,dc=example,dc=org"
	Password string //for Portunus' service user
	conn     *goldap.Conn
}

func newLDAPWorker() *LDAPWorker {
	//read config from environment (we don't do any further validation here
	//because portunus-orchestrator supplied these values and we trust in the
	//leadership of our glorious orchestrator)
	w := &LDAPWorker{
		DNSuffix: os.Getenv("PORTUNUS_LDAP_SUFFIX"),
		Password: os.Getenv("PORTUNUS_LDAP_PASSWORD"),
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
	err := w.add(w.DNSuffix,
		mkAttr("dc", dcName),
		mkAttr("o", dcName),
		mkAttr("objectClass", "dcObject", "organization", "top"),
	)
	if err != nil {
		logg.Fatal(err.Error())
	}

	//create main structure: organizational units
	for _, ouName := range []string{"users", "groups"} {
		err := w.add(fmt.Sprintf("ou=%s,%s", ouName, w.DNSuffix),
			mkAttr("ou", ouName),
			mkAttr("objectClass", "organizationalUnit", "top"),
		)
		if err != nil {
			logg.Fatal(err.Error())
		}
	}

	//create main structure: service user account
	err = w.add(w.UserDN,
		mkAttr("cn", "portunus"),
		mkAttr("description", "Internal service user for Portunus"),
		mkAttr("objectClass", "organizationalRole", "top"),
	)
	if err != nil {
		logg.Fatal(err.Error())
	}

	return w
}

//Does not return. Call with `go`.
func (w *LDAPWorker) processEvents(events <-chan core.Event) {
	//process events (errors here are not fatal anymore; defects in single
	//entries should not compromise the availability of the overall service)
	for event := range events {
		if len(event.Added) > 0 {
			logg.Info("adding %d entities to the LDAP database", len(event.Added))
			for _, entity := range event.Added {
				addReq := entity.RenderToLDAP(w.DNSuffix)
				err := w.add(addReq.DN, addReq.Attributes...)
				if err != nil {
					logg.Error(err.Error())
				}
			}
		}

		//TODO event.Modified
		//TODO event.Deleted
	}
}

func (w LDAPWorker) getConn(retryCounter int, sleepInterval time.Duration) *goldap.Conn {
	if retryCounter == 10 {
		logg.Fatal("giving up on LDAP server after 10 connection attempts")
	}
	time.Sleep(sleepInterval)

	conn, err := goldap.Dial("tcp", ":ldap")
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

func (w LDAPWorker) add(dn string, attrs ...goldap.Attribute) error {
	err := w.conn.Add(&goldap.AddRequest{DN: dn, Attributes: attrs})
	if err == nil {
		return nil
	}
	return fmt.Errorf("could not add object %s to the LDAP database: %s", dn, err.Error())
}
