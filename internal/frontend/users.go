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
	"strconv"
	"strings"

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

var usersListSnippet = h.NewSnippet(`
	<table class="table responsive">
		<thead>
			<tr>
				<th>Login name</th>
				<th>Full name</th>
				<th>POSIX ID</th>
				<th>Groups</th>
				<th class="actions">
					<a href="/users/new" class="button button-primary">New user</a>
				</th>
			</tr>
		</thead>
		<tbody>
			{{range .}}
				<tr>
					<td data-label="Login name"><code>{{.User.LoginName}}</code></td>
					<td data-label="Full name">{{.UserFullName}}</td>
					{{ if .User.POSIX -}}
						<td data-label="POSIX ID">{{.User.POSIX.UID}}</td>
					{{- else -}}
						<td data-label="POSIX ID" class="text-muted">None</td>
					{{- end }}
					<td data-label="Groups" class="comma-separated-list">
						{{- range .Groups -}}
						<a href="/groups/{{.Name}}/edit">{{.LongName}}</a><span class="comma">,&nbsp;</span>
						{{- end -}}
					</td>
					<td class="actions">
						<a href="/users/{{.User.LoginName}}/edit">Edit</a>
						·
						<a href="/users/{{.User.LoginName}}/delete">Delete</a>
					</td>
				</tr>
			{{end}}
		</tbody>
	</table>
`)

func usersList(e core.Engine) func(*Interaction) Page {
	return func(i *Interaction) Page {
		groups := e.ListGroups()
		sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
		users := e.ListUsers()
		sort.Slice(users, func(i, j int) bool { return users[i].LoginName < users[j].LoginName })

		type userItem struct {
			User         core.User
			UserFullName string
			Groups       []core.Group
		}
		data := make([]userItem, len(users))
		for idx, user := range users {
			item := userItem{
				User:         user,
				UserFullName: user.FullName(),
			}
			for _, group := range groups {
				if group.ContainsUser(user) {
					item.Groups = append(item.Groups, group)
				}
			}
			data[idx] = item
		}

		return Page{
			Status:   http.StatusOK,
			Title:    "Users",
			Contents: usersListSnippet.Render(data),
			Wide:     true,
		}
	}
}

var codeTagSnippet = h.NewSnippet(`<code>{{.}}</code>`)

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

		i.FormSpec.Fields = append(i.FormSpec.Fields,
			buildUserMasterdataFieldset(e, i.TargetUser, i.FormState),
			buildUserPosixFieldset(i.TargetUser, i.FormState),
			buildUserPasswordFieldset(i.TargetUser),
		)

	}
}

func buildUserMasterdataFieldset(e core.Engine, u *core.User, state *h.FormState) h.FormField {
	var fields []h.FormField
	if u == nil {
		mustNotBeInUse := func(loginName string) error {
			if e.FindUser(loginName) != nil {
				return errors.New("is already in use")
			}
			return nil
		}
		fields = append(fields, h.InputFieldSpec{
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
		fields = append(fields, h.StaticField{
			Label: "Login name",
			Value: codeTagSnippet.Render(u.LoginName),
		})
	}

	fields = append(fields,
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
		h.InputFieldSpec{
			InputType: "text",
			Name:      "email",
			Label:     "Email address (optional in Portunus, but required by some services)",
			Rules: []h.ValidationRule{
				h.MustNotHaveSurroundingSpaces,
			},
		},
		h.MultilineInputFieldSpec{
			Name:  "ssh_public_keys",
			Label: "SSH public key(s)",
			Rules: []h.ValidationRule{
				h.MustNotHaveSurroundingSpaces,
				h.MustBeSSHPublicKeys,
			},
		},
	)
	if u != nil {
		state.Fields["given_name"] = &h.FieldState{Value: u.GivenName}
		state.Fields["family_name"] = &h.FieldState{Value: u.FamilyName}
		state.Fields["email"] = &h.FieldState{Value: u.EMailAddress}
		state.Fields["ssh_public_keys"] = &h.FieldState{
			Value: strings.Join(u.SSHPublicKeys, "\r\n"),
		}
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
		if u != nil {
			isGroupSelected[group.Name] = group.ContainsUser(*u)
		}
	}
	fields = append(fields, h.SelectFieldSpec{
		Name:    "memberships",
		Label:   "Group memberships",
		Options: groupOpts,
	})
	state.Fields["memberships"] = &h.FieldState{Selected: isGroupSelected}

	return h.FieldSet{
		Label:      "Master data",
		Fields:     fields,
		IsFoldable: false,
	}
}

func buildUserPasswordFieldset(u *core.User) h.FormField {
	if u == nil {
		return h.FieldSet{
			Label:      "Initial password",
			IsFoldable: false,
			Fields: []h.FormField{
				h.InputFieldSpec{
					InputType: "password",
					Name:      "password",
					Label:     "Password",
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
			},
		}
	}
	return h.FieldSet{
		Name:       "reset_password",
		Label:      "Reset password",
		IsFoldable: true,
		Fields: []h.FormField{
			h.InputFieldSpec{
				InputType: "password",
				Name:      "password",
				Label:     "New password",
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
		},
	}
}

func buildUserPosixFieldset(u *core.User, state *h.FormState) h.FormField {
	if u != nil && u.POSIX != nil {
		state.Fields["posix"] = &h.FieldState{IsUnfolded: true}
		state.Fields["posix_uid"] = &h.FieldState{Value: u.POSIX.UID.String()}
		state.Fields["posix_gid"] = &h.FieldState{Value: u.POSIX.GID.String()}
		state.Fields["posix_home"] = &h.FieldState{Value: u.POSIX.HomeDirectory}
		state.Fields["posix_shell"] = &h.FieldState{Value: u.POSIX.LoginShell}
		state.Fields["posix_gecos"] = &h.FieldState{Value: u.POSIX.GECOS}
	}

	return h.FieldSet{
		Name:       "posix",
		Label:      "Is a POSIX user account",
		IsFoldable: true,
		Fields: []h.FormField{
			h.InputFieldSpec{
				Name:      "posix_uid",
				Label:     "User ID",
				InputType: "text",
				Rules: []h.ValidationRule{
					h.MustNotBeEmpty,
					h.MustNotHaveSurroundingSpaces,
					h.MustBePosixUIDorGID,
				},
			},
			h.InputFieldSpec{
				Name:      "posix_gid",
				Label:     "Primary group ID",
				InputType: "text",
				Rules: []h.ValidationRule{
					h.MustNotBeEmpty,
					h.MustNotHaveSurroundingSpaces,
					h.MustBePosixUIDorGID,
				},
			},
			h.InputFieldSpec{
				Name:      "posix_home",
				Label:     "Home directory",
				InputType: "text",
				Rules: []h.ValidationRule{
					h.MustNotBeEmpty,
					h.MustNotHaveSurroundingSpaces,
					h.MustBeAbsolutePath,
				},
			},
			h.InputFieldSpec{
				Name:      "posix_shell",
				Label:     "Login shell (optional)",
				InputType: "text",
				Rules: []h.ValidationRule{
					h.MustBeAbsolutePath,
				},
			},
			h.InputFieldSpec{
				Name:      "posix_gecos",
				Label:     `GECOS`,
				InputType: "text",
			},
		},
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
		validateEditUserForm,
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
			i.RedirectWithFlashTo("/users", Flash{"danger", msg})
		} else {
			i.TargetUser = &user.User
		}
	}
}

func validateEditUserForm(i *Interaction) {
	fs := i.FormState
	if i.FormState.Fields["reset_password"].IsUnfolded {
		password1 := fs.Fields["password"].Value
		password2 := fs.Fields["repeat_password"].Value
		if password1 != password2 {
			fs.Fields["repeat_password"].ErrorMessage = "did not match"
		}
	}
}

func executeEditUserForm(e core.Engine) HandlerStep {
	return func(i *Interaction) {
		passwordHash := ""
		if i.FormState.Fields["reset_password"].IsUnfolded {
			if pw := i.FormState.Fields["password"].Value; pw != "" {
				passwordHash = core.HashPasswordForLDAP(pw)
			}
		}
		err := e.ChangeUser(i.TargetUser.LoginName, func(u core.User) (*core.User, error) {
			if u.LoginName == "" {
				return nil, fmt.Errorf("no such user")
			}
			u.GivenName = i.FormState.Fields["given_name"].Value
			u.FamilyName = i.FormState.Fields["family_name"].Value
			u.EMailAddress = i.FormState.Fields["email"].Value
			u.SSHPublicKeys = h.SplitSSHPublicKeys(i.FormState.Fields["ssh_public_keys"].Value)
			if passwordHash != "" {
				u.PasswordHash = passwordHash
			}
			if i.FormState.Fields["posix"].IsUnfolded {
				uidAsUint64, _ := strconv.ParseUint(i.FormState.Fields["posix_uid"].Value, 10, 16)
				gidAsUint64, _ := strconv.ParseUint(i.FormState.Fields["posix_gid"].Value, 10, 16)
				u.POSIX = &core.UserPosixAttributes{
					UID:           core.PosixID(uidAsUint64),
					GID:           core.PosixID(gidAsUint64),
					HomeDirectory: i.FormState.Fields["posix_home"].Value,
					LoginShell:    i.FormState.Fields["posix_shell"].Value,
					GECOS:         i.FormState.Fields["posix_gecos"].Value,
				}
			} else {
				u.POSIX = nil
			}
			return &u, nil
		})
		if err != nil {
			i.RedirectWithFlashTo("/users", Flash{"danger", err.Error()})
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

		var posixAttrs *core.UserPosixAttributes
		if i.FormState.Fields["posix"].IsUnfolded {
			uidAsUint64, _ := strconv.ParseUint(i.FormState.Fields["posix_uid"].Value, 10, 16)
			gidAsUint64, _ := strconv.ParseUint(i.FormState.Fields["posix_gid"].Value, 10, 16)
			posixAttrs = &core.UserPosixAttributes{
				UID:           core.PosixID(uidAsUint64),
				GID:           core.PosixID(gidAsUint64),
				HomeDirectory: i.FormState.Fields["posix_home"].Value,
				LoginShell:    i.FormState.Fields["posix_shell"].Value,
				GECOS:         i.FormState.Fields["posix_gecos"].Value,
			}
		}

		e.ChangeUser(loginName, func(u core.User) (*core.User, error) {
			return &core.User{
				LoginName:    loginName,
				GivenName:    i.FormState.Fields["given_name"].Value,
				FamilyName:   i.FormState.Fields["family_name"].Value,
				EMailAddress: i.FormState.Fields["email"].Value,
				PasswordHash: passwordHash,
				POSIX:        posixAttrs,
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

var deleteUserConfirmSnippet = h.NewSnippet(`
	<p>Really delete user <code>{{.}}</code>? This cannot be undone.</p>
`)

func useDeleteUserForm(i *Interaction) {
	if i.TargetUser.LoginName == i.CurrentUser.LoginName {
		i.RedirectWithFlashTo("/users", Flash{"danger", "You cannot delete yourself."})
		return
	}

	i.FormSpec = &h.FormSpec{
		PostTarget:  "/users/" + i.TargetUser.LoginName + "/delete",
		SubmitLabel: "Delete user",
		Fields: []h.FormField{
			h.StaticField{
				Value: deleteUserConfirmSnippet.Render(i.TargetUser.LoginName),
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
