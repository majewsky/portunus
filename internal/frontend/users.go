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
	"github.com/sapcc/go-bits/errext"
)

var adminPerms = core.Permissions{
	Portunus: core.PortunusPermissions{
		IsAdmin: true,
	},
}

func getUsersHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		VerifyPermissions(adminPerms),
		ShowView(usersList(n)),
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

func usersList(n core.Nexus) func(*Interaction) Page {
	return func(_ *Interaction) Page {
		groups := n.ListGroups()
		sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
		users := n.ListUsers()
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

func useUserForm(n core.Nexus) HandlerStep {
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
			buildUserMasterdataFieldset(n, i.TargetUser, i.FormState),
			buildUserPosixFieldset(i.TargetUser, i.FormState),
			buildUserPasswordFieldset(i.TargetUser),
		)

	}
}

func buildUserMasterdataFieldset(n core.Nexus, u *core.User, state *h.FormState) h.FormField {
	var fields []h.FormField
	if u == nil {
		mustNotBeInUse := func(loginName string) error {
			_, exists := n.FindUser(func(u core.User) bool { return u.LoginName == loginName })
			if exists {
				return errors.New("is already in use")
			}
			return nil
		}
		fields = append(fields, h.InputFieldSpec{
			InputType: "text",
			Name:      "uid",
			Label:     "Login name",
			Rules: []h.ValidationRule{
				core.MustNotBeEmpty,
				core.MustNotHaveSurroundingSpaces,
				core.MustBePosixAccountName,
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
				core.MustNotBeEmpty,
				core.MustNotHaveSurroundingSpaces,
			},
		},
		h.InputFieldSpec{
			InputType: "text",
			Name:      "family_name",
			Label:     "Family name",
			Rules: []h.ValidationRule{
				core.MustNotBeEmpty,
				core.MustNotHaveSurroundingSpaces,
			},
		},
		h.InputFieldSpec{
			InputType: "text",
			Name:      "email",
			Label:     "Email address (optional in Portunus, but required by some services)",
			Rules: []h.ValidationRule{
				core.MustNotHaveSurroundingSpaces,
			},
		},
		h.MultilineInputFieldSpec{
			Name:  "ssh_public_keys",
			Label: "SSH public key(s)",
			Rules: []h.ValidationRule{
				core.MustNotHaveSurroundingSpaces,
				core.MustBeSSHPublicKeys,
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

	allGroups := n.ListGroups()
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
						core.MustNotBeEmpty,
					},
				},
				h.InputFieldSpec{
					InputType: "password",
					Name:      "repeat_password",
					Label:     "Repeat password",
					Rules: []h.ValidationRule{
						core.MustNotBeEmpty,
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
					core.MustNotBeEmpty,
				},
			},
			h.InputFieldSpec{
				InputType: "password",
				Name:      "repeat_password",
				Label:     "Repeat password",
				Rules: []h.ValidationRule{
					core.MustNotBeEmpty,
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
					core.MustNotBeEmpty,
					core.MustNotHaveSurroundingSpaces,
					core.MustBePosixUIDorGID,
				},
			},
			h.InputFieldSpec{
				Name:      "posix_gid",
				Label:     "Primary group ID",
				InputType: "text",
				Rules: []h.ValidationRule{
					core.MustNotBeEmpty,
					core.MustNotHaveSurroundingSpaces,
					core.MustBePosixUIDorGID,
				},
			},
			h.InputFieldSpec{
				Name:      "posix_home",
				Label:     "Home directory",
				InputType: "text",
				Rules: []h.ValidationRule{
					core.MustNotBeEmpty,
					core.MustNotHaveSurroundingSpaces,
					core.MustBeAbsolutePath,
				},
			},
			h.InputFieldSpec{
				Name:      "posix_shell",
				Label:     "Login shell (optional)",
				InputType: "text",
				Rules: []h.ValidationRule{
					core.MustBeAbsolutePath,
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

func getUserEditHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		VerifyPermissions(adminPerms),
		loadTargetUser(n),
		useUserForm(n),
		ShowForm("Edit user"),
	)
}

func postUserEditHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		VerifyPermissions(adminPerms),
		loadTargetUser(n),
		useUserForm(n),
		ReadFormStateFromRequest,
		validateEditUserForm,
		ShowFormIfErrors("Edit user"),
		executeEditUserForm(n),
	)
}

func loadTargetUser(n core.Nexus) HandlerStep {
	return func(i *Interaction) {
		userLoginName := mux.Vars(i.Req)["uid"]
		user, exists := n.FindUser(func(u core.User) bool { return u.LoginName == userLoginName })
		if exists {
			i.TargetUser = &user.User
		} else {
			msg := fmt.Sprintf("User %q does not exist.", userLoginName)
			i.RedirectWithFlashTo("/users", Flash{"danger", msg})
		}
	}
}

func buildUserFromFormState(fs *h.FormState, loginName, passwordHash string) core.User {
	result := core.User{
		LoginName:     loginName,
		GivenName:     fs.Fields["given_name"].Value,
		FamilyName:    fs.Fields["family_name"].Value,
		EMailAddress:  fs.Fields["email"].Value,
		SSHPublicKeys: core.SplitSSHPublicKeys(fs.Fields["ssh_public_keys"].Value),
		PasswordHash:  passwordHash,
		POSIX:         nil,
	}
	if fs.Fields["posix"].IsUnfolded {
		uidAsUint64, _ := strconv.ParseUint(fs.Fields["posix_uid"].Value, 10, 16)
		gidAsUint64, _ := strconv.ParseUint(fs.Fields["posix_gid"].Value, 10, 16)
		result.POSIX = &core.UserPosixAttributes{
			UID:           core.PosixID(uidAsUint64),
			GID:           core.PosixID(gidAsUint64),
			HomeDirectory: fs.Fields["posix_home"].Value,
			LoginShell:    fs.Fields["posix_shell"].Value,
			GECOS:         fs.Fields["posix_gecos"].Value,
		}
	}
	return result
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

func executeEditUserForm(n core.Nexus) HandlerStep {
	return func(i *Interaction) {
		passwordHash := i.TargetUser.PasswordHash
		if i.FormState.Fields["reset_password"].IsUnfolded {
			if pw := i.FormState.Fields["password"].Value; pw != "" {
				passwordHash = n.PasswordHasher().HashPassword(pw)
			}
		}

		errs := n.Update(func(db *core.Database) (errs errext.ErrorSet) {
			newUser := buildUserFromFormState(i.FormState, i.TargetUser.LoginName, passwordHash)
			errs.Add(db.Users.Update(newUser))

			isMemberOf := i.FormState.Fields["memberships"].Selected
			for idx := range db.Groups {
				group := &db.Groups[idx]
				if group.MemberLoginNames == nil {
					group.MemberLoginNames = make(map[string]bool)
				}
				group.MemberLoginNames[i.TargetUser.LoginName] = isMemberOf[group.Name]
			}
			return
		}, interactiveUpdate)
		if !errs.IsEmpty() {
			i.RedirectWithFlashTo("/users", Flash{"danger", errs.Join(", ")})
			return
		}

		msg := fmt.Sprintf("Updated user %q.", i.TargetUser.LoginName)
		i.RedirectWithFlashTo("/users", Flash{"success", msg})
	}
}

func getUsersNewHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		VerifyPermissions(adminPerms),
		useUserForm(n),
		ShowForm("Create user"),
	)
}

func postUsersNewHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		VerifyPermissions(adminPerms),
		useUserForm(n),
		ReadFormStateFromRequest,
		validateCreateUserForm,
		ShowFormIfErrors("Create user"),
		executeCreateUserForm(n),
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

func executeCreateUserForm(n core.Nexus) HandlerStep {
	return func(i *Interaction) {
		loginName := i.FormState.Fields["uid"].Value
		passwordHash := n.PasswordHasher().HashPassword(i.FormState.Fields["password"].Value)

		errs := n.Update(func(db *core.Database) (errs errext.ErrorSet) {
			newUser := buildUserFromFormState(i.FormState, loginName, passwordHash)
			db.Users = append(db.Users, newUser)

			isMemberOf := i.FormState.Fields["memberships"].Selected
			for idx := range db.Groups {
				group := &db.Groups[idx]
				if group.MemberLoginNames == nil {
					group.MemberLoginNames = make(map[string]bool)
				}
				group.MemberLoginNames[loginName] = isMemberOf[group.Name]
			}
			return
		}, interactiveUpdate)
		if !errs.IsEmpty() {
			i.RedirectWithFlashTo("/users", Flash{"danger", errs.Join(", ")})
			return
		}

		msg := fmt.Sprintf("Created user %q.", loginName)
		i.RedirectWithFlashTo("/users", Flash{"success", msg})
	}
}

func getUserDeleteHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		VerifyPermissions(adminPerms),
		loadTargetUser(n),
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

func postUserDeleteHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		VerifyPermissions(adminPerms),
		loadTargetUser(n),
		executeDeleteUser(n),
	)
}

func executeDeleteUser(n core.Nexus) HandlerStep {
	return func(i *Interaction) {
		userLoginName := i.TargetUser.LoginName
		errs := n.Update(func(db *core.Database) (errs errext.ErrorSet) {
			errs.Add(db.Users.Delete(userLoginName))

			for _, group := range db.Groups {
				if group.MemberLoginNames != nil {
					group.MemberLoginNames[userLoginName] = false
				}
			}
			return
		}, interactiveUpdate)
		if !errs.IsEmpty() {
			i.RedirectWithFlashTo("/users", Flash{"danger", errs.Join(", ")})
			return
		}

		msg := fmt.Sprintf("Deleted user %q.", userLoginName)
		i.RedirectWithFlashTo("/users", Flash{"success", msg})
	}
}
