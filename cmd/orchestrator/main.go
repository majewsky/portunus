/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/majewsky/portunus/internal/crypt"
	"github.com/sapcc/go-bits/logg"
	"github.com/sapcc/go-bits/must"
)

func main() {
	environment, ids := readConfig()
	logg.ShowDebug = environment["PORTUNUS_DEBUG"] == "true"
	hasher := must.Return(crypt.NewPasswordHasher())

	//delete leftovers from previous runs
	slapdStatePath := environment["PORTUNUS_SLAPD_STATE_DIR"]
	must.Succeed(os.RemoveAll(slapdStatePath))

	//setup the slapd directory with the correct permissions
	must.Succeed(os.Mkdir(slapdStatePath, 0700))
	must.Succeed(os.Chown(slapdStatePath, ids["PORTUNUS_SLAPD_UID"], ids["PORTUNUS_SLAPD_GID"]))

	slapdDataPath := filepath.Join(slapdStatePath, "data")
	must.Succeed(os.Mkdir(slapdDataPath, 0770))
	must.Succeed(os.Chown(slapdDataPath, ids["PORTUNUS_SLAPD_UID"], ids["PORTUNUS_SLAPD_GID"]))

	customSchemaPath := filepath.Join(environment["PORTUNUS_SLAPD_STATE_DIR"], "portunus.schema")
	must.Succeed(os.WriteFile(customSchemaPath, []byte(customSchema), 0444))

	slapdConfigPath := filepath.Join(slapdStatePath, "slapd.conf")
	must.Succeed(os.WriteFile(slapdConfigPath, renderSlapdConfig(environment, hasher), 0444))

	//copy TLS cert and private key into a location where slapd can definitely read it
	if environment["PORTUNUS_SLAPD_TLS_CERTIFICATE"] != "" {
		copyTLSFile := func(destName, srcPath string) {
			destPath := filepath.Join(environment["PORTUNUS_SLAPD_STATE_DIR"], destName)
			buf := must.Return(os.ReadFile(srcPath))
			must.Succeed(os.WriteFile(destPath, buf, 0400))
			must.Succeed(os.Chown(destPath, ids["PORTUNUS_SLAPD_UID"], ids["PORTUNUS_SLAPD_GID"]))
		}

		copyTLSFile("cert.pem", environment["PORTUNUS_SLAPD_TLS_CERTIFICATE"])
		copyTLSFile("key.pem", environment["PORTUNUS_SLAPD_TLS_PRIVATE_KEY"])
		copyTLSFile("ca.pem", environment["PORTUNUS_SLAPD_TLS_CA_CERTIFICATE"])
	}

	//setup our state directory with the correct permissions
	statePath := environment["PORTUNUS_SERVER_STATE_DIR"]
	must.Succeed(os.MkdirAll(statePath, 0770))
	must.Succeed(os.Chown(statePath, ids["PORTUNUS_SERVER_UID"], ids["PORTUNUS_SERVER_GID"]))

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
