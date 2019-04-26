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
	"crypto/rand"
	"encoding/hex"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/majewsky/portunus/internal/core"
	"github.com/sapcc/go-bits/logg"
)

//Notes on this configuration template:
//- Only Portunus' own technical user has any sort of write access.
var configTemplate = `
include %SLAPDSCHEMADIR%/core.schema
include %SLAPDSCHEMADIR%/cosine.schema
include %SLAPDSCHEMADIR%/inetorgperson.schema
include %SLAPDSCHEMADIR%/nis.schema

access to dn.base="" by * read
access to dn.base="cn=Subschema" by * read
access to *
	by dn.base="cn=portunus,%SUFFIX%" write
	by users read
	by anonymous auth

database   mdb
maxsize    1073741824
suffix     "%SUFFIX%"
rootdn     "cn=portunus,%SUFFIX%"
rootpw     "%PASSWORD%"
directory  "%RUNTIMEPATH%/slapd-data"

index objectClass eq
`

var ldapSuffixRx = regexp.MustCompile(`^dc=[a-z0-9_-]+(?:,dc=[a-z0-9_-]+)*$`)

//TODO: TLS (bind to ldap://127.0.0.1 and ldaps:///)
//TODO: restrict read access to users in groups with respective permissions

//RunServer runs slapd and updates its database whenever an event is received.
//This function does not return.
func RunServer(eventsChan <-chan core.Event) {
	//prepare the runtime directory for slapd
	runtimePath := filepath.Join(core.Getenv("XDG_RUNTIME_DIR").Must(), "portunus")
	err := os.RemoveAll(runtimePath)
	if err != nil {
		logg.Fatal(err.Error())
	}
	err = os.Mkdir(runtimePath, 0700)
	if err != nil {
		logg.Fatal(err.Error())
	}
	err = os.Mkdir(filepath.Join(runtimePath, "slapd-data"), 0700)
	if err != nil {
		logg.Fatal(err.Error())
	}

	//generate configuration
	suffix := core.Getenv("PORTUNUS_LDAP_SUFFIX").Format(ldapSuffixRx).Must()
	userDN := "cn=portunus," + suffix
	password, passwordHash := generateServiceUserPassword()
	logg.Debug("password for %s is %s", userDN, password)

	config := configTemplate
	config = strings.Replace(config, "%SUFFIX%", suffix, -1)
	config = strings.Replace(config, "%RUNTIMEPATH%", runtimePath, -1)
	config = strings.Replace(config, "%PASSWORD%", passwordHash, -1)
	config = strings.Replace(config, "%SLAPDSCHEMADIR%", core.Getenv("PORTUNUS_SLAPD_SCHEMA_DIR").Or("/etc/openldap/schema"), -1)

	configPath := filepath.Join(runtimePath, "slapd.conf")
	err = ioutil.WriteFile(configPath, []byte(config), 0400)
	if err != nil {
		logg.Fatal(err.Error())
	}

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

func generateServiceUserPassword() (plain, hashed string) {
	buf := make([]byte, 32)
	_, err := rand.Read(buf[:])
	if err != nil {
		logg.Fatal(err.Error())
	}
	plain = hex.EncodeToString(buf[:])
	return plain, core.HashPasswordForLDAP(plain)
}
