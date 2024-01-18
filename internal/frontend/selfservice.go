/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package frontend

import (
	"net/http"
	"sort"
	"strings"

	"github.com/majewsky/portunus/internal/core"
	"github.com/majewsky/portunus/internal/crypt"
	h "github.com/majewsky/portunus/internal/html"
	"github.com/sapcc/go-bits/errext"
)

const (
	Memberships    = "memberships"
	changePassword = "change_password"
	oldPassword    = "old_password"
	newPassword    = "new_password"
	repeatPassword = "repeat_password"
	sshPublicKeys  = "ssh_public_keys"
)

// TODO: allow flipped order (family name first)
var userFullNameSnippet = h.NewSnippet(`
	<span class="given-name">{{.GivenName}}</span> <span class="family-name">{{.FamilyName}}</span>
`)
var userEMailAddressSnippet = h.NewSnippet(`
	{{if .EMailAddress}}{{.EMailAddress}}{{else}}<em>Not specified</em>{{end}}
`)

func useSelfServiceForm(n core.Nexus) HandlerStep {
	return func(i *Interaction) {
		user := i.CurrentUser
		i.TargetRef = user.Ref()

		isAdmin := user.Perms.Portunus.IsAdmin
		visibleGroups := user.GroupMemberships
		if isAdmin {
			visibleGroups = n.ListGroups()
		}
		sort.Slice(visibleGroups, func(i, j int) bool {
			return visibleGroups[i].LongName < visibleGroups[j].LongName
		})

		var memberships []h.SelectOptionSpec
		isSelected := make(map[string]bool)
		for _, group := range visibleGroups {
			membership := h.SelectOptionSpec{
				Value: group.Name,
				Label: group.LongName,
			}
			memberships = append(memberships, membership)
			isSelected[group.Name] = group.ContainsUser(user.User)
		}

		i.FormState = &h.FormState{
			Fields: map[string]*h.FieldState{
				Memberships: {
					Selected: isSelected,
				},
				sshPublicKeys: {
					Value: strings.Join(user.SSHPublicKeys, "\r\n"),
				},
			},
		}

		i.FormSpec = &h.FormSpec{
			PostTarget: "/self",
			Links: []h.Link{{
				DisplayName: "Configure TOTP",
				Target:      "/self/totp",
			}},
			SubmitLabel: "Update profile",
			Fields: []h.FormField{
				h.StaticField{
					Label: "Login name",
					Value: codeTagSnippet.Render(user.LoginName),
				},
				h.StaticField{
					Label: "Full name",
					Value: userFullNameSnippet.Render(user),
				},
				h.StaticField{
					Label: "Email address",
					Value: userEMailAddressSnippet.Render(user),
				},
				h.SelectFieldSpec{
					Name:     Memberships,
					Label:    "Group memberships",
					Options:  memberships,
					ReadOnly: true,
				},
				h.MultilineInputFieldSpec{
					Name:  sshPublicKeys,
					Label: "SSH public key(s)",
				},
				h.FieldSet{
					Name:       changePassword,
					Label:      "Change password",
					IsFoldable: true,
					Fields: []h.FormField{
						h.InputFieldSpec{
							InputType: "password",
							Name:      oldPassword,
							Label:     "Old password",
						},
						h.InputFieldSpec{
							InputType: "password",
							Name:      newPassword,
							Label:     "New password",
						},
						h.InputFieldSpec{
							InputType: "password",
							Name:      repeatPassword,
							Label:     "Repeat password",
						},
					},
				},
			},
		}
	}
}

func getSelfHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		useSelfServiceForm(n),
		ShowForm("My profile"),
	)
}

func postSelfHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		useSelfServiceForm(n),
		ReadFormStateFromRequest,
		validateSelfServiceForm(n),
		TryUpdateNexus(n, executeSelfService),
		ShowFormIfErrors("My profile"),
		RedirectWithFlashTo("/self", "Updated"),
	)
}

func validateSelfServiceForm(n core.Nexus) HandlerStep {
	return func(i *Interaction) {
		fs := i.FormState
		user := i.CurrentUser

		if fs.Fields[changePassword].IsUnfolded {
			oldPassword := fs.Fields[oldPassword].GetValueOrSetError()
			if oldPassword != "" && !n.PasswordHasher().CheckPasswordHash(oldPassword, user.PasswordHash) {
				fs.Fields[oldPassword].ErrorMessage = "is not correct"
			}

			newPassword1 := fs.Fields[newPassword].GetValueOrSetError()
			newPassword2 := fs.Fields[repeatPassword].GetValueOrSetError()
			if newPassword2 != "" && newPassword1 != newPassword2 {
				fs.Fields[repeatPassword].ErrorMessage = "did not match"
			}
		}
	}
}

func executeSelfService(db *core.Database, i *Interaction, hasher crypt.PasswordHasher) (errs errext.ErrorSet) {
	fs := i.FormState
	for idx, user := range db.Users {
		if user.LoginName != i.CurrentUser.LoginName {
			continue
		}
		if fs.Fields["change_password"].IsUnfolded {
			user.PasswordHash = hasher.HashPassword(fs.Fields["new_password"].Value)
		}
		user.SSHPublicKeys = core.SplitSSHPublicKeys(fs.Fields[sshPublicKeys].Value)
		db.Users[idx] = user //`user` copies by value, so we need to write the changes back explicitly
	}
	return
}
