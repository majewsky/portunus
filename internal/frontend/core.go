/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package frontend

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/majewsky/portunus/internal/core"
	"github.com/majewsky/portunus/internal/crypt"
	h "github.com/majewsky/portunus/internal/html"
	"github.com/majewsky/portunus/static"
	"github.com/sapcc/go-bits/errext"
	"github.com/sapcc/go-bits/logg"
)

// HTTPHandler returns the main http.Handler.
func HTTPHandler(nexus core.Nexus, isBehindTLSProxy bool) http.Handler {
	r := mux.NewRouter()
	r.Methods("GET").Path(`/`).Handler(getToplevelHandler(nexus))
	r.Methods("GET").Path(`/static/{path:.+}`).Handler(http.StripPrefix("/static/", http.FileServer(http.FS(static.FS))))

	r.Methods("GET").Path(`/login`).Handler(getLoginHandler(nexus))
	r.Methods("POST").Path(`/login`).Handler(postLoginHandler(nexus))
	r.Methods("GET").Path(`/logout`).Handler(getLogoutHandler(nexus))

	r.Methods("GET").Path(`/self`).Handler(getSelfHandler(nexus))
	r.Methods("POST").Path(`/self`).Handler(postSelfHandler(nexus))

	r.Methods("GET").Path(`/users`).Handler(getUsersHandler(nexus))
	r.Methods("GET").Path(`/users/new`).Handler(getUsersNewHandler(nexus))
	r.Methods("POST").Path(`/users/new`).Handler(postUsersNewHandler(nexus))
	r.Methods("GET").Path(`/users/{uid}/edit`).Handler(getUserEditHandler(nexus))
	r.Methods("POST").Path(`/users/{uid}/edit`).Handler(postUserEditHandler(nexus))
	r.Methods("GET").Path(`/users/{uid}/delete`).Handler(getUserDeleteHandler(nexus))
	r.Methods("POST").Path(`/users/{uid}/delete`).Handler(postUserDeleteHandler(nexus))

	r.Methods("GET").Path(`/groups`).Handler(getGroupsHandler(nexus))
	r.Methods("GET").Path(`/groups/new`).Handler(getGroupsNewHandler(nexus))
	r.Methods("POST").Path(`/groups/new`).Handler(postGroupsNewHandler(nexus))
	r.Methods("GET").Path(`/groups/{name}/edit`).Handler(getGroupEditHandler(nexus))
	r.Methods("POST").Path(`/groups/{name}/edit`).Handler(postGroupEditHandler(nexus))
	r.Methods("GET").Path(`/groups/{name}/delete`).Handler(getGroupDeleteHandler(nexus))
	r.Methods("POST").Path(`/groups/{name}/delete`).Handler(postGroupDeleteHandler(nexus))

	//add various security headers/checks via middleware
	handler := http.NewCrossOriginProtection().Handler(r)
	handler = securityHeadersMiddleware(handler)

	// if not on TLS (i.e. development setup that directly uses http://localhost:8080 or such),
	// then we need to not set "Secure=true" on our session cookie
	sessionStore.Options.Secure = false

	return handler
}

func securityHeadersMiddleware(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := w.Header()
		hdr.Set("X-Frame-Options", "SAMEORIGIN")
		hdr.Set("X-XSS-Protection", "1; mode=block")
		hdr.Set("X-Content-Type-Options", "nosniff")
		hdr.Set("Referrer-Policy", "strict-origin")
		hdr.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:;")
		inner.ServeHTTP(w, r)
	})
}

func getToplevelHandler(n core.Nexus) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(n),
		RedirectTo("/self"),
	)
}

////////////////////////////////////////////////////////////////////////////////
// type Handler

// Handler allows to construct HTTP handlers by chained method calls describing
// the sequence of actions taken.
type Handler struct {
	steps []HandlerStep
}

// Do creates a Handler with the given steps.
func Do(steps ...HandlerStep) Handler {
	return Handler{steps: steps}
}

// ServeHTTP implements the http.Handler interface.
func (hh Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	i := Interaction{
		Req:    r,
		writer: w,
	}
	for _, step := range hh.steps {
		step(&i)
		if i.writer == nil {
			return
		}
	}
}

// HandlerStep is a single step executed by a handler. When a handler step
// renders a result, it shall set i.Writer = nil to ensure that the remaining
// steps do not get executed.
type HandlerStep func(i *Interaction)

// Interaction describes a single invocation of Handler.ServeHTTP().
type Interaction struct {
	Req    *http.Request
	writer http.ResponseWriter
	//Slots for data associated with a request, which may be stored by one step
	//and then used by later steps.
	Session     *sessions.Session
	CurrentUser *core.UserWithPerms
	FormSpec    *h.FormSpec
	FormState   *h.FormState
	TargetUser  *core.User     //only used by CRUD views editing a single user
	TargetGroup *core.Group    //only used by CRUD views editing a single group
	TargetRef   core.ObjectRef //refers to TargetGroup/TargetUser (for admin forms) or CurrentUser (for selfservice forms)
}

// WriteError wraps http.Error().
func (i *Interaction) WriteError(msg string, code int) {
	http.Error(i.writer, msg, code)
	i.writer = nil
}

// RedirectTo redirects to the given URL.
func (i *Interaction) RedirectTo(url string) {
	http.Redirect(i.writer, i.Req, url, http.StatusSeeOther)
	i.writer = nil
}

// RedirectWithFlashTo is like RedirectTo, but stores a flash to show on the next page.
func (i *Interaction) RedirectWithFlashTo(url string, f Flash) {
	i.Session.AddFlash(f)
	if i.SaveSession() {
		i.RedirectTo(url)
	}
}

// SaveSession calls i.Session.Save(). When false is returned, a 500 error was
// written and the calling handler step shall abort immediately.
func (i *Interaction) SaveSession() bool {
	err := i.Session.Save(i.Req, i.writer)
	if err != nil {
		i.WriteError(err.Error(), http.StatusInternalServerError)
		return false
	}
	return true
}

////////////////////////////////////////////////////////////////////////////////
// standard handler steps

// ShowView is a final handler step that uses a callback to render the requested
// page.
func ShowView(view func(i *Interaction) Page) HandlerStep {
	return func(i *Interaction) {
		if i.Session == nil {
			panic("ShowView must come after LoadSession")
		}
		view(i).Render(i.writer, i.Req, i.CurrentUser, i.Session)
		i.writer = nil
	}
}

// RedirectTo is a handler step that always redirects to the given URL.
func RedirectTo(url string) HandlerStep {
	return func(i *Interaction) {
		i.RedirectTo(url)
	}
}

// RedirectWithFlashTo is like RedirectTo, but adds a flash
// describing a successful CRUD operation.
func RedirectWithFlashTo(url, action string) HandlerStep {
	return func(i *Interaction) {
		ref := i.TargetRef
		msg := fmt.Sprintf("%s %s %q.", action, ref.Type, ref.Name)
		i.RedirectWithFlashTo(url, Flash{"success", msg})
	}
}

var sessionStore *sessions.CookieStore

func init() {
	keyPath := filepath.Join(os.Getenv("PORTUNUS_SERVER_STATE_DIR"), "session-key.dat")
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		keyBytes = nil
		if !os.IsNotExist(err) {
			logg.Error(err.Error())
		}
	}

	if len(keyBytes) != 32 {
		logg.Info("generating new session key")
		keyBytes = core.GenerateRandomKey(32)
		err := os.WriteFile(keyPath, keyBytes, 0600)
		if err != nil {
			logg.Error(err.Error())
		}
	}

	sessionStore = sessions.NewCookieStore(keyBytes)
	sessionStore.Options.SameSite = http.SameSiteStrictMode
}

// LoadSession is a handler step that loads the session or starts a new one if
// there is no valid session.
func LoadSession(i *Interaction) {
	var err error
	i.Session, err = sessionStore.Get(i.Req, "portunus-login")
	if err != nil {
		//the session is broken - start a fresh one
		logg.Error("could not decode user session cookie: " + err.Error())
		i.Req.Header.Del("Cookie")
		i.Session, err = sessionStore.New(i.Req, "portunus-login")
		if err != nil {
			i.WriteError(err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if i.Session == nil {
		i.WriteError("unexpected empty session", http.StatusInternalServerError)
	}
}

// SaveSession is a handler step that calls Interaction.SaveSession.
func SaveSession(i *Interaction) {
	i.SaveSession()
}

// VerifyLogin is a handler step that checks the current session for a valid
// login, and redirects to /login if it cannot find one.
func VerifyLogin(n core.Nexus) HandlerStep {
	return func(i *Interaction) {
		if i.Session == nil {
			panic("VerifyLogin must come after LoadSession")
		}
		uid, ok := i.Session.Values["uid"].(string)
		if !ok {
			i.RedirectTo("/login")
			return
		}
		user, ok := n.FindUser(func(u core.User) bool { return u.LoginName == uid })
		if ok {
			i.CurrentUser = &user
		} else {
			i.RedirectTo("/login")
		}
	}
}

// VerifyPermissions is a handler step that checks whether the current user has
// at least the given permissions.
func VerifyPermissions(perms core.Permissions) HandlerStep {
	return func(i *Interaction) {
		if i.CurrentUser == nil {
			panic("VerifyPermissions must come after VerifyLogin")
		}
		if !i.CurrentUser.Perms.Includes(perms) {
			i.WriteError("Forbidden", http.StatusForbidden)
			return
		}
	}
}

// UseEmptyFormState is a handler step that initializes an empty i.FormState.
func UseEmptyFormState(i *Interaction) {
	i.FormState = &h.FormState{}
}

// ReadFormStateFromRequest is a handler step that initializes i.FormState from
// the incoming request. This is only suitable for POST handlers.
func ReadFormStateFromRequest(i *Interaction) {
	if i.FormSpec == nil {
		panic("ReadFormStateFromRequest requires a form to be selected")
	}
	if i.FormState == nil {
		i.FormState = &h.FormState{}
	}
	err := i.FormSpec.ReadState(i.Req, i.FormState)
	if err != nil {
		i.RedirectWithFlashTo("/self", Flash{"danger", "could not parse form submitted to POST /login: " + err.Error()})
	}
}

// ShowForm is a final handler step that renders i.FormSpec with i.FormState.
func ShowForm(title string) HandlerStep {
	return func(i *Interaction) {
		if i.Session == nil {
			panic("ShowForm must come after LoadSession")
		}
		if i.FormSpec == nil {
			panic("ShowForm requires a form to be selected")
		}
		if i.FormState == nil {
			panic("ShowForm requires a form state")
		}
		Page{
			Status:   http.StatusOK,
			Title:    title,
			Contents: i.FormSpec.Render(i.Req, *i.FormState),
		}.Render(i.writer, i.Req, i.CurrentUser, i.Session)
		i.writer = nil
	}
}

// ShowFormIfErrors is like ShowForm, but only renders an output if i.FormState
// is not valid.
func ShowFormIfErrors(title string) HandlerStep {
	return func(i *Interaction) {
		if i.FormState == nil {
			panic("ShowFormIfErrors requires a form state")
		}
		if !i.FormState.IsValid() {
			ShowForm(title)(i)
		}
	}
}

// TryUpdateNexus is a handler step that calls nexus.Update() if the FormState
// does not have any errors yet, and pushes all errors into the FormState.
func TryUpdateNexus(n core.Nexus, action func(*core.Database, *Interaction, crypt.PasswordHasher) errext.ErrorSet) HandlerStep {
	return func(i *Interaction) {
		opts := core.UpdateOptions{
			ConflictWithSeedIsError: true,
			DryRun:                  !i.FormState.IsValid(),
		}
		errs := n.Update(func(db *core.Database) errext.ErrorSet {
			return action(db, i, n.PasswordHasher())
		}, &opts)
		i.FormState.FillErrorsFrom(errs, i.TargetRef)
	}
}
