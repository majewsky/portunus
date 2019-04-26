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
	"os/exec"
	"regexp"

	"github.com/sapcc/go-bits/logg"
)

var (
	envDefaults = map[string]string{
		//empty value = not optional
		"PORTUNUS_LDAP_SUFFIX":      "",
		"PORTUNUS_SERVER_BINARY":    "portunus-server",
		"PORTUNUS_SERVER_GROUP":     "portunus",
		"PORTUNUS_SERVER_USER":      "portunus",
		"PORTUNUS_SLAPD_BINARY":     "slapd",
		"PORTUNUS_SLAPD_GROUP":      "ldap",
		"PORTUNUS_SLAPD_SCHEMA_DIR": "/etc/openldap/schema",
		"PORTUNUS_SLAPD_USER":       "ldap",
		"PORTUNUS_STATE_DIR":        "/var/lib/portunus",
		"XDG_RUNTIME_DIR":           "/run",
	}

	ldapSuffixRx  = regexp.MustCompile(`^dc=[a-z0-9_-]+(?:,dc=[a-z0-9_-]+)*$`)
	userOrGroupRx = regexp.MustCompile(`^[a-z_][a-z0-9_-]*\$?$`)
	envFormats    = map[string]*regexp.Regexp{
		"PORTUNUS_LDAP_SUFFIX":  ldapSuffixRx,
		"PORTUNUS_SERVER_GROUP": userOrGroupRx,
		"PORTUNUS_SERVER_USER":  userOrGroupRx,
		"PORTUNUS_SLAPD_GROUP":  userOrGroupRx,
		"PORTUNUS_SLAPD_USER":   userOrGroupRx,
	}
)

func main() {
	//read and validate all relevant environment variables
	environment := make(map[string]string)
	for key, defaultValue := range envDefaults {
		value := os.Getenv(key)
		if value == "" {
			value = defaultValue
		}
		if value == "" {
			logg.Fatal("missing required environment variable: " + key)
		}
		if rx := envFormats[key]; rx != nil {
			if !rx.MatchString(value) {
				logg.Fatal("malformed environment variable: %s must look like /%s/", value, rx.String())
			}
		}
		environment[key] = value
		os.Unsetenv(key) //avoid unintentional leakage of env vars to child processes
	}

	prepareFilesystem(environment)
	renderLDAPConfig(environment)
	go runLDAPServer(environment)

	//run portunus-server (thus blocking this goroutine)
	portunusUID := getUIDForName(environment["PORTUNUS_SERVER_USER"])
	portunusGID := getGIDForName(environment["PORTUNUS_SERVER_GROUP"])

	cmd := exec.Command(environment["PORTUNUS_SERVER_BINARY"])
	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORTUNUS_SERVER_UID=%d", portunusUID),
		fmt.Sprintf("PORTUNUS_SERVER_GID=%d", portunusGID),
		"PORTUNUS_LDAP_SUFFIX="+environment["PORTUNUS_LDAP_SUFFIX"],
		"PORTUNUS_LDAP_PASSWORD="+environment["PORTUNUS_LDAP_PASSWORD"],
	)
	err := cmd.Run()
	if err != nil {
		logg.Fatal("error encountered while running portunus-server: " + err.Error())
	}
}
