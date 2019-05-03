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

	"github.com/gorilla/csrf"
	"github.com/majewsky/portunus/internal/core"
	h "github.com/majewsky/portunus/internal/html"
)

func getSelfHandler(e core.Engine) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentUser := checkAuth(w, r, e, core.Permissions{})
		if currentUser == nil {
			return
		}

		WriteHTMLPage(w, http.StatusOK, "Users",
			h.Join(
				RenderNavbarForUser(*currentUser, r),
				h.Tag("main", selfServiceForm{User: *currentUser}.Render(r)),
			),
		)
	})
}

func postSelfHandler(e core.Engine) http.HandlerFunc {
	//TODO implement POST /self
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	})
}

type selfServiceForm struct {
	User      core.UserWithPerms
	Password1 FieldState
	Password2 FieldState
}

func (f selfServiceForm) Render(r *http.Request) h.RenderedHTML {
	fieldPassword1 := FieldSpec{InputType: "password", Name: "password1", Label: "New password"}
	fieldPassword2 := FieldSpec{InputType: "password", Name: "password2", Label: "Repeat password"}

	return h.Tag("form", h.Attr("method", "POST"), h.Attr("action", "/self"),
		h.Embed(csrf.TemplateField(r)),
		RenderDisplayField("Login name", h.Tag("code", h.Text(f.User.LoginName))),
		RenderDisplayField("Full name",
			//TODO: allow flipped order (family name first)
			h.Tag("span", h.Attr("class", "given-name"), h.Text(f.User.GivenName)),
			h.Text(" "),
			h.Tag("span", h.Attr("class", "family-name"), h.Text(f.User.FamilyName)),
		),
		RenderDisplayField("Group memberships", RenderGroupMemberships(f.User.User, f.User.GroupMemberships, f.User)),
		fieldPassword1.Render(f.Password1),
		fieldPassword2.Render(f.Password2),
		h.Tag("div", h.Attr("class", "button-row"),
			h.Tag("button", h.Attr("type", "submit"), h.Attr("class", "btn btn-primary"), h.Text("Change password")),
		),
	)
}
