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

package frontend

import (
	"bytes"
	"net/http"
	"path"

	"github.com/gorilla/mux"
	"github.com/majewsky/portunus/internal/static"
)

//HTTPHandler returns the main http.Handler.
func HTTPHandler() http.Handler {
	r := mux.NewRouter()
	r.Methods("GET").Path(`/static/{path:.+}`).HandlerFunc(staticHandler)
	r.Methods("GET").Path(`/`).HandlerFunc(entryHandler)
	return r
}

func staticHandler(w http.ResponseWriter, r *http.Request) {
	assetPath := mux.Vars(r)["path"]
	assetBytes, err := static.Asset(assetPath)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	assetInfo, err := static.AssetInfo(assetPath)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	http.ServeContent(w, r, path.Base(assetPath), assetInfo.ModTime(), bytes.NewReader(assetBytes))
}

func entryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<link rel="stylesheet" href="/static/css/spectre.min.css"><p>Hallo Welt</p>`))
}
