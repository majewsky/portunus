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

var adminPerms = core.Permissions{
	Portunus: core.PortunusPermissions{
		IsAdmin: true,
	},
}

func getUsersHandler(e core.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser := checkAuth(w, r, e, adminPerms)
		if currentUser == nil {
			return
		}

		groups := e.ListGroups()
		sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
		users := e.ListUsers()
		sort.Slice(users, func(i, j int) bool { return users[i].LoginName < users[j].LoginName })

		var userRows []h.TagArgument
		for _, user := range users {
			userURL := "/users/" + user.LoginName
			userRows = append(userRows, h.Tag("tr",
				h.Tag("td", h.Text(user.LoginName)),
				h.Tag("td", h.Text(user.FullName())),
				h.Tag("td", RenderGroupMemberships(user, groups, *currentUser)),
				h.Tag("td", h.Attr("class", "actions"),
					h.Tag("a", h.Attr("href", userURL), h.Text("Edit")),
					h.Text(" · "),
					h.Tag("a", h.Attr("href", userURL+"/delete"), h.Text("Delete")),
				),
			))
		}

		usersTable := h.Tag("table",
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
			h.Tag("tbody", userRows...),
		)

		WriteHTMLPage(w, http.StatusOK, "Users",
			h.Join(
				RenderNavbarForUser(*currentUser, r),
				h.Tag("main", usersTable),
			),
		)
	}
}
