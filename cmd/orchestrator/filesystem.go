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
	"path/filepath"

	"github.com/sapcc/go-bits/logg"
	osutil_user "github.com/tredoe/osutil/user"
)

func prepareFilesystem(environment map[string]string) {
	//resolve user/group names into IDs for os.Chown()
	portunusUID := getUIDForName(environment["PORTUNUS_USER"])
	portunusGID := getGIDForName(environment["PORTUNUS_GROUP"])
	slapdUID := getUIDForName(environment["PORTUNUS_SLAPD_USER"])
	slapdGID := getGIDForName(environment["PORTUNUS_SLAPD_GROUP"])

	//delete leftovers from previous runs
	runtimePath := filepath.Join(environment["XDG_RUNTIME_DIR"], "portunus")
	err := os.RemoveAll(runtimePath)
	if err != nil {
		logg.Fatal(err.Error())
	}

	//setup our runtime directory with the correct permissions
	err = os.Mkdir(runtimePath, 0700)
	if err != nil {
		logg.Fatal(err.Error())
	}

	slapdDataPath := filepath.Join(runtimePath, "slapd-data")
	err = os.Mkdir(slapdDataPath, 0770)
	if err != nil {
		logg.Fatal(err.Error())
	}
	err = os.Chown(slapdDataPath, slapdUID, slapdGID)
	if err != nil {
		logg.Fatal(err.Error())
	}

	//setup our state directory with the correct permissions
	statePath := environment["PORTUNUS_STATE_DIR"]
	err = os.MkdirAll(statePath, 0770)
	if err != nil {
		logg.Fatal(err.Error())
	}
	err = os.Chown(statePath, portunusUID, portunusGID)
	if err != nil {
		logg.Fatal(err.Error())
	}

	fmt.Println("portunus-orchestrator")
}

func getGIDForName(name string) int {
	group, err := osutil_user.LookupGroup(name)
	if err != nil {
		logg.Fatal("cannot find group %s: %s", name, err.Error())
	}
	if group == nil {
		logg.Fatal("cannot find group %s", name)
	}
	return group.GID
}

func getUIDForName(name string) int {
	user, err := osutil_user.LookupUser(name)
	if err != nil {
		logg.Fatal("cannot find user %s: %s", name, err.Error())
	}
	if user == nil {
		logg.Fatal("cannot find user %s", name)
	}
	return user.UID
}
