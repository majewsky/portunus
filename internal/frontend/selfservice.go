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
				InputType: "password1",
				Name:      "password",
				Label:     "New password",
				Rules: []h.ValidationRule{
					h.MustNotBeEmpty,
				},
			},
			h.FieldSpec{
				InputType: "password2",
				Name:      "password",
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
		currentUser := checkAuth(w, r, e, core.Permissions{})
		if currentUser == nil {
			return
		}

		WriteHTMLPage(w, http.StatusOK, "Users",
			h.Join(
				RenderNavbarForUser(*currentUser, r),
				h.Tag("main", getSelfServiceForm(*currentUser).Render(r, h.FormState{})),
			),
		)
	})
}

func postSelfHandler(e core.Engine) http.HandlerFunc {
	//TODO implement POST /self
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	})
}
