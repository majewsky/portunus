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
	"github.com/majewsky/portunus/internal/core"
)

//RunServer runs slapd and updates its database whenever an event is received.
//This function does not return.
func RunServer(eventsChan <-chan core.Event) {
	//branch off the goroutine that translates the incoming events into LDAP commands
	worker := Worker{
		DNSuffix: suffix,
		UserDN:   userDN,
		Password: password,
	}
	go worker.HandleEvents(eventsChan)
}
