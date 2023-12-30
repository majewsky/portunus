/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/majewsky/portunus/internal/grammars"
	"github.com/sapcc/go-bits/logg"
	"github.com/sapcc/go-bits/must"
)

type valueCheck struct {
	Checker    func(string) bool
	FormatDesc string
}

var (
	userOrGroupPattern = `^[a-z_][a-z0-9_-]*\$?$`
	envDefaults        = map[string]string{
		//empty value = not optional
		"PORTUNUS_DEBUG":              "false",
		"PORTUNUS_GROUP_NAME_REGEX":   userOrGroupPattern,
		"PORTUNUS_LDAP_SUFFIX":        "",
		"PORTUNUS_SERVER_BINARY":      "portunus-server",
		"PORTUNUS_SERVER_GROUP":       "portunus",
		"PORTUNUS_SERVER_HTTP_LISTEN": "127.0.0.1:8080",
		"PORTUNUS_SERVER_HTTP_SECURE": "true",
		"PORTUNUS_SERVER_STATE_DIR":   "/var/lib/portunus",
		"PORTUNUS_SERVER_USER":        "portunus",
		"PORTUNUS_SLAPD_BINARY":       "slapd",
		"PORTUNUS_SLAPD_GROUP":        "ldap",
		"PORTUNUS_SLAPD_SCHEMA_DIR":   "/etc/openldap/schema",
		"PORTUNUS_SLAPD_STATE_DIR":    "/var/run/portunus-slapd",
		"PORTUNUS_SLAPD_USER":         "ldap",
		"PORTUNUS_USER_NAME_REGEX":    userOrGroupPattern,
	}

	strictBoolCheck    = valueCheck{isStrictBool, `either "true" or "false"`}
	ldapSuffixCheck    = valueCheck{grammars.IsLDAPSuffix, `an RDN with only dc= components`}
	listenAddressCheck = valueCheck{grammars.IsListenAddress, `a listen address like "1.2.3.4:80" or "[::1]:8080"`}
	posixAcctNameCheck = valueCheck{grammars.IsPOSIXAccountName, "a POSIX account name (see `man 8 useradd` for format description)"}

	envFormats = map[string]valueCheck{
		"PORTUNUS_DEBUG":              strictBoolCheck,
		"PORTUNUS_LDAP_SUFFIX":        ldapSuffixCheck,
		"PORTUNUS_SERVER_GROUP":       posixAcctNameCheck,
		"PORTUNUS_SERVER_HTTP_LISTEN": listenAddressCheck,
		"PORTUNUS_SERVER_HTTP_SECURE": strictBoolCheck,
		"PORTUNUS_SERVER_USER":        posixAcctNameCheck,
		"PORTUNUS_SLAPD_GROUP":        posixAcctNameCheck,
		"PORTUNUS_SLAPD_USER":         posixAcctNameCheck,
	}
)

func isStrictBool(input string) bool {
	return input == "true" || input == "false"
}

func readConfig() (environment map[string]string, ids map[string]int) {
	//last-minute initializations in envDefaults
	if os.Getenv("PORTUNUS_SLAPD_TLS_CERTIFICATE") != "" {
		envDefaults["PORTUNUS_SLAPD_TLS_CERTIFICATE"] = ""
		envDefaults["PORTUNUS_SLAPD_TLS_DOMAIN_NAME"] = ""
		envDefaults["PORTUNUS_SLAPD_TLS_PRIVATE_KEY"] = ""
		envDefaults["PORTUNUS_SLAPD_TLS_CA_CERTIFICATE"] = ""
	}

	//read and validate all relevant environment variables
	environment = make(map[string]string)
	for key, defaultValue := range envDefaults {
		value := os.Getenv(key)
		if value == "" {
			value = defaultValue
		}
		if value == "" {
			logg.Fatal("missing required environment variable: " + key)
		}
		if check := envFormats[key]; check.Checker != nil {
			if !check.Checker(value) {
				logg.Fatal("malformed environment variable: %s must be %s", value, check.FormatDesc)
			}
		}
		environment[key] = value
		os.Unsetenv(key) //avoid unintentional leakage of env vars to child processes
	}

	//resolve user/group names into IDs
	ids = map[string]int{
		"PORTUNUS_SERVER_UID": must.Return(lookupID("/etc/passwd", environment["PORTUNUS_SERVER_USER"])),
		"PORTUNUS_SERVER_GID": must.Return(lookupID("/etc/group", environment["PORTUNUS_SERVER_GROUP"])),
		"PORTUNUS_SLAPD_UID":  must.Return(lookupID("/etc/passwd", environment["PORTUNUS_SLAPD_USER"])),
		"PORTUNUS_SLAPD_GID":  must.Return(lookupID("/etc/group", environment["PORTUNUS_SLAPD_GROUP"])),
	}

	return
}

func lookupID(databasePath, entityName string) (int, error) {
	//In both `/etc/passwd` and `/etc/passwd`:
	//- The columns are colon-separated.
	//- The first column has the entity name.
	//- The third column has the entity's own numeric ID.
	buf := must.Return(os.ReadFile(databasePath))
	for _, line := range strings.Split(string(buf), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, ":")
		if fields[0] != entityName {
			continue
		}

		id, err := strconv.ParseUint(fields[2], 10, 32) // in Linux, uid_t = gid_t = uint32_t
		if err != nil {
			return 0, fmt.Errorf("while reading %q: cannot parse ID for %q: %w",
				databasePath, entityName, err)
		}
		return int(id), nil
	}

	return 0, fmt.Errorf("while reading %q: cannot find ID for %q",
		databasePath, entityName)
}
