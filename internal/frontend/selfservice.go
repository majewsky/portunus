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
	"sort"

	"github.com/majewsky/portunus/internal/core"
	h "github.com/majewsky/portunus/internal/html"
)

func getSelfServiceForm(user core.UserWithPerms) h.FormSpec {
	isAdmin := user.Perms.Portunus.IsAdmin
	var memberships []h.RenderedHTML
	sort.Slice(user.GroupMemberships, func(i, j int) bool {
		return user.GroupMemberships[i].LongName < user.GroupMemberships[j].LongName
	})
	for _, group := range user.GroupMemberships {
		if isAdmin {
			memberships = append(memberships, h.Tag("a",
				h.Attr("href", "/groups/"+group.Name),
				h.Attr("class", "item item-checked"),
				h.Text(group.LongName),
			))
		} else {
			memberships = append(memberships, h.Tag("span",
				h.Attr("class", "item item-checked"),
				h.Text(group.LongName),
			))
		}
	}

	return h.FormSpec{
		PostTarget:  "/self",
		SubmitLabel: "Change password",
		Fields: []h.FormField{
			h.StaticField{
				Label: "Login name",
				Value: h.Tag("code", h.Text(user.LoginName)),
			},
			h.StaticField{
				Label: "Full name",
				Value: h.Join(
					//TODO: allow flipped order (family name first)
					h.Tag("span", h.Attr("class", "given-name"), h.Text(user.GivenName)),
					h.Text(" "),
					h.Tag("span", h.Attr("class", "family-name"), h.Text(user.FamilyName)),
				),
			},
			h.StaticField{
				Label:      "Group memberships",
				CSSClasses: "item-list",
				Value:      h.Join(memberships...),
			},
			h.FieldSpec{
				InputType: "password",
				Name:      "old_password",
				Label:     "Old password",
				Rules: []h.ValidationRule{
					h.MustNotBeEmpty,
				},
			},
			h.FieldSpec{
				InputType: "password",
				Name:      "new_password",
				Label:     "New password",
				Rules: []h.ValidationRule{
					h.MustNotBeEmpty,
				},
			},
			h.FieldSpec{
				InputType: "password",
				Name:      "repeat_password",
				Label:     "Repeat password",
				Rules: []h.ValidationRule{
					h.MustNotBeEmpty,
				},
			},
		},
	}
}

func getSelfHandler(e core.Engine) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentUser, s := checkAuth(w, r, e, core.Permissions{})
		if currentUser == nil {
			return
		}

		page{
			Status:   http.StatusOK,
			Title:    "My profile",
			Contents: getSelfServiceForm(*currentUser).Render(r, h.FormState{}),
		}.Render(w, r, currentUser, s)
	})
}

func postSelfHandler(e core.Engine) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentUser, s := checkAuth(w, r, e, core.Permissions{})
		if currentUser == nil {
			return
		}

		f := getSelfServiceForm(*currentUser)
		var fs h.FormState
		f.ReadState(r, &fs)

		if fs.IsValid() {
			newPassword1 := fs.Fields["new_password"].Value
			newPassword2 := fs.Fields["repeat_password"].Value
			if newPassword1 != newPassword2 {
				fs.Fields["repeat_password"].ErrorMessage = "did not match"
			}
		}

		if fs.IsValid() {
			oldPassword := fs.Fields["old_password"].Value
			if !core.CheckPasswordHash(oldPassword, currentUser.PasswordHash) {
				fs.Fields["old_password"].ErrorMessage = "is not correct"
			}
		}

		if fs.IsValid() {
			//TODO perform the change
			s.AddFlash(flash{"success", h.Text("Password changed.")})
		}

		page{
			Status:   http.StatusOK,
			Title:    "My profile",
			Contents: f.Render(r, fs),
		}.Render(w, r, currentUser, s)
	})
}
