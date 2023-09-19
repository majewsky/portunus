/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package frontend

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/majewsky/portunus/internal/core"
	h "github.com/majewsky/portunus/internal/html"
)

func getGroupsHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		VerifyPermissions(adminPerms),
		ShowView(groupsList(n)),
	)
}

var groupsListSnippet = h.NewSnippet(`
	<table class="table responsive">
		<thead>
			<tr>
				<th>Name</th>
				<th>Long name</th>
				<th>POSIX ID</th>
				<th>Members</th>
				<th>Permissions granted</th>
				<th class="actions">
					<a href="/groups/new" class="button button-primary">New group</a>
				</th>
			</tr>
		</thead>
		<tbody>
			{{range .}}
				<tr>
					<td data-label="Name"><code>{{.Group.Name}}</code></td>
					<td data-label="Long name">{{.Group.LongName}}</td>
					{{ if .Group.PosixGID -}}
						<td data-label="POSIX ID">{{.Group.PosixGID}}</td>
					{{- else -}}
						<td data-label="POSIX ID" class="text-muted">None</td>
					{{- end }}
					<td data-label="Members">{{.MemberCount}}</td>
					<td data-label="Permissions granted">{{.PermissionsText}}</td>
					<td class="actions">
						<a href="/groups/{{.Group.Name}}/edit">Edit</a>
						·
						<a href="/groups/{{.Group.Name}}/delete">Delete</a>
					</td>
				</tr>
			{{end}}
		</tbody>
	</table>
`)

func groupsList(n core.Nexus) func(*Interaction) Page {
	return func(_ *Interaction) Page {
		groups := n.ListGroups()
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

func useGroupForm(n core.Nexus) HandlerStep {
	return func(i *Interaction) {
		i.FormState = &h.FormState{
			Fields: map[string]*h.FieldState{},
		}
		i.FormSpec = &h.FormSpec{
			Fields: []h.FormField{
				buildGroupMasterdataFieldset(n, i.TargetGroup, i.FormState),
				buildGroupPermissionsFieldset(i.TargetGroup, i.FormState),
				buildGroupPosixFieldset(i.TargetGroup, i.FormState),
				buildGroupMemberFieldset(n, i.TargetGroup, i.FormState),
			},
		}

		if i.TargetGroup == nil {
			i.FormSpec.PostTarget = "/groups/new"
			i.FormSpec.SubmitLabel = "Create group"
		} else {
			i.FormSpec.PostTarget = "/groups/" + i.TargetGroup.Name + "/edit"
			i.FormSpec.SubmitLabel = "Save"
		}
	}
}

func buildGroupMasterdataFieldset(n core.Nexus, g *core.Group, state *h.FormState) h.FormField {
	var nameField h.FormField
	if g == nil {
		mustNotBeInUse := func(name string) error {
			_, exists := n.FindGroup(func(g core.Group) bool { return g.Name == name })
			if exists {
				return errors.New("is already in use")
			}
			return nil
		}
		nameField = h.InputFieldSpec{
			InputType: "text",
			Name:      "name",
			Label:     "Name",
			Rules: []h.ValidationRule{
				core.MustNotBeEmpty,
				core.MustNotHaveSurroundingSpaces,
				core.MustBePosixAccountName,
				mustNotBeInUse,
			},
		}
	} else {
		nameField = h.StaticField{
			Label: "Name",
			Value: codeTagSnippet.Render(g.Name),
		}
		state.Fields["long_name"] = &h.FieldState{Value: g.LongName}
	}

	return h.FieldSet{
		Label:      "Master data",
		IsFoldable: false,
		Fields: []h.FormField{
			nameField,
			h.InputFieldSpec{
				InputType: "text",
				Name:      "long_name",
				Label:     "Long name",
				Rules: []h.ValidationRule{
					core.MustNotBeEmpty,
					core.MustNotHaveSurroundingSpaces,
				},
			},
		},
	}
}

func buildGroupPermissionsFieldset(g *core.Group, state *h.FormState) h.FormField {
	if g != nil {
		state.Fields["portunus_perms"] = &h.FieldState{
			Selected: map[string]bool{
				"is_admin": g.Permissions.Portunus.IsAdmin,
			},
		}
		state.Fields["ldap_perms"] = &h.FieldState{
			Selected: map[string]bool{
				"can_read": g.Permissions.LDAP.CanRead,
			},
		}
	}

	return h.FieldSet{
		Label:      "Permissions",
		IsFoldable: false,
		Fields: []h.FormField{
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
		},
	}
}

func buildGroupMemberFieldset(n core.Nexus, g *core.Group, state *h.FormState) h.FormField {
	allUsers := n.ListUsers()
	sort.Slice(allUsers, func(i, j int) bool {
		return allUsers[i].LoginName < allUsers[j].LoginName
	})
	var memberOpts []h.SelectOptionSpec
	isUserSelected := make(map[string]bool)
	for _, user := range allUsers {
		memberOpts = append(memberOpts, h.SelectOptionSpec{
			Value: user.LoginName,
			Label: user.LoginName,
		})
		if g != nil {
			isUserSelected[user.LoginName] = g.ContainsUser(user)
		}
	}
	if g != nil {
		state.Fields["members"] = &h.FieldState{Selected: isUserSelected}
	}

	return h.FieldSet{
		Label:      "Users",
		IsFoldable: false,
		Fields: []h.FormField{
			h.SelectFieldSpec{
				Name:    "members",
				Label:   "Members of this Group",
				Options: memberOpts,
			},
		},
	}
}

func buildGroupPosixFieldset(g *core.Group, state *h.FormState) h.FormField {
	if g != nil && g.PosixGID != nil {
		state.Fields["posix"] = &h.FieldState{IsUnfolded: true}
		state.Fields["posix_gid"] = &h.FieldState{Value: g.PosixGID.String()}
	}

	return h.FieldSet{
		Name:       "posix",
		Label:      "Is a POSIX group",
		IsFoldable: true,
		Fields: []h.FormField{
			h.InputFieldSpec{
				Name:      "posix_gid",
				Label:     "Group ID",
				InputType: "text",
				Rules: []h.ValidationRule{
					core.MustNotBeEmpty,
					core.MustNotHaveSurroundingSpaces,
					core.MustBePosixUIDorGID,
				},
			},
		},
	}
}

func getGroupEditHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		VerifyPermissions(adminPerms),
		loadTargetGroup(n),
		useGroupForm(n),
		ShowForm("Edit group"),
	)
}

func postGroupEditHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		VerifyPermissions(adminPerms),
		loadTargetGroup(n),
		useGroupForm(n),
		ReadFormStateFromRequest,
		ShowFormIfErrors("Edit group"),
		executeEditGroupForm(n),
	)
}

func loadTargetGroup(n core.Nexus) HandlerStep {
	return func(i *Interaction) {
		groupName := mux.Vars(i.Req)["name"]
		group, exists := n.FindGroup(func(g core.Group) bool { return g.Name == groupName })
		if exists {
			i.TargetGroup = &group
		} else {
			msg := fmt.Sprintf("Group %q does not exist.", groupName)
			i.RedirectWithFlashTo("/groups", Flash{"danger", msg})
		}
	}
}

func buildGroupFromFormState(fs *h.FormState, name string) core.Group {
	result := core.Group{
		Name:             name,
		LongName:         fs.Fields["long_name"].Value,
		MemberLoginNames: fs.Fields["members"].Selected,
		Permissions: core.Permissions{
			Portunus: core.PortunusPermissions{
				IsAdmin: fs.Fields["portunus_perms"].Selected["is_admin"],
			},
			LDAP: core.LDAPPermissions{
				CanRead: fs.Fields["ldap_perms"].Selected["can_read"],
			},
		},
		PosixGID: nil,
	}
	if fs.Fields["posix"].IsUnfolded {
		gidAsUint64, _ := strconv.ParseUint(fs.Fields["posix_gid"].Value, 10, 16)
		gid := core.PosixID(gidAsUint64)
		result.PosixGID = &gid
	}
	return result
}

func executeEditGroupForm(n core.Nexus) HandlerStep {
	return func(i *Interaction) {
		errs := n.Update(func(db *core.Database) error {
			newGroup := buildGroupFromFormState(i.FormState, i.TargetGroup.Name)
			return db.Groups.Update(newGroup)
		}, interactiveUpdate)
		if !errs.IsEmpty() {
			i.RedirectWithFlashTo("/groups", Flash{"danger", errs.Join(", ")})
			return
		}

		msg := fmt.Sprintf("Updated group %q.", i.TargetGroup.Name)
		i.RedirectWithFlashTo("/groups", Flash{"success", msg})
	}
}

func getGroupsNewHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		VerifyPermissions(adminPerms),
		useGroupForm(n),
		ShowForm("Create group"),
	)
}

func postGroupsNewHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		VerifyPermissions(adminPerms),
		useGroupForm(n),
		ReadFormStateFromRequest,
		ShowFormIfErrors("Create group"),
		executeCreateGroupForm(n),
	)
}

func executeCreateGroupForm(n core.Nexus) HandlerStep {
	return func(i *Interaction) {
		groupName := i.FormState.Fields["name"].Value
		errs := n.Update(func(db *core.Database) error {
			newGroup := buildGroupFromFormState(i.FormState, groupName)
			db.Groups = append(db.Groups, newGroup)
			return nil
		}, interactiveUpdate)
		if !errs.IsEmpty() {
			i.RedirectWithFlashTo("/groups", Flash{"danger", errs.Join(", ")})
			return
		}

		msg := fmt.Sprintf("Created group %q.", groupName)
		i.RedirectWithFlashTo("/groups", Flash{"success", msg})
	}
}

func getGroupDeleteHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		VerifyPermissions(adminPerms),
		loadTargetGroup(n),
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

func postGroupDeleteHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		VerifyPermissions(adminPerms),
		loadTargetGroup(n),
		executeDeleteGroup(n),
	)
}

func executeDeleteGroup(n core.Nexus) HandlerStep {
	return func(i *Interaction) {
		groupName := i.TargetGroup.Name
		errs := n.Update(func(db *core.Database) error {
			return db.Groups.Delete(groupName)
		}, interactiveUpdate)
		if !errs.IsEmpty() {
			i.RedirectWithFlashTo("/groups", Flash{"danger", errs.Join(", ")})
			return
		}

		msg := fmt.Sprintf("Deleted group %q.", groupName)
		i.RedirectWithFlashTo("/groups", Flash{"success", msg})
	}
}
