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
	"os"
	"os/exec"

	"github.com/majewsky/portunus/internal/core"
	"github.com/sapcc/go-bits/logg"
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

	//run slapd
	cmd := exec.Command(core.Getenv("PORTUNUS_SLAPD_BINARY").Or("slapd"),
		"-h", "ldap:///",
		"-f", configPath,
		"-d", "0", //no debug logging (but still important because presence of `-d` keeps slapd from daemonizing)
	)
	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		logg.Error("error encountered while running slapd: " + err.Error())
		logg.Info("Since slapd logs to syslog only, check there for more information.")
		os.Exit(1)
	}
}
