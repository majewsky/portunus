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

	"github.com/majewsky/portunus/internal/core"
	h "github.com/majewsky/portunus/internal/html"
)

func useLoginForm(i *Interaction) {
	i.FormSpec = &h.FormSpec{
		PostTarget:  "/login",
		SubmitLabel: "Login",
		Fields: []h.FormField{
			h.InputFieldSpec{
				InputType:        "text",
				Name:             "uid",
				Label:            "Login name",
				AutoFocus:        true,
				AutocompleteMode: "on",
				Rules: []h.ValidationRule{
					h.MustNotBeEmpty,
				},
			},
			h.InputFieldSpec{
				InputType:        "password",
				Name:             "password",
				Label:            "Password",
				AutocompleteMode: "on",
				Rules: []h.ValidationRule{
					h.MustNotBeEmpty,
				},
			},
		},
	}
}

//Handles GET /login.
func getLoginHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		skipLoginIfAlreadyLoggedIn(e),
		useLoginForm,
		UseEmptyFormState,
		ShowForm("Login"),
	)
}

func skipLoginIfAlreadyLoggedIn(e core.Engine) HandlerStep {
	return func(i *Interaction) {
		if uid, ok := i.Session.Values["uid"].(string); ok {
			if e.FindUser(uid) != nil {
				i.RedirectTo("/self")
			}
		}
	}
}

//Handles POST /login.
func postLoginHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		useLoginForm,
		ReadFormStateFromRequest,
		checkLogin(e),
		ShowFormIfErrors("Login"),
		SaveSession,
		RedirectTo("/self"),
	)
}

func checkLogin(e core.Engine) HandlerStep {
	return func(i *Interaction) {
		fs := i.FormState
		uid := fs.Fields["uid"].Value
		pwd := fs.Fields["password"].Value

		var user *core.UserWithPerms
		if fs.IsValid() {
			user = e.FindUser(uid)
			passwordHash := ""
			if user != nil {
				passwordHash = user.PasswordHash
			}
			if core.CheckPasswordHash(pwd, passwordHash) {
				i.Session.Values["uid"] = i.FormState.Fields["uid"].Value
			} else {
				fs.Fields["password"].ErrorMessage = "is not valid (or the user account does not exist)"
			}
		}
	}
}

//Handles GET /logout.
func getLogoutHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		clearLogin,
		SaveSession,
		RedirectTo("/login"),
	)
}

func clearLogin(i *Interaction) {
	delete(i.Session.Values, "uid")
}
