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
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sapcc/go-bits/logg"
)

func main() {
	logg.ShowDebug = true //TODO make configurable
	environment, ids := readConfig()

	//delete leftovers from previous runs
	slapdStatePath := environment["PORTUNUS_SLAPD_STATE_DIR"]
	must(os.RemoveAll(slapdStatePath))

	//setup the slapd directory with the correct permissions
	must(os.Mkdir(slapdStatePath, 0700))
	must(os.Chown(slapdStatePath, ids["PORTUNUS_SLAPD_UID"], ids["PORTUNUS_SLAPD_GID"]))

	slapdDataPath := filepath.Join(slapdStatePath, "data")
	must(os.Mkdir(slapdDataPath, 0770))
	must(os.Chown(slapdDataPath, ids["PORTUNUS_SLAPD_UID"], ids["PORTUNUS_SLAPD_GID"]))

	slapdConfigPath := filepath.Join(slapdStatePath, "slapd.conf")
	must(ioutil.WriteFile(slapdConfigPath, renderSlapdConfig(environment, ids), 0444))

	//setup our state directory with the correct permissions
	statePath := environment["PORTUNUS_SERVER_STATE_DIR"]
	must(os.MkdirAll(statePath, 0770))
	must(os.Chown(statePath, ids["PORTUNUS_SERVER_UID"], ids["PORTUNUS_SERVER_GID"]))

	go runLDAPServer(environment)

	//run portunus-server (thus blocking this goroutine)
	cmd := exec.Command(environment["PORTUNUS_SERVER_BINARY"])
	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORTUNUS_SERVER_UID=%d", ids["PORTUNUS_SERVER_UID"]),
		fmt.Sprintf("PORTUNUS_SERVER_GID=%d", ids["PORTUNUS_SERVER_GID"]),
		"PORTUNUS_LDAP_SUFFIX="+environment["PORTUNUS_LDAP_SUFFIX"],
		"PORTUNUS_LDAP_PASSWORD="+environment["PORTUNUS_LDAP_PASSWORD"],
		"PORTUNUS_SERVER_HTTP_LISTEN="+environment["PORTUNUS_SERVER_HTTP_LISTEN"],
		"PORTUNUS_SERVER_HTTP_SECURE="+environment["PORTUNUS_SERVER_HTTP_SECURE"],
		"PORTUNUS_SERVER_STATE_DIR="+environment["PORTUNUS_SERVER_STATE_DIR"],
	)
	err := cmd.Run()
	if err != nil {
		logg.Fatal("error encountered while running portunus-server: " + err.Error())
	}
}

func must(err error) {
	if err != nil {
		logg.Fatal(err.Error())
	}
}
