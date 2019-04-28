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
	"net/http"
	"strings"

	"github.com/gorilla/csrf"
	"github.com/majewsky/portunus/internal/core"
	h "github.com/majewsky/portunus/internal/html"
)

func redirectToLoginPageUnlessLoggedIn(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//the login workflow and everything required by it are accessible to everyone
		if r.URL.Path == "/login" || strings.HasPrefix(r.URL.Path, "/static/") {
			h.ServeHTTP(w, r)
			return
		}

		s := getSessionOrFail(w, r)
		if s == nil {
			return
		}
		if _, ok := s.Values["uid"].(string); ok {
			//logged-in users can proceed to the other pages
			h.ServeHTTP(w, r)
			return
		}

		//redirect everything else to the login page
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

//Handles GET /login.
func getLoginHandler(e core.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := getSessionOrFail(w, r)
		if s == nil {
			return
		}
		if uid, ok := s.Values["uid"].(string); ok {
			if _, _, ok := e.FindUser(uid); ok {
				//already logged in
				http.Redirect(w, r, "/users", http.StatusSeeOther)
				return
			}
		}

		WriteHTMLPage(w, http.StatusOK, "Login", h.Join(
			RenderNavbar("", NavbarItem{URL: "/login", Title: "Login", Active: true}),
			h.Tag("main",
				h.Tag("form", h.Attr("method", "POST"), h.Attr("action", "/login"),
					h.Embed(csrf.TemplateField(r)),
					h.Tag("div", h.Attr("class", "form-row"),
						h.Tag("label", h.Attr("for", "uid"), h.Text("User ID")),
						h.Tag("input", h.Attr("name", "uid"), h.Attr("type", "text")),
					),
					h.Tag("div", h.Attr("class", "form-row"),
						h.Tag("label", h.Attr("for", "password"), h.Text("Password")),
						h.Tag("input", h.Attr("name", "username"), h.Attr("type", "password")),
					),
					h.Tag("div", h.Attr("class", "button-row"),
						h.Tag("button", h.Attr("type", "submit"), h.Attr("class", "btn btn-primary"), h.Text("Login")),
					),
				),
			),
		))
	}
}

//Handles POST /login.
func postLoginHandler(e core.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := getSessionOrFail(w, r)
		if s == nil {
			return
		}

		//TODO stub, needs to actually look at r.PostForm
		s.Values["uid"] = "jane"
		err := s.Save(r, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/users", http.StatusSeeOther)
	}
}

//Handles GET /logout.
func getLogoutHandler(e core.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := getSessionOrFail(w, r)
		if s == nil {
			return
		}
		delete(s.Values, "uid")
		err := s.Save(r, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}
