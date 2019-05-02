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
	"github.com/majewsky/portunus/internal/static"
	"github.com/sapcc/go-bits/logg"
)

//HTTPHandler returns the main http.Handler.
func HTTPHandler(engine core.Engine, isBehindTLSProxy bool) http.Handler {
	r := mux.NewRouter()
	r.Methods("GET").Path(`/static/{path:.+}`).HandlerFunc(staticHandler)

	r.Methods("GET").Path(`/login`).HandlerFunc(getLoginHandler(engine))
	r.Methods("POST").Path(`/login`).HandlerFunc(postLoginHandler(engine))
	r.Methods("GET").Path(`/logout`).HandlerFunc(getLogoutHandler(engine))

	r.Methods("GET").Path(`/self`).HandlerFunc(getSelfHandler(engine))
	r.Methods("POST").Path(`/self`).HandlerFunc(postSelfHandler(engine))

	r.Methods("GET").Path(`/users`).HandlerFunc(getUsersHandler(engine))
	//TODO CRUD users
	//TODO CRUD groups
	//TODO self-service UI (view own user, change password)
	//TODO first-time setup hint

	//TODO FIXME CSRF validation error should redirect to the same form with a flash message (and we should not logg.Error() that anymore then)

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

//Checks whether the user making the request is authenticated and has at least
//the given permissions.
func checkAuth(w http.ResponseWriter, r *http.Request, e core.Engine, requiredPerms core.Permissions) (core.User, core.Permissions, bool) {
	//TODO replace redirectToLoginPageUnlessLoggedIn with consistent usage of this
	s := getSessionOrFail(w, r)
	if s == nil {
		return core.User{}, core.Permissions{}, false
	}
	uid, ok := s.Values["uid"].(string)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return core.User{}, core.Permissions{}, false
	}
	user, userPerms, ok := e.FindUser(uid)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return core.User{}, core.Permissions{}, false
	}
	if !userPerms.Includes(requiredPerms) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return core.User{}, core.Permissions{}, false
	}
	return user, userPerms, true
}
