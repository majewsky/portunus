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

package ldap

import (
	"fmt"
	"strings"
	"time"

	"github.com/majewsky/portunus/internal/core"
	"github.com/sapcc/go-bits/logg"
	goldap "gopkg.in/ldap.v3"
)

//Worker contains the configuration for the worker goroutine that reflects
//changes in our internal data model into the LDAP server.
type Worker struct {
	DNSuffix string //e.g. "dc=example,dc=org"
	UserDN   string //e.g. "cn=portunus,dc=example,dc=org"
	Password string //for Portunus' service user
}

//HandleEvents listens on `eventsChan` and reflects all events into the LDAP
//server managed by Portunus. This function does not return.
func (w Worker) HandleEvents(eventsChan <-chan core.Event) {
	//this goroutine is created while slapd is still starting up -> when
	//initially connecting to LDAP, retry up to 10 times with exponential backoff
	//to give slapd enough time to start up
	conn := w.getConn(0, 5*time.Millisecond)
	logg.Info("connected to LDAP server")

	//create main structure: domain-component objects
	suffixRDNs := strings.Split(w.DNSuffix, ",")
	dcName := strings.TrimPrefix(suffixRDNs[0], "dc=")
	err := ldapAdd(conn, w.DNSuffix,
		mkAttr("dc", dcName),
		mkAttr("o", dcName),
		mkAttr("objectClass", "dcObject", "organization", "top"),
	)
	if err != nil {
		logg.Fatal(err.Error())
	}

	//create main structure: organizational units
	for _, ouName := range []string{"users", "groups"} {
		err := ldapAdd(conn, fmt.Sprintf("ou=%s,%s", ouName, w.DNSuffix),
			mkAttr("ou", ouName),
			mkAttr("objectClass", "organizationalUnit", "top"),
		)
		if err != nil {
			logg.Fatal(err.Error())
		}
	}

	//create main structure: service user account
	err = ldapAdd(conn, w.UserDN,
		mkAttr("cn", "portunus"),
		mkAttr("description", "Internal service user for Portunus"),
		mkAttr("objectClass", "organizationalRole", "top"),
	)
	if err != nil {
		logg.Fatal(err.Error())
	}

	//process events (errors here are not fatal anymore; defects in single
	//entries should not compromise the availability of the overall service)
	for event := range eventsChan {
		if len(event.AddedUsers) > 0 {
			logg.Info("adding %d users to the LDAP database", len(event.AddedUsers))
			for _, user := range event.AddedUsers {
				err := ldapAdd(conn, w.getUserDN(user), w.getUserAttributes(user)...)
				if err != nil {
					logg.Error(err.Error())
				}
			}
		}

		if len(event.AddedGroups) > 0 {
			logg.Info("adding %d groups to the LDAP database", len(event.AddedGroups))
			for _, group := range event.AddedGroups {
				err := ldapAdd(conn, w.getGroupDN(group), w.getGroupAttributes(group)...)
				if err != nil {
					logg.Error(err.Error())
				}
			}
		}

		//TODO event.ModifiedUsers
		//TODO event.ModifiedGroups
		//TODO event.DeletedUsers
		//TODO event.DeletedGroups
	}
}

func (w Worker) getConn(retryCounter int, sleepInterval time.Duration) *goldap.Conn {
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

func (w Worker) getUserDN(u core.User) string {
	return fmt.Sprintf("uid=%s,ou=users,%s", u.LoginName, w.DNSuffix)
}
func (w Worker) getGroupDN(g core.Group) string {
	return fmt.Sprintf("cn=%s,ou=groups,%s", g.Name, w.DNSuffix)
}
func (w Worker) getUserAttributes(u core.User) []goldap.Attribute {
	//TODO: make this a posixAccount (requires attributes uidNumber, gidNumber, homeDirectory; optional attributes loginShell, gecos, description)
	return []goldap.Attribute{
		mkAttr("uid", u.LoginName),
		mkAttr("cn", u.GivenName+" "+u.FamilyName), //TODO: allow flipped order (family name first)
		mkAttr("sn", u.FamilyName),
		mkAttr("givenName", u.GivenName),
		mkAttr("objectClass", "inetOrgPerson", "organizationalPerson", "person", "top"),
	}
}
func (w Worker) getGroupAttributes(g core.Group) []goldap.Attribute {
	//TODO: allow making this a posixGroup instead of a groupOfNames (requires gidNumber attribute)
	//NOTE: maybe duplicate posixGroups under a different ou so that we can have both a groupOfNames and a posixGroup for the same core.Group

	memberDNames := make([]string, len(g.MemberLoginNames))
	for idx, name := range g.MemberLoginNames {
		memberDNames[idx] = w.getUserDN(core.User{LoginName: name})
	}

	return []goldap.Attribute{
		mkAttr("cn", g.Name),
		mkAttr("member", memberDNames...),
		mkAttr("objectClass", "groupOfNames", "top"),
	}
}

func mkAttr(typeName string, values ...string) goldap.Attribute {
	return goldap.Attribute{Type: typeName, Vals: values}
}

func ldapAdd(conn *goldap.Conn, dn string, attrs ...goldap.Attribute) error {
	err := conn.Add(&goldap.AddRequest{DN: dn, Attributes: attrs})
	if err == nil {
		return nil
	}
	return fmt.Errorf("could not add object %s to the LDAP database: %s", dn, err.Error())
}
