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
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/majewsky/portunus/internal/core"
	"github.com/sapcc/go-bits/logg"
)

//TODO: TLS (bind to ldap://127.0.0.1 and ldaps:///)
//TODO: restrict read access to users in groups with respective permissions

//Notes on this configuration template:
//- Only Portunus' own technical user has any sort of write access.
var configTemplate = `
include %PORTUNUS_SLAPD_SCHEMA_DIR%/core.schema
include %PORTUNUS_SLAPD_SCHEMA_DIR%/cosine.schema
include %PORTUNUS_SLAPD_SCHEMA_DIR%/inetorgperson.schema
include %PORTUNUS_SLAPD_SCHEMA_DIR%/nis.schema

access to dn.base="" by * read
access to dn.base="cn=Subschema" by * read
access to *
	by dn.base="cn=portunus,%PORTUNUS_LDAP_SUFFIX%" write
	by users read
	by anonymous auth

database   mdb
maxsize    1073741824
suffix     "%PORTUNUS_LDAP_SUFFIX%"
rootdn     "cn=portunus,%PORTUNUS_LDAP_SUFFIX%"
rootpw     "%PORTUNUS_LDAP_PASSWORD%"
directory  "%XDG_RUNTIME_DIR%/portunus/slapd-data"

index objectClass eq
`

func renderLDAPConfig(environment map[string]string) {
	var password string
	password, environment["PORTUNUS_LDAP_PASSWORD"] = generateServiceUserPassword()
	logg.Debug("password for cn=portunus,%s is %s",
		environment["PORTUNUS_LDAP_SUFFIX"], password)

	config := regexp.MustCompile(`%\w+%`).
		ReplaceAllStringFunc(configTemplate, func(match string) string {
			match = strings.TrimPrefix(match, "%")
			match = strings.TrimSuffix(match, "%")
			return environment[match]
		})

	configPath := filepath.Join(environment["XDG_RUNTIME_DIR"], "portunus", "slapd.conf")
	err := ioutil.WriteFile(configPath, []byte(config), 0444)
	if err != nil {
		logg.Fatal(err.Error())
	}
}

func generateServiceUserPassword() (plain, hashed string) {
	buf := make([]byte, 32)
	_, err := rand.Read(buf[:])
	if err != nil {
		logg.Fatal(err.Error())
	}
	plain = hex.EncodeToString(buf[:])
	return plain, core.HashPasswordForLDAP(plain)
}
