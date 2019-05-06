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
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/majewsky/portunus/internal/core"
	"github.com/majewsky/portunus/internal/frontend"
	"github.com/sapcc/go-bits/logg"
)

func main() {
	logg.ShowDebug = true //TODO make configurable
	dropPrivileges()

	fs := core.FileStore{
		Path: filepath.Join(os.Getenv("PORTUNUS_SERVER_STATE_DIR"), "database.json"),
	}
	fsAPI := fs.RunAsync()

	ldapWorker := newLDAPWorker()
	engine, ldapUpdatesChan := core.RunEngineAsync(fsAPI, ldapWorker.DNSuffix)
	go ldapWorker.processEvents(ldapUpdatesChan)

	handler := frontend.HTTPHandler(engine, os.Getenv("PORTUNUS_SERVER_HTTP_SECURE") == "true")
	logg.Fatal(http.ListenAndServe(os.Getenv("PORTUNUS_SERVER_HTTP_LISTEN"), handler).Error())
}

func dropPrivileges() {
	gidParsed, err := strconv.ParseUint(os.Getenv("PORTUNUS_SERVER_GID"), 10, 32)
	if err != nil {
		logg.Fatal("cannot parse PORTUNUS_SERVER_GID: " + err.Error())
	}
	gid := int(gidParsed)
	err = syscall.Setresgid(gid, gid, gid)
	if err != nil {
		logg.Fatal("change GID failed: " + err.Error())
	}

	uidParsed, err := strconv.ParseUint(os.Getenv("PORTUNUS_SERVER_UID"), 10, 32)
	if err != nil {
		logg.Fatal("cannot parse PORTUNUS_SERVER_UID: " + err.Error())
	}
	uid := int(uidParsed)
	err = syscall.Setresuid(uid, uid, uid)
	if err != nil {
		logg.Fatal("change UID failed: " + err.Error())
	}
}
