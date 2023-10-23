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
	h "github.com/majewsky/portunus/internal/html"
	"github.com/sapcc/go-bits/errext"
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
				"memberships": {
					Selected: isSelected,
				},
				"ssh_public_keys": {
					Value: strings.Join(user.SSHPublicKeys, "\r\n"),
				},
			},
		}

		i.FormSpec = &h.FormSpec{
			PostTarget:  "/self",
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
					Name:     "memberships",
					Label:    "Group memberships",
					Options:  memberships,
					ReadOnly: true,
				},
				h.MultilineInputFieldSpec{
					Name:  "ssh_public_keys",
					Label: "SSH public key(s)",
					Rules: []h.ValidationRule{
						core.MustNotHaveSurroundingSpaces,
						core.MustBeSSHPublicKeys,
					},
				},
				h.FieldSet{
					Name:       "change_password",
					Label:      "Change password",
					IsFoldable: true,
					Fields: []h.FormField{
						h.InputFieldSpec{
							InputType: "password",
							Name:      "old_password",
							Label:     "Old password",
							Rules: []h.ValidationRule{
								core.MustNotBeEmpty,
							},
						},
						h.InputFieldSpec{
							InputType: "password",
							Name:      "new_password",
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
		ShowFormIfErrors("My profile"),
		executeSelfServiceForm(n),
		ShowForm("My profile"),
	)
}

func validateSelfServiceForm(n core.Nexus) HandlerStep {
	return func(i *Interaction) {
		fs := i.FormState

		if fs.Fields["change_password"].IsUnfolded {
			if fs.IsValid() {
				newPassword1 := fs.Fields["new_password"].Value
				newPassword2 := fs.Fields["repeat_password"].Value
				if newPassword1 != newPassword2 {
					fs.Fields["repeat_password"].ErrorMessage = "did not match"
				}
			}

			if fs.IsValid() {
				oldPassword := fs.Fields["old_password"].Value
				if !n.PasswordHasher().CheckPasswordHash(oldPassword, i.CurrentUser.PasswordHash) {
					fs.Fields["old_password"].ErrorMessage = "is not correct"
				}
			}
		}
	}
}

func executeSelfServiceForm(n core.Nexus) HandlerStep {
	return func(i *Interaction) {
		fs := i.FormState
		errs := n.Update(func(db *core.Database) (errs errext.ErrorSet) {
			user, exists := db.Users.Find(func(u core.User) bool { return u.LoginName == i.CurrentUser.LoginName })
			if !exists {
				errs.Addf("no such user")
				return
			}

			if fs.Fields["change_password"].IsUnfolded {
				user.PasswordHash = n.PasswordHasher().HashPassword(fs.Fields["new_password"].Value)
			}
			user.SSHPublicKeys = core.SplitSSHPublicKeys(fs.Fields["ssh_public_keys"].Value)
			errs.Add(db.Users.Update(user))
			return
		}, interactiveUpdate)

		if errs.IsEmpty() {
			i.Session.AddFlash(Flash{"success", "Profile updated."})
		} else {
			i.Session.AddFlash(Flash{"danger", errs.Join(", ")})
		}
	}
}
