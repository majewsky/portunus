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
			if e.FindUser(uid) != nil {
				//already logged in
				http.Redirect(w, r, "/self", http.StatusSeeOther)
				return
			}
		}

		WriteHTMLPage(w, http.StatusOK, "Login", h.Join(
			RenderNavbar("", NavbarItem{URL: "/login", Title: "Login", Active: true}),
			h.Tag("main", loginForm{}.Render(r)),
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

		var l loginForm
		hasErrors := false

		uid := r.PostForm.Get("uid")
		if uid == "" {
			l.UserName.ErrorMessage = "is missing"
			hasErrors = true
		} else {
			l.UserName.Value = uid
		}

		password := r.PostForm.Get("password")
		if password == "" {
			l.Password.ErrorMessage = "is missing"
			hasErrors = true
		}

		var user *core.UserWithPerms
		if !hasErrors {
			user = e.FindUser(uid)
			passwordHash := ""
			if user != nil {
				passwordHash = user.PasswordHash
			}
			if !core.CheckPasswordHash(password, passwordHash) {
				l.Password.ErrorMessage = "is not valid (or the user account does not exist)"
				hasErrors = true
			}
		}

		if hasErrors {
			WriteHTMLPage(w, http.StatusOK, "Login", h.Join(
				RenderNavbar("", NavbarItem{URL: "/login", Title: "Login", Active: true}),
				h.Tag("main", l.Render(r)),
			))
			return
		}

		s.Values["uid"] = uid
		err := s.Save(r, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/self", http.StatusSeeOther)
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

type loginForm struct {
	UserName FieldState
	Password FieldState
}

func (f loginForm) Render(r *http.Request) h.RenderedHTML {
	fieldLoginName := FieldSpec{InputType: "text", Name: "uid", Label: "Login name", AutoFocus: true}
	fieldPassword := FieldSpec{InputType: "password", Name: "password", Label: "Password"}

	return h.Tag("form", h.Attr("method", "POST"), h.Attr("action", "/login"),
		h.Embed(csrf.TemplateField(r)),
		fieldLoginName.Render(f.UserName),
		fieldPassword.Render(f.Password),
		h.Tag("div", h.Attr("class", "button-row"),
			h.Tag("button", h.Attr("type", "submit"), h.Attr("class", "btn btn-primary"), h.Text("Login")),
		),
	)
}
