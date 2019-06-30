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
	"strings"

	"github.com/gorilla/mux"
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

var groupsListSnippet = h.NewSnippet(`
	<table>
		<thead>
			<tr>
				<th>Name</th>
				<th>Long name</th>
				<th>Members</th>
				<th>Permissions granted</th>
				<th class="actions">
					<a href="/groups/new" class="btn btn-primary">New group</a>
				</th>
			</tr>
		</thead>
		<tbody>
			{{range .}}
				<tr>
					<td><code>{{.Group.Name}}</code></td>
					<td>{{.Group.LongName}}</td>
					<td>{{.MemberCount}}</td>
					<td>{{.PermissionsText}}</td>
					<td class="actions">
						<a href="/groups/{{.Group.Name}}/edit">Edit</a>
						·
						<a href="/groups/{{.Group.Name}}/delete">Delete</a>
					</td>
				<tr>
			{{end}}
		</tbody>
	</table>
`)

func groupsList(e core.Engine) func(*Interaction) Page {
	return func(i *Interaction) Page {
		groups := e.ListGroups()
		sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })

		type groupItem struct {
			Group           core.Group
			MemberCount     int
			PermissionsText string
		}
		data := make([]groupItem, len(groups))
		for idx, group := range groups {
			item := groupItem{
				Group:       group,
				MemberCount: len(group.MemberLoginNames),
			}

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
			item.PermissionsText = strings.Join(permTexts, ", ")

			data[idx] = item
		}

		return Page{
			Status:   http.StatusOK,
			Title:    "Groups",
			Contents: groupsListSnippet.Render(data),
			Wide:     true,
		}
	}
}

func useGroupForm(e core.Engine) HandlerStep {
	return func(i *Interaction) {
		i.FormSpec = &h.FormSpec{}
		i.FormState = &h.FormState{
			Fields: map[string]*h.FieldState{},
		}

		if i.TargetGroup == nil {
			i.FormSpec.PostTarget = "/groups/new"
			i.FormSpec.SubmitLabel = "Create group"
		} else {
			i.FormSpec.PostTarget = "/groups/" + i.TargetGroup.Name + "/edit"
			i.FormSpec.SubmitLabel = "Save"
		}

		if i.TargetGroup == nil {
			mustNotBeInUse := func(name string) error {
				if e.FindGroup(name) != nil {
					return errors.New("is already in use")
				}
				return nil
			}
			i.FormSpec.Fields = append(i.FormSpec.Fields, h.InputFieldSpec{
				InputType: "text",
				Name:      "name",
				Label:     "Name",
				Rules: []h.ValidationRule{
					h.MustNotBeEmpty,
					h.MustNotHaveSurroundingSpaces,
					h.MustBePosixAccountName,
					mustNotBeInUse,
				},
			})
		} else {
			i.FormSpec.Fields = append(i.FormSpec.Fields, h.StaticField{
				Label: "Name",
				Value: codeTagSnippet.Render(i.TargetGroup.Name),
			})
		}

		i.FormSpec.Fields = append(i.FormSpec.Fields,
			h.InputFieldSpec{
				InputType: "text",
				Name:      "long_name",
				Label:     "Long name",
				Rules: []h.ValidationRule{
					h.MustNotBeEmpty,
					h.MustNotHaveSurroundingSpaces,
				},
			},
			h.SelectFieldSpec{
				Name:  "portunus_perms",
				Label: "Grants permissions in Portunus?",
				Options: []h.SelectOptionSpec{
					{
						Value: "is_admin",
						Label: "Admin access",
					},
				},
			},
			h.SelectFieldSpec{
				Name:  "ldap_perms",
				Label: "Grants permissions in LDAP?",
				Options: []h.SelectOptionSpec{
					{
						Value: "can_read",
						Label: "Read access",
					},
				},
			},
		)
		if i.TargetGroup != nil {
			i.FormState.Fields["long_name"] = &h.FieldState{Value: i.TargetGroup.LongName}
			i.FormState.Fields["portunus_perms"] = &h.FieldState{
				Selected: map[string]bool{
					"is_admin": i.TargetGroup.Permissions.Portunus.IsAdmin,
				},
			}
			i.FormState.Fields["ldap_perms"] = &h.FieldState{
				Selected: map[string]bool{
					"can_read": i.TargetGroup.Permissions.LDAP.CanRead,
				},
			}
		}
	}
}

func getGroupEditHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(e),
		VerifyPermissions(adminPerms),
		loadTargetGroup(e),
		useGroupForm(e),
		ShowForm("Edit group"),
	)
}

func postGroupEditHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(e),
		VerifyPermissions(adminPerms),
		loadTargetGroup(e),
		useGroupForm(e),
		ReadFormStateFromRequest,
		ShowFormIfErrors("Edit group"),
		executeEditGroupForm(e),
	)
}

func loadTargetGroup(e core.Engine) HandlerStep {
	return func(i *Interaction) {
		groupName := mux.Vars(i.Req)["name"]
		group := e.FindGroup(groupName)
		if group == nil {
			msg := fmt.Sprintf("Group %q does not exist.", groupName)
			i.RedirectWithFlashTo("/groups", Flash{"error", msg})
		} else {
			i.TargetGroup = group
		}
	}
}

func executeEditGroupForm(e core.Engine) HandlerStep {
	return func(i *Interaction) {
		err := e.ChangeGroup(i.TargetGroup.Name, func(g core.Group) (*core.Group, error) {
			if g.Name == "" {
				return nil, fmt.Errorf("no such group")
			}
			g.LongName = i.FormState.Fields["long_name"].Value
			g.Permissions.Portunus.IsAdmin = i.FormState.Fields["portunus_perms"].Selected["is_admin"]
			g.Permissions.LDAP.CanRead = i.FormState.Fields["ldap_perms"].Selected["can_read"]
			return &g, nil
		})
		if err != nil {
			i.RedirectWithFlashTo("/groups", Flash{"error", err.Error()})
			return
		}

		msg := fmt.Sprintf("Updated group %q.", i.TargetGroup.Name)
		i.RedirectWithFlashTo("/groups", Flash{"success", msg})
	}
}

func getGroupsNewHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(e),
		VerifyPermissions(adminPerms),
		useGroupForm(e),
		ShowForm("Create group"),
	)
}

func postGroupsNewHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(e),
		VerifyPermissions(adminPerms),
		useGroupForm(e),
		ReadFormStateFromRequest,
		ShowFormIfErrors("Create group"),
		executeCreateGroupForm(e),
	)
}

func executeCreateGroupForm(e core.Engine) HandlerStep {
	return func(i *Interaction) {
		name := i.FormState.Fields["name"].Value
		e.ChangeGroup(name, func(g core.Group) (*core.Group, error) {
			return &core.Group{
				Name:     name,
				LongName: i.FormState.Fields["long_name"].Value,
				Permissions: core.Permissions{
					Portunus: core.PortunusPermissions{
						IsAdmin: i.FormState.Fields["portunus_perms"].Selected["is_admin"],
					},
					LDAP: core.LDAPPermissions{
						CanRead: i.FormState.Fields["ldap_perms"].Selected["can_read"],
					},
				},
			}, nil
		})

		msg := fmt.Sprintf("Created group %q.", name)
		i.RedirectWithFlashTo("/groups", Flash{"success", msg})
	}
}

func getGroupDeleteHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(e),
		VerifyPermissions(adminPerms),
		loadTargetGroup(e),
		useDeleteGroupForm,
		UseEmptyFormState,
		ShowForm("Confirm group deletion"),
	)
}

var deleteGroupConfirmSnippet = h.NewSnippet(`
	<p>Really delete group <code>{{.}}</code>? This cannot be undone.</p>
`)

func useDeleteGroupForm(i *Interaction) {
	i.FormSpec = &h.FormSpec{
		PostTarget:  "/groups/" + i.TargetGroup.Name + "/delete",
		SubmitLabel: "Delete group",
		Fields: []h.FormField{
			h.StaticField{
				Value: deleteGroupConfirmSnippet.Render(i.TargetGroup.Name),
			},
		},
	}
}

func postGroupDeleteHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(e),
		VerifyPermissions(adminPerms),
		loadTargetGroup(e),
		executeDeleteGroup(e),
	)
}

func executeDeleteGroup(e core.Engine) HandlerStep {
	return func(i *Interaction) {
		groupName := i.TargetGroup.Name
		e.ChangeGroup(groupName, func(core.Group) (*core.Group, error) {
			return nil, nil
		})

		msg := fmt.Sprintf("Deleted group %q.", groupName)
		i.RedirectWithFlashTo("/groups", Flash{"success", msg})
	}
}
