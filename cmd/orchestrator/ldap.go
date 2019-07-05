/******************************************************************************
*
*  Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
*
*  Licensed under the Apache License, Version 2.0 (the "License");
*  you may not use this file except in compliance with the License.
*  You may obtain a copy of the License at
*
*      http://www.apache.org/licenses/LICENSE-2.0
*
*  Unless required by applicable law or agreed to in writing, software
*  distributed under the License is distributed on an "AS IS" BASIS,
*  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*  See the License for the specific language governing permissions and
*  limitations under the License.
*
******************************************************************************/

package main

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/majewsky/portunus/internal/core"
	"github.com/sapcc/go-bits/logg"
)

//TODO: TLS (bind to ldap://127.0.0.1 and ldaps:///)

//Notes on this configuration template:
//- Only Portunus' own technical user has any sort of write access.
var configTemplate = `
include %PORTUNUS_SLAPD_SCHEMA_DIR%/core.schema
include %PORTUNUS_SLAPD_SCHEMA_DIR%/cosine.schema
include %PORTUNUS_SLAPD_SCHEMA_DIR%/inetorgperson.schema
include %PORTUNUS_SLAPD_SCHEMA_DIR%/nis.schema

include %PORTUNUS_SLAPD_STATE_DIR%/portunus.schema

access to dn.base="" by * read
access to dn.base="cn=Subschema" by * read

access to *
	by dn.base="cn=portunus,%PORTUNUS_LDAP_SUFFIX%" write
	by group.exact="cn=portunus-viewers,%PORTUNUS_LDAP_SUFFIX%" read
	by anonymous auth

database   mdb
maxsize    1073741824
suffix     "%PORTUNUS_LDAP_SUFFIX%"
rootdn     "cn=portunus,%PORTUNUS_LDAP_SUFFIX%"
rootpw     "%PORTUNUS_LDAP_PASSWORD_HASH%"
directory  "%PORTUNUS_SLAPD_STATE_DIR%/data"

index objectClass eq
`

//We do not use the OLC machinery for the memberOf attribute because
//portunus-server itself can do it much more easily. But that means we have to
//define the memberOf attribute on the schema level.
var customSchema = `
	attributetype ( 9999.1.1 NAME 'memberOf'
		DESC 'back-reference to groups this user is a member of'
		SUP distinguishedName )

	objectclass ( 9999.2.1 NAME 'hasMemberOf'
		DESC 'addon to objectClass person that permits memberOf attribute'
		SUP top AUXILIARY
		MAY memberOf )

`

func renderSlapdConfig(environment map[string]string, ids map[string]int) []byte {
	password := generateServiceUserPassword()
	logg.Debug("password for cn=portunus,%s is %s",
		environment["PORTUNUS_LDAP_SUFFIX"], password)
	environment["PORTUNUS_LDAP_PASSWORD"] = password
	environment["PORTUNUS_LDAP_PASSWORD_HASH"] = core.HashPasswordForLDAP(password)

	config := regexp.MustCompile(`%\w+%`).
		ReplaceAllStringFunc(configTemplate, func(match string) string {
			match = strings.TrimPrefix(match, "%")
			match = strings.TrimSuffix(match, "%")
			return environment[match]
		})

	return []byte(config)
}

func generateServiceUserPassword() string {
	buf := make([]byte, 32)
	_, err := rand.Read(buf[:])
	if err != nil {
		logg.Fatal(err.Error())
	}
	return hex.EncodeToString(buf[:])
}

//Does not return. Call with `go`.
func runLDAPServer(environment map[string]string) {
	logg.Info("starting LDAP server")
	//run slapd
	cmd := exec.Command(environment["PORTUNUS_SLAPD_BINARY"],
		"-u", environment["PORTUNUS_SLAPD_USER"],
		"-g", environment["PORTUNUS_SLAPD_GROUP"],
		"-h", "ldap:///",
		"-f", filepath.Join(environment["PORTUNUS_SLAPD_STATE_DIR"], "slapd.conf"),
		"-d", "0", //no debug logging (but still important because presence of `-d` keeps slapd from daemonizing)
	)
	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		logg.Error("error encountered while running slapd: " + err.Error())
		logg.Info("Since slapd logs to syslog only, check there for more information.")
		os.Exit(1)
	}
}
