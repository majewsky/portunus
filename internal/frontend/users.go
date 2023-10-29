/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package frontend

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gorilla/mux"
	"github.com/majewsky/portunus/internal/core"
	"github.com/majewsky/portunus/internal/crypt"
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
		fields = append(fields, h.InputFieldSpec{
			InputType: "text",
			Name:      "login_name",
			Label:     "Login name",
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
		},
		h.InputFieldSpec{
			InputType: "text",
			Name:      "family_name",
			Label:     "Family name",
		},
		h.InputFieldSpec{
			InputType: "text",
			Name:      "email",
			Label:     "Email address (optional in Portunus, but required by some services)",
		},
		h.MultilineInputFieldSpec{
			Name:  "ssh_public_keys",
			Label: "SSH public key(s)",
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
	fields := []h.FormField{
		h.InputFieldSpec{
			InputType: "password",
			Name:      "password",
			Label:     "Password",
		},
		h.InputFieldSpec{
			InputType: "password",
			Name:      "repeat_password",
			Label:     "Repeat password",
		},
	}

	if u == nil {
		return h.FieldSet{
			Label:      "Initial password",
			IsFoldable: false,
			Fields:     fields,
		}
	}
	return h.FieldSet{
		Name:       "reset_password",
		Label:      "Reset password",
		IsFoldable: true,
		Fields:     fields,
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
			},
			h.InputFieldSpec{
				Name:      "posix_gid",
				Label:     "Primary group ID",
				InputType: "text",
			},
			h.InputFieldSpec{
				Name:      "posix_home",
				Label:     "Home directory",
				InputType: "text",
			},
			h.InputFieldSpec{
				Name:      "posix_shell",
				Label:     "Login shell (optional)",
				InputType: "text",
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
		validateUserForm,
		TryUpdateNexus(n, executeEditUser),
		ShowFormIfErrors("Edit user"),
		RedirectWithFlashTo("/users", "Updated"),
	)
}

func loadTargetUser(n core.Nexus) HandlerStep {
	return func(i *Interaction) {
		userLoginName := mux.Vars(i.Req)["uid"]
		user, exists := n.FindUser(func(u core.User) bool { return u.LoginName == userLoginName })
		if exists {
			i.TargetUser = &user.User
			i.TargetRef = user.User.Ref()
		} else {
			msg := fmt.Sprintf("User %q does not exist.", userLoginName)
			i.RedirectWithFlashTo("/users", Flash{"danger", msg})
		}
	}
}

func buildUserFromFormState(fs *h.FormState, loginName, passwordHash string) (result core.User, errs errext.ErrorSet) {
	result = core.User{
		LoginName:     loginName,
		GivenName:     fs.Fields["given_name"].Value,
		FamilyName:    fs.Fields["family_name"].Value,
		EMailAddress:  fs.Fields["email"].Value,
		SSHPublicKeys: core.SplitSSHPublicKeys(fs.Fields["ssh_public_keys"].Value),
		PasswordHash:  passwordHash,
		POSIX:         nil,
	}
	if fs.Fields["posix"].IsUnfolded {
		uid, err := core.ParsePosixID(fs.Fields["posix_uid"].Value, result.Ref().Field("posix_uid"))
		errs.Add(err)
		gid, err := core.ParsePosixID(fs.Fields["posix_gid"].Value, result.Ref().Field("posix_gid"))
		errs.Add(err)

		result.POSIX = &core.UserPosixAttributes{
			UID:           uid,
			GID:           gid,
			HomeDirectory: fs.Fields["posix_home"].Value,
			LoginShell:    fs.Fields["posix_shell"].Value,
			GECOS:         fs.Fields["posix_gecos"].Value,
		}
	}
	return
}

func validateUserForm(i *Interaction) {
	fs := i.FormState
	if i.TargetUser == nil || fs.Fields["reset_password"].IsUnfolded {
		password1 := fs.Fields["password"].GetValueOrSetError()
		password2 := fs.Fields["repeat_password"].GetValueOrSetError()
		if password2 != "" && password1 != password2 {
			fs.Fields["repeat_password"].ErrorMessage = "did not match"
		}
	}
}

func executeEditUser(db *core.Database, i *Interaction, hasher crypt.PasswordHasher) errext.ErrorSet {
	passwordHash := i.TargetUser.PasswordHash
	if i.FormState.Fields["reset_password"].IsUnfolded {
		if pw := i.FormState.Fields["password"].Value; pw != "" {
			passwordHash = hasher.HashPassword(pw)
		}
	}

	newUser, errs := buildUserFromFormState(i.FormState, i.TargetUser.LoginName, passwordHash)
	errs.Add(db.Users.Update(newUser))

	isMemberOf := i.FormState.Fields["memberships"].Selected
	for idx := range db.Groups {
		group := &db.Groups[idx]
		if group.MemberLoginNames == nil {
			group.MemberLoginNames = make(map[string]bool)
		}
		group.MemberLoginNames[i.TargetUser.LoginName] = isMemberOf[group.Name]
	}
	return errs
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
		validateUserForm,
		TryUpdateNexus(n, executeCreateUser),
		ShowFormIfErrors("Create user"),
		RedirectWithFlashTo("/users", "Created"),
	)
}

func executeCreateUser(db *core.Database, i *Interaction, hasher crypt.PasswordHasher) errext.ErrorSet {
	loginName := i.FormState.Fields["login_name"].Value
	passwordHash := hasher.HashPassword(i.FormState.Fields["password"].Value)

	newUser, errs := buildUserFromFormState(i.FormState, loginName, passwordHash)
	i.TargetRef = newUser.Ref()
	db.Users = append(db.Users, newUser)

	isMemberOf := i.FormState.Fields["memberships"].Selected
	for idx := range db.Groups {
		group := &db.Groups[idx]
		if group.MemberLoginNames == nil {
			group.MemberLoginNames = make(map[string]bool)
		}
		group.MemberLoginNames[loginName] = isMemberOf[group.Name]
	}
	return errs
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
		useDeleteUserForm,
		UseEmptyFormState,
		TryUpdateNexus(n, executeDeleteUser),
		ShowFormIfErrors("Confirm user deletion"),
		RedirectWithFlashTo("/users", "Deleted"),
	)
}

func executeDeleteUser(db *core.Database, i *Interaction, _ crypt.PasswordHasher) (errs errext.ErrorSet) {
	userLoginName := i.TargetUser.LoginName
	errs.Add(db.Users.Delete(userLoginName))

	for _, group := range db.Groups {
		if group.MemberLoginNames != nil {
			group.MemberLoginNames[userLoginName] = false
		}
	}
	return
}
