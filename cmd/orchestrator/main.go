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
	"github.com/tredoe/osutil/file"
)

func main() {
	environment, ids := readConfig()
	logg.ShowDebug = environment["PORTUNUS_DEBUG"] == "true"

	//delete leftovers from previous runs
	slapdStatePath := environment["PORTUNUS_SLAPD_STATE_DIR"]
	must(os.RemoveAll(slapdStatePath))

	//setup the slapd directory with the correct permissions
	must(os.Mkdir(slapdStatePath, 0700))
	must(os.Chown(slapdStatePath, ids["PORTUNUS_SLAPD_UID"], ids["PORTUNUS_SLAPD_GID"]))

	slapdDataPath := filepath.Join(slapdStatePath, "data")
	must(os.Mkdir(slapdDataPath, 0770))
	must(os.Chown(slapdDataPath, ids["PORTUNUS_SLAPD_UID"], ids["PORTUNUS_SLAPD_GID"]))

	customSchemaPath := filepath.Join(environment["PORTUNUS_SLAPD_STATE_DIR"], "portunus.schema")
	must(ioutil.WriteFile(customSchemaPath, []byte(customSchema), 0444))

	slapdConfigPath := filepath.Join(slapdStatePath, "slapd.conf")
	must(ioutil.WriteFile(slapdConfigPath, renderSlapdConfig(environment, ids), 0444))

	//copy TLS cert and private key into a location where slapd can definitely read it
	if certPath := environment["PORTUNUS_SLAPD_TLS_CERTIFICATE"]; certPath != "" {
		certPath2 := filepath.Join(environment["PORTUNUS_SLAPD_STATE_DIR"], "cert.pem")
		must(file.Copy(certPath, certPath2))
		must(os.Chown(certPath2, ids["PORTUNUS_SLAPD_UID"], ids["PORTUNUS_SLAPD_GID"]))

		keyPath := environment["PORTUNUS_SLAPD_TLS_PRIVATE_KEY"]
		keyPath2 := filepath.Join(environment["PORTUNUS_SLAPD_STATE_DIR"], "key.pem")
		must(file.Copy(keyPath, keyPath2))
		must(os.Chown(keyPath2, ids["PORTUNUS_SLAPD_UID"], ids["PORTUNUS_SLAPD_GID"]))

		caPath := environment["PORTUNUS_SLAPD_TLS_CA_CERTIFICATE"]
		caPath2 := filepath.Join(environment["PORTUNUS_SLAPD_STATE_DIR"], "ca.pem")
		must(file.Copy(caPath, caPath2))
		must(os.Chown(caPath2, ids["PORTUNUS_SLAPD_UID"], ids["PORTUNUS_SLAPD_GID"]))
	}

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
		"PORTUNUS_DEBUG="+environment["PORTUNUS_DEBUG"],
		"PORTUNUS_LDAP_SUFFIX="+environment["PORTUNUS_LDAP_SUFFIX"],
		"PORTUNUS_LDAP_PASSWORD="+environment["PORTUNUS_LDAP_PASSWORD"],
		"PORTUNUS_SERVER_HTTP_LISTEN="+environment["PORTUNUS_SERVER_HTTP_LISTEN"],
		"PORTUNUS_SERVER_HTTP_SECURE="+environment["PORTUNUS_SERVER_HTTP_SECURE"],
		"PORTUNUS_SERVER_STATE_DIR="+environment["PORTUNUS_SERVER_STATE_DIR"],
		"PORTUNUS_SLAPD_TLS_DOMAIN_NAME="+environment["PORTUNUS_SLAPD_TLS_DOMAIN_NAME"],
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
