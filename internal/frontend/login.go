/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package frontend

import (
	"net/http"
	"strings"

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
				Name:             "user_ident",
				Label:            "Login name or email address",
				AutoFocus:        true,
				AutocompleteMode: "on",
				Rules: []h.ValidationRule{
					core.MustNotBeEmpty,
				},
			},
			h.InputFieldSpec{
				InputType:        "password",
				Name:             "password",
				Label:            "Password",
				AutocompleteMode: "on",
				Rules: []h.ValidationRule{
					core.MustNotBeEmpty,
				},
			},
		},
	}
}

// Handles GET /login.
func getLoginHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		skipLoginIfAlreadyLoggedIn(n),
		useLoginForm,
		UseEmptyFormState,
		ShowForm("Login"),
	)
}

func skipLoginIfAlreadyLoggedIn(n core.Nexus) HandlerStep {
	return func(i *Interaction) {
		if uid, ok := i.Session.Values["uid"].(string); ok {
			_, exists := n.FindUser(func(u core.User) bool { return u.LoginName == uid })
			if exists {
				i.RedirectTo("/self")
			}
		}
	}
}

// Handles POST /login.
func postLoginHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		useLoginForm,
		ReadFormStateFromRequest,
		checkLogin(n),
		ShowFormIfErrors("Login"),
		SaveSession,
		RedirectTo("/self"),
	)
}

func checkLogin(n core.Nexus) HandlerStep {
	return func(i *Interaction) {
		fs := i.FormState
		userIdent := fs.Fields["user_ident"].Value //either uid or email address
		pwd := fs.Fields["password"].Value

		if fs.IsValid() {
			var predicate func(core.User) bool
			if strings.Contains(userIdent, "@") {
				predicate = func(u core.User) bool { return u.EMailAddress == userIdent }
			} else {
				predicate = func(u core.User) bool { return u.LoginName == userIdent }
			}
			user, exists := n.FindUser(predicate)
			passwordHash := ""
			if exists {
				passwordHash = user.PasswordHash
			}
			if core.CheckPasswordHash(pwd, passwordHash) {
				i.Session.Values["uid"] = user.LoginName
			} else {
				fs.Fields["password"].ErrorMessage = "is not valid (or the user account does not exist)"
			}
		}
	}
}

// Handles GET /logout.
func getLogoutHandler(n core.Nexus) http.Handler {
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
