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

	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/majewsky/portunus/internal/core"
	h "github.com/majewsky/portunus/internal/html"
	"github.com/majewsky/portunus/internal/static"
	"github.com/sapcc/go-bits/logg"
)

//HTTPHandler returns the main http.Handler.
func HTTPHandler(engine core.Engine, isBehindTLSProxy bool) http.Handler {
	r := mux.NewRouter()
	r.Methods("GET").Path(`/static/{path:.+}`).HandlerFunc(staticHandler)
	r.Methods("GET").Path(`/users`).HandlerFunc(testHandler) //TODO remove
	r.Methods("GET").Path(`/login`).HandlerFunc(getLoginHandler)
	r.Methods("POST").Path(`/login`).HandlerFunc(postLoginHandler)
	r.Methods("GET").Path(`/logout`).HandlerFunc(getLogoutHandler)
	//TODO logout
	//TODO CRUD users
	//TODO CRUD groups

	//setup CSRF with maxAge = 30 minutes
	csrfKey := securecookie.GenerateRandomKey(32)
	csrfMiddleware := csrf.Protect(csrfKey, csrf.MaxAge(1800), csrf.Secure(isBehindTLSProxy))
	handler := csrfMiddleware(r)

	handler = redirectToLoginPageUnlessLoggedIn(handler)

	return handler
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

var sessionStore = sessions.NewCookieStore(securecookie.GenerateRandomKey(32))

func getSessionOrFail(w http.ResponseWriter, r *http.Request) *sessions.Session {
	session, err := sessionStore.Get(r, "portunus-login")
	if err != nil {
		//the session is broken - start a fresh one
		logg.Error("could not decode user session cookie: " + err.Error())
		r.Header.Del("Cookie")
		session, err = sessionStore.New(r, "portunus-login")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return nil
		}
	}
	if session == nil {
		http.Error(w, "unexpected empty session", http.StatusInternalServerError)
		return nil
	}
	return session
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	WriteHTMLPage(w, http.StatusOK, "Users",
		h.Join(
			RenderNavbar("Jane Doe", //TODO: use actual user ID
				NavbarItem{URL: "/users", Title: "Users", Active: true},
				NavbarItem{URL: "/groups", Title: "Groups"},
			),
			h.Tag("main",
				h.Tag("table",
					h.Tag("thead",
						h.Tag("tr",
							h.Tag("th", h.Text("User ID")),
							h.Tag("th", h.Text("Name")),
							h.Tag("th", h.Text("Groups")),
							h.Tag("th", h.Attr("class", "actions"),
								h.Tag("a",
									h.Attr("href", "#"),
									h.Attr("class", "btn btn-primary"),
									h.Text("New user"),
								),
							),
						),
					),
					h.Tag("tbody",
						h.Tag("tr",
							h.Tag("td", h.Text("jane")),
							h.Tag("td", h.Text("Jane Doe")),
							h.Tag("td",
								h.Tag("a", h.Attr("href", "#"), h.Text("Administrators")),
								h.Text(", "),
								h.Tag("a", h.Attr("href", "#"), h.Text("Users")),
							),
							h.Tag("td", h.Attr("class", "actions"),
								h.Tag("a", h.Attr("href", "#"), h.Text("Edit")),
								h.Text(" · "),
								h.Tag("a", h.Attr("href", "#"), h.Text("Delete")),
							),
						),
						h.Tag("tr",
							h.Tag("td", h.Text("john")),
							h.Tag("td", h.Text("John Doe")),
							h.Tag("td",
								h.Tag("a", h.Attr("href", "#"), h.Text("Users")),
							),
							h.Tag("td", h.Attr("class", "actions"),
								h.Tag("a", h.Attr("href", "#"), h.Text("Edit")),
								h.Text(" · "),
								h.Tag("a", h.Attr("href", "#"), h.Text("Delete")),
							),
						),
					),
				),
			),
		),
	)
}
