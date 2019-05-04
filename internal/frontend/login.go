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

	"github.com/gorilla/sessions"
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

var loginForm = h.FormSpec{
	PostTarget:  "/login",
	SubmitLabel: "Login",
	Fields: []h.FormField{
		h.InputFieldSpec{
			InputType: "text",
			Name:      "uid",
			Label:     "Login name",
			AutoFocus: true,
			Rules: []h.ValidationRule{
				h.MustNotBeEmpty,
			},
		},
		h.InputFieldSpec{
			InputType: "password",
			Name:      "password",
			Label:     "Password",
			Rules: []h.ValidationRule{
				h.MustNotBeEmpty,
			},
		},
	},
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

		writeLoginPage(w, r, h.FormState{}, s)
	}
}

func writeLoginPage(w http.ResponseWriter, r *http.Request, fs h.FormState, s *sessions.Session) {
	page{
		Status:   http.StatusOK,
		Title:    "Login",
		Contents: loginForm.Render(r, fs),
	}.Render(w, r, nil, s)
}

//Handles POST /login.
func postLoginHandler(e core.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := getSessionOrFail(w, r)
		if s == nil {
			return
		}

		var fs h.FormState
		loginForm.ReadState(r, &fs)
		uid := fs.Fields["uid"].Value
		pwd := fs.Fields["password"].Value

		var user *core.UserWithPerms
		if fs.IsValid() {
			user = e.FindUser(uid)
			passwordHash := ""
			if user != nil {
				passwordHash = user.PasswordHash
			}
			if !core.CheckPasswordHash(pwd, passwordHash) {
				fs.Fields["password"].ErrorMessage = "is not valid (or the user account does not exist)"
			}
		}

		if !fs.IsValid() {
			writeLoginPage(w, r, fs, s)
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
