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
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/majewsky/portunus/internal/core"
	h "github.com/majewsky/portunus/internal/html"
)

func getGroupsHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(e),
		VerifyPermissions(adminPerms),
		ShowView(groupsList(e)),
	)
}

func groupsList(e core.Engine) func(*Interaction) Page {
	return func(i *Interaction) Page {
		groups := e.ListGroups()
		sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })

		var rows []h.TagArgument
		for _, group := range groups {
			var permTexts []string
			if group.Permissions.Portunus.IsAdmin {
				permTexts = append(permTexts, "Portunus admin")
			}
			if group.Permissions.LDAP.CanRead {
				permTexts = append(permTexts, "LDAP read access")
			}

			if len(permTexts) == 0 {
				permTexts = []string{"None"}
			}

			groupURL := "/group/" + group.Name
			rows = append(rows, h.Tag("tr",
				h.Tag("td", h.Tag("code", h.Text(group.Name))),
				h.Tag("td", h.Text(group.LongName)),
				h.Tag("td", h.Text(fmt.Sprintf("%d", len(group.MemberLoginNames)))),
				h.Tag("td", h.Text(strings.Join(permTexts, ", "))),
				h.Tag("td", h.Attr("class", "actions"),
					h.Tag("a", h.Attr("href", groupURL+"/edit"), h.Text("Edit")),
					h.Text(" · "),
					h.Tag("a", h.Attr("href", groupURL+"/delete"), h.Text("Delete")),
				),
			))
		}

		groupsTable := h.Tag("table",
			h.Tag("thead",
				h.Tag("tr",
					h.Tag("th", h.Text("Name")),
					h.Tag("th", h.Text("Long name")),
					h.Tag("th", h.Text("Members")),
					h.Tag("th", h.Text("Permissions granted")),
					h.Tag("th", h.Attr("class", "actions"),
						h.Tag("a",
							h.Attr("href", "/groups/new"),
							h.Attr("class", "btn btn-primary"),
							h.Text("New group"),
						),
					),
				),
			),
			h.Tag("tbody", rows...),
		)

		return Page{
			Status:   http.StatusOK,
			Title:    "Groups",
			Contents: groupsTable,
		}
	}
}
