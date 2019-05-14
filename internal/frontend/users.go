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

func getUsersHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(e),
		VerifyPermissions(adminPerms),
		ShowView(usersList(e)),
	)
}

func usersList(e core.Engine) func(*Interaction) Page {
	return func(i *Interaction) Page {
		groups := e.ListGroups()
		sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
		users := e.ListUsers()
		sort.Slice(users, func(i, j int) bool { return users[i].LoginName < users[j].LoginName })

		var userRows []h.TagArgument
		for _, user := range users {
			var groupMemberships []h.RenderedHTML
			for _, group := range groups {
				if !group.ContainsUser(user) {
					continue
				}
				if len(groupMemberships) > 0 {
					groupMemberships = append(groupMemberships, h.Text(", "))
				}
				groupMemberships = append(groupMemberships, h.Tag("a",
					h.Attr("href", "/groups/"+group.Name+"/edit"),
					h.Text(group.LongName),
				))
			}

			userURL := "/users/" + user.LoginName
			userRows = append(userRows, h.Tag("tr",
				h.Tag("td", h.Tag("code", h.Text(user.LoginName))),
				h.Tag("td", h.Text(user.FullName())),
				h.Tag("td", h.Join(groupMemberships...)),
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

		return Page{
			Status:   http.StatusOK,
			Title:    "Users",
			Contents: usersTable,
			Wide:     true,
		}
	}
}

func useUserForm(e core.Engine) HandlerStep {
	return func(i *Interaction) {
		i.FormSpec = &h.FormSpec{}
		i.FormState = &h.FormState{
			Fields: map[string]*h.FieldState{},
		}

		if i.TargetUser == nil {
			i.FormSpec.PostTarget = "/users/new"
			i.FormSpec.SubmitLabel = "Create user"
		} else {
			i.FormSpec.PostTarget = "/users/" + i.TargetUser.LoginName + "/edit"
			i.FormSpec.SubmitLabel = "Save"
		}

		if i.TargetUser == nil {
			mustNotBeInUse := func(loginName string) error {
				if e.FindUser(loginName) != nil {
					return errors.New("is already in use")
				}
				return nil
			}
			i.FormSpec.Fields = append(i.FormSpec.Fields, h.InputFieldSpec{
				InputType: "text",
				Name:      "uid",
				Label:     "Login name",
				Rules: []h.ValidationRule{
					h.MustNotBeEmpty,
					h.MustNotHaveSurroundingSpaces,
					h.MustBePosixAccountName,
					mustNotBeInUse,
				},
			})
		} else {
			i.FormSpec.Fields = append(i.FormSpec.Fields, h.StaticField{
				Label: "Login name",
				Value: h.Tag("code", h.Text(i.TargetUser.LoginName)),
			})
		}

		i.FormSpec.Fields = append(i.FormSpec.Fields,
			h.InputFieldSpec{
				InputType: "text",
				Name:      "given_name",
				Label:     "Given name",
				Rules: []h.ValidationRule{
					h.MustNotBeEmpty,
					h.MustNotHaveSurroundingSpaces,
				},
			},
			h.InputFieldSpec{
				InputType: "text",
				Name:      "family_name",
				Label:     "Family name",
				Rules: []h.ValidationRule{
					h.MustNotBeEmpty,
					h.MustNotHaveSurroundingSpaces,
				},
			},
		)
		if i.TargetUser != nil {
			i.FormState.Fields["given_name"] = &h.FieldState{Value: i.TargetUser.GivenName}
			i.FormState.Fields["family_name"] = &h.FieldState{Value: i.TargetUser.FamilyName}
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
			if i.TargetUser != nil {
				isGroupSelected[group.Name] = group.ContainsUser(*i.TargetUser)
			}
		}
		i.FormSpec.Fields = append(i.FormSpec.Fields, h.SelectFieldSpec{
			Name:    "memberships",
			Label:   "Group memberships",
			Options: groupOpts,
		})
		i.FormState.Fields["memberships"] = &h.FieldState{Selected: isGroupSelected}

		//TODO: allow to reset password for existing user (in case they forgot it)
		if i.TargetUser == nil {
			i.FormSpec.Fields = append(i.FormSpec.Fields,
				h.InputFieldSpec{
					InputType: "password",
					Name:      "password",
					Label:     "Initial password",
					Rules: []h.ValidationRule{
						h.MustNotBeEmpty,
					},
				},
				h.InputFieldSpec{
					InputType: "password",
					Name:      "repeat_password",
					Label:     "Repeat password",
					Rules: []h.ValidationRule{
						h.MustNotBeEmpty,
					},
				},
			)
		}
	}
}

func getUserEditHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(e),
		VerifyPermissions(adminPerms),
		loadTargetUser(e),
		useUserForm(e),
		ShowForm("Edit user"),
	)
}

func postUserEditHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(e),
		VerifyPermissions(adminPerms),
		loadTargetUser(e),
		useUserForm(e),
		ReadFormStateFromRequest,
		ShowFormIfErrors("Edit user"),
		executeEditUserForm(e),
	)
}

func loadTargetUser(e core.Engine) HandlerStep {
	return func(i *Interaction) {
		userLoginName := mux.Vars(i.Req)["uid"]
		user := e.FindUser(userLoginName)
		if user == nil {
			msg := fmt.Sprintf("User %q does not exist.", userLoginName)
			i.RedirectWithFlashTo("/users", Flash{"error", msg})
		} else {
			i.TargetUser = &user.User
		}
	}
}

func executeEditUserForm(e core.Engine) HandlerStep {
	return func(i *Interaction) {
		err := e.ChangeUser(i.TargetUser.LoginName, func(u core.User) (*core.User, error) {
			if u.LoginName == "" {
				return nil, fmt.Errorf("no such user")
			}
			u.GivenName = i.FormState.Fields["given_name"].Value
			u.FamilyName = i.FormState.Fields["family_name"].Value
			return &u, nil
		})
		if err != nil {
			i.RedirectWithFlashTo("/users", Flash{"error", err.Error()})
			return
		}

		isMemberOf := i.FormState.Fields["memberships"].Selected
		for _, group := range e.ListGroups() {
			e.ChangeGroup(group.Name, func(g core.Group) (*core.Group, error) {
				if g.Name == "" {
					return nil, nil //if the group was deleted in parallel, no need to complain
				}
				g.MemberLoginNames[i.TargetUser.LoginName] = isMemberOf[group.Name]
				return &g, nil
			})
		}

		msg := fmt.Sprintf("Updated user %q.", i.TargetUser.LoginName)
		i.RedirectWithFlashTo("/users", Flash{"success", msg})
	}
}

func getUsersNewHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(e),
		VerifyPermissions(adminPerms),
		useUserForm(e),
		ShowForm("Create user"),
	)
}

func postUsersNewHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(e),
		VerifyPermissions(adminPerms),
		useUserForm(e),
		ReadFormStateFromRequest,
		validateCreateUserForm,
		ShowFormIfErrors("Create user"),
		executeCreateUserForm(e),
	)
}

func validateCreateUserForm(i *Interaction) {
	fs := i.FormState
	password1 := fs.Fields["password"].Value
	password2 := fs.Fields["repeat_password"].Value
	if password1 != password2 {
		fs.Fields["repeat_password"].ErrorMessage = "did not match"
	}
}

func executeCreateUserForm(e core.Engine) HandlerStep {
	return func(i *Interaction) {
		loginName := i.FormState.Fields["uid"].Value
		passwordHash := core.HashPasswordForLDAP(i.FormState.Fields["password"].Value)
		e.ChangeUser(loginName, func(u core.User) (*core.User, error) {
			return &core.User{
				LoginName:    loginName,
				GivenName:    i.FormState.Fields["given_name"].Value,
				FamilyName:   i.FormState.Fields["family_name"].Value,
				PasswordHash: passwordHash,
			}, nil
		})

		isMemberOf := i.FormState.Fields["memberships"].Selected
		for _, group := range e.ListGroups() {
			if !isMemberOf[group.Name] {
				continue
			}
			e.ChangeGroup(group.Name, func(g core.Group) (*core.Group, error) {
				if g.Name == "" {
					return nil, nil //if the group was deleted in parallel, no need to complain
				}
				g.MemberLoginNames[loginName] = true
				return &g, nil
			})
		}

		msg := fmt.Sprintf("Created user %q.", loginName)
		i.RedirectWithFlashTo("/users", Flash{"success", msg})
	}
}

func getUserDeleteHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(e),
		VerifyPermissions(adminPerms),
		loadTargetUser(e),
		useDeleteUserForm,
		UseEmptyFormState,
		ShowForm("Confirm user deletion"),
	)
}

func useDeleteUserForm(i *Interaction) {
	if i.TargetUser.LoginName == i.CurrentUser.LoginName {
		i.RedirectWithFlashTo("/users", Flash{"error", "You cannot delete yourself."})
		return
	}

	i.FormSpec = &h.FormSpec{
		PostTarget:  "/users/" + i.TargetUser.LoginName + "/delete",
		SubmitLabel: "Delete user",
		Fields: []h.FormField{
			h.StaticField{
				Value: h.Tag("p",
					h.Text("Really delete user "),
					h.Tag("code", h.Text(i.TargetUser.LoginName)),
					h.Text("? This cannot be undone."),
				),
			},
		},
	}
}

func postUserDeleteHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(e),
		VerifyPermissions(adminPerms),
		loadTargetUser(e),
		executeDeleteUser(e),
	)
}

func executeDeleteUser(e core.Engine) HandlerStep {
	return func(i *Interaction) {
		userLoginName := i.TargetUser.LoginName
		e.ChangeUser(userLoginName, func(core.User) (*core.User, error) {
			return nil, nil
		})
		for _, group := range e.ListGroups() {
			e.ChangeGroup(group.Name, func(g core.Group) (*core.Group, error) {
				if g.Name == "" {
					return nil, nil //if the group was deleted in parallel, no need to complain
				}
				g.MemberLoginNames[userLoginName] = false
				return &g, nil
			})
		}

		msg := fmt.Sprintf("Deleted user %q.", userLoginName)
		i.RedirectWithFlashTo("/users", Flash{"success", msg})
	}
}
