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
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/gorilla/mux"
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
		currentUser, s := checkAuth(w, r, e, adminPerms)
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
					h.Tag("a", h.Attr("href", userURL+"/edit"), h.Text("Edit")),
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
							h.Attr("href", "/users/new"),
							h.Attr("class", "btn btn-primary"),
							h.Text("New user"),
						),
					),
				),
			),
			h.Tag("tbody", userRows...),
		)

		page{
			Status:   http.StatusOK,
			Title:    "Users",
			Contents: usersTable,
		}.Render(w, r, currentUser, s)
	}
}

func getUserForm(user *core.User, e core.Engine) (h.FormSpec, h.FormState) {
	var spec h.FormSpec
	state := h.FormState{
		Fields: map[string]*h.FieldState{},
	}

	if user == nil {
		spec.PostTarget = "/users/new"
		spec.SubmitLabel = "Create user"
	} else {
		spec.PostTarget = "/users/" + user.LoginName + "/edit"
		spec.SubmitLabel = "Save"
	}

	if user == nil {
		mustNotBeInUse := func(loginName string) error {
			if e.FindUser(loginName) != nil {
				return errors.New("is already in use")
			}
			return nil
		}
		spec.Fields = append(spec.Fields, h.FieldSpec{
			InputType: "text",
			Name:      "uid",
			Label:     "Login name",
			Rules: []h.ValidationRule{
				h.MustNotBeEmpty,
				//TODO: validate against regex
				mustNotBeInUse,
			},
		})
	} else {
		spec.Fields = append(spec.Fields, h.StaticField{
			Label: "Login name",
			Value: h.Tag("code", h.Text(user.LoginName)),
		})
	}

	spec.Fields = append(spec.Fields,
		h.FieldSpec{
			InputType: "text",
			Name:      "given_name",
			Label:     "Given name",
			Rules: []h.ValidationRule{
				h.MustNotBeEmpty,
				//TODO validate against regex
			},
		},
		h.FieldSpec{
			InputType: "text",
			Name:      "family_name",
			Label:     "Family name",
			Rules: []h.ValidationRule{
				h.MustNotBeEmpty,
				//TODO validate against regex
			},
		},
	)
	if user != nil {
		state.Fields["given_name"] = &h.FieldState{Value: user.GivenName}
		state.Fields["family_name"] = &h.FieldState{Value: user.FamilyName}
	}

	allGroups := e.ListGroups()
	sort.Slice(allGroups, func(i, j int) bool {
		return allGroups[i].LongName < allGroups[j].LongName
	})
	var groupOpts []h.SelectOptionSpec
	isGroupSelected := make(map[string]bool)
	for _, group := range allGroups {
		groupOpts = append(groupOpts, h.SelectOptionSpec{
			Value: group.Name,
			Label: group.LongName,
		})
		isGroupSelected[group.Name] = group.ContainsUser(*user)
	}
	spec.Fields = append(spec.Fields, h.SelectFieldSpec{
		Name:    "memberships",
		Label:   "Group memberships",
		Options: groupOpts,
	})
	state.Fields["memberships"] = &h.FieldState{Selected: isGroupSelected}

	if user == nil {
		spec.Fields = append(spec.Fields, h.FieldSpec{
			InputType: "password",
			Name:      "password",
			Label:     "Initial password",
			Rules: []h.ValidationRule{
				h.MustNotBeEmpty,
			},
		})
	}

	return spec, state
}

func getUserEditHandler(e core.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, s := checkAuth(w, r, e, adminPerms)
		if currentUser == nil {
			return
		}

		userLoginName := mux.Vars(r)["uid"]
		user := e.FindUser(userLoginName)
		if user == nil {
			msg := fmt.Sprintf("User %q does not exist.", userLoginName)
			RedirectWithFlash(w, r, s, "/users", flash{"error", msg})
			return
		}

		f, fs := getUserForm(&user.User, e)
		page{
			Status:   http.StatusOK,
			Title:    "Edit user",
			Contents: f.Render(r, fs),
		}.Render(w, r, currentUser, s)
	}
}

func postUserEditHandler(e core.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, s := checkAuth(w, r, e, adminPerms)
		if currentUser == nil {
			return
		}

		userLoginName := mux.Vars(r)["uid"]
		user := e.FindUser(userLoginName)
		if user == nil {
			msg := fmt.Sprintf("User %q does not exist.", userLoginName)
			RedirectWithFlash(w, r, s, "/users", flash{"error", msg})
			return
		}

		f, fs := getUserForm(&user.User, e)
		f.ReadState(r, &fs)
		if !fs.IsValid() {
			page{
				Status:   http.StatusOK,
				Title:    "Edit user",
				Contents: f.Render(r, fs),
			}.Render(w, r, currentUser, s)
			return
		}

		err := e.ChangeUser(user.LoginName, func(u core.User) (*core.User, error) {
			if u.LoginName == "" {
				return nil, fmt.Errorf("no such user")
			}
			u.GivenName = fs.Fields["given_name"].Value
			u.FamilyName = fs.Fields["family_name"].Value
			return &u, nil
		})
		if err != nil {
			RedirectWithFlash(w, r, s, "/users", flash{"error", err.Error()})
			return
		}

		isMemberOf := fs.Fields["memberships"].Selected
		for _, group := range e.ListGroups() {
			e.ChangeGroup(group.Name, func(g core.Group) (*core.Group, error) {
				if g.Name == "" {
					return nil, nil //if the group was deleted in parallel, no need to complain
				}
				g.MemberLoginNames[user.LoginName] = isMemberOf[group.Name]
				return &g, nil
			})
		}

		//TODO persist selected group memberships

		msg := fmt.Sprintf("Updated user %q.", user.LoginName)
		RedirectWithFlash(w, r, s, "/users", flash{"success", msg})
	}
}

func getUsersNewHandler(e core.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, s := checkAuth(w, r, e, adminPerms)
		if currentUser == nil {
			return
		}

		f, fs := getUserForm(nil, e)
		page{
			Status:   http.StatusOK,
			Title:    "Create user",
			Contents: f.Render(r, fs),
		}.Render(w, r, currentUser, s)
	}
}

func postUsersNewHandler(e core.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//TODO implement
	}
}
