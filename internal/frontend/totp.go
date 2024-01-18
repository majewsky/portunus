/*******************************************************************************
* Copyright 2024 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package frontend

import (
	"bytes"
	"fmt"
	"image/png"
	"net/http"
	"strconv"

	"github.com/majewsky/portunus/internal/core"
	"github.com/majewsky/portunus/internal/crypt"
	h "github.com/majewsky/portunus/internal/html"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/sapcc/go-bits/errext"
	"github.com/sapcc/go-bits/logg"
)

const (
	configureTotp = "configure_totp"
	totpToken     = "totp_token"
)

var plainSnippet = h.NewSnippet(`
	{{.}}
`)

func useSelfServiceTOTPForm(n core.Nexus, generateSecret bool) HandlerStep {
	return func(i *Interaction) {
		user := i.CurrentUser
		i.TargetRef = user.Ref()

		var (
			errs   errext.ErrorSet
			key    *otp.Key
			qrCode bytes.Buffer
			err    error // otherwise we shadow the return value of key by totp.Generate()
		)

		// show the TOTP secret if the user didn't enable TOTP before
		if !user.TOTPEnabled {
			if generateSecret {
				key, err = totp.Generate(totp.GenerateOpts{
					Issuer:      n.WebDomain(),
					AccountName: user.LoginName,
				})
				if err != nil {
					errs.Add(fmt.Errorf("Failed to generate totp key: %e", err))
				}

				// only save the secret if we are not juts validating a form
				i.CurrentUser.TOTPKeyURL = key.String()
			} else {
				key, err = otp.NewKeyFromURL(user.TOTPKeyURL)
				if err != nil {
					errs.Add(fmt.Errorf("Failed to parse totp key: %e", err))
				}
			}

			img, err := key.Image(200, 200)
			if err != nil {
				errs.Add(fmt.Errorf("Failed to generate qr code: %e", err))
			}

			err = png.Encode(&qrCode, img)
			if err != nil {
				errs.Add(fmt.Errorf("Failed to encode qr code: %e", err))
			}

			i.CurrentUser.TOTPKey = key
			i.FormState.FillErrorsFrom(errs, i.TargetRef)
		}

		label := "Disable"
		totpFields := []h.FormField{}
		if !user.TOTPEnabled {
			label = "Enable"
			totpFields = []h.FormField{
				h.ImgData{
					Alt:   "QR code for TOTP enrollment",
					Image: qrCode.Bytes(),
					Label: "QR code",
					Name:  "qrcode",
				},
				h.StaticField{
					Label: "Algorithm",
					Value: plainSnippet.Render(key.Algorithm()),
				},
				h.StaticField{
					Label: "Digits",
					Value: plainSnippet.Render(key.Digits()),
				},
				h.StaticField{
					Label: "Period",
					Value: plainSnippet.Render(key.Period()),
				},
				h.StaticField{
					Label: "Secret",
					Value: plainSnippet.Render(key.Secret()),
				},
				h.InputFieldSpec{
					InputType:        "text",
					Name:             totpToken,
					Label:            "Token",
					AutocompleteMode: "one-time-code",
					Pattern:          "[0-9]*",
				},
			}
		}

		i.FormState = &h.FormState{}

		i.FormSpec = &h.FormSpec{
			PostTarget:  "/self/totp",
			SubmitLabel: label,
			Warning:     user.TOTPEnabled,
			Fields: []h.FormField{
				h.FieldSet{
					Name:   configureTotp,
					Label:  "Configure TOTP",
					Fields: totpFields,
				},
			},
		}
	}
}

func getSelfTOTPHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		useSelfServiceTOTPForm(n, true),
		TryUpdateNexus(n, executeSelfServiceTOTP),
		ShowForm("Configure 2FA"),
	)
}

func postSelfTOTPHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		useSelfServiceTOTPForm(n, false),
		ReadFormStateFromRequest,
		validateSelfServiceTOTPForm(),
		TryUpdateNexus(n, executeSelfServiceTOTP),
		ShowFormIfErrors("Configure 2FA"),
		RedirectWithFlashTo("/self", "Changed 2FA for"),
	)
}

func validateSelfServiceTOTPForm() HandlerStep {
	return func(i *Interaction) {
		logg.Info("%#v", i)
		fs := i.FormState
		user := i.CurrentUser
		tokenField := fs.Fields[totpToken]

		if tokenField == nil || tokenField.Value == "" {
			user.TOTPEnabled = false
			user.TOTPClear = true
			return
		}

		_, err := strconv.Atoi(tokenField.Value)
		if len(tokenField.Value) != 6 || err != nil {
			fs.Fields[totpToken].ErrorMessage = "has incorrect format"
		}

		if !totp.Validate(tokenField.Value, user.TOTPKey.Secret()) {
			fs.Fields[totpToken].ErrorMessage = "is not correct"
		}

		user.TOTPEnabled = true
	}
}

func executeSelfServiceTOTP(db *core.Database, i *Interaction, _ crypt.PasswordHasher) (errs errext.ErrorSet) {
	for idx, user := range db.Users {
		if user.LoginName != i.CurrentUser.LoginName {
			continue
		}

		// always save the TOTPKey regardless of if it is enabled, to persist the key after showing the selfservice page
		if i.CurrentUser.TOTPKeyURL != "" {
			user.TOTPKeyURL = i.CurrentUser.TOTPKeyURL
		}

		if i.CurrentUser.TOTPClear {
			user.TOTPKeyURL = ""
		}

		user.TOTPEnabled = i.CurrentUser.TOTPEnabled
		db.Users[idx] = user //`user` copies by value, so we need to write the changes back explicitly
	}
	return
}
