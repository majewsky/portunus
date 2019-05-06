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
	"bytes"
	"net/http"
	"path"

	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/majewsky/portunus/internal/core"
	h "github.com/majewsky/portunus/internal/html"
	"github.com/majewsky/portunus/internal/static"
	"github.com/sapcc/go-bits/logg"
)

//HTTPHandler returns the main http.Handler.
func HTTPHandler(engine core.Engine, isBehindTLSProxy bool) http.Handler {
	r := mux.NewRouter()
	r.Methods("GET").Path(`/`).Handler(getToplevelHandler(engine))
	r.Methods("GET").Path(`/static/{path:.+}`).HandlerFunc(staticHandler)

	r.Methods("GET").Path(`/login`).Handler(getLoginHandler(engine))
	r.Methods("POST").Path(`/login`).Handler(postLoginHandler(engine))
	r.Methods("GET").Path(`/logout`).Handler(getLogoutHandler(engine))

	r.Methods("GET").Path(`/self`).Handler(getSelfHandler(engine))
	r.Methods("POST").Path(`/self`).Handler(postSelfHandler(engine))

	r.Methods("GET").Path(`/users`).Handler(getUsersHandler(engine))
	r.Methods("GET").Path(`/users/new`).Handler(getUsersNewHandler(engine))
	r.Methods("POST").Path(`/users/new`).Handler(postUsersNewHandler(engine))
	r.Methods("GET").Path(`/users/{uid}/edit`).Handler(getUserEditHandler(engine))
	r.Methods("POST").Path(`/users/{uid}/edit`).Handler(postUserEditHandler(engine))
	r.Methods("GET").Path(`/users/{uid}/delete`).Handler(getUserDeleteHandler(engine))
	r.Methods("POST").Path(`/users/{uid}/delete`).Handler(postUserDeleteHandler(engine))

	r.Methods("GET").Path(`/groups`).Handler(getGroupsHandler(engine))
	r.Methods("GET").Path(`/groups/new`).Handler(getGroupsNewHandler(engine))
	r.Methods("POST").Path(`/groups/new`).Handler(postGroupsNewHandler(engine))
	r.Methods("GET").Path(`/groups/{name}/edit`).Handler(getGroupEditHandler(engine))
	r.Methods("POST").Path(`/groups/{name}/edit`).Handler(postGroupEditHandler(engine))
	r.Methods("GET").Path(`/groups/{name}/delete`).Handler(getGroupDeleteHandler(engine))
	r.Methods("POST").Path(`/groups/{name}/delete`).Handler(postGroupDeleteHandler(engine))

	//setup CSRF with maxAge = 30 minutes
	csrfKey := securecookie.GenerateRandomKey(32)
	csrfMiddleware := csrf.Protect(csrfKey, csrf.MaxAge(1800), csrf.Secure(isBehindTLSProxy))
	handler := csrfMiddleware(r)

	//TODO: add middleware for security headers

	return handler
}

func staticHandler(w http.ResponseWriter, r *http.Request) {
	assetPath := mux.Vars(r)["path"]
	assetBytes, err := static.Asset(assetPath)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	assetInfo, err := static.AssetInfo(assetPath)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	http.ServeContent(w, r, path.Base(assetPath), assetInfo.ModTime(), bytes.NewReader(assetBytes))
}

func getToplevelHandler(e core.Engine) http.Handler {
	return Do(
		LoadSession,
		VerifyLogin(e),
		RedirectTo("/self"),
	)
}

////////////////////////////////////////////////////////////////////////////////
// type Handler

//Handler allows to construct HTTP handlers by chained method calls describing
//the sequence of actions taken.
type Handler struct {
	steps []HandlerStep
}

//Do creates a Handler with the given steps.
func Do(steps ...HandlerStep) Handler {
	return Handler{steps: steps}
}

//ServeHTTP implements the http.Handler interface.
func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	i := Interaction{
		Req:    r,
		writer: w,
	}
	for _, step := range h.steps {
		step(&i)
		if i.writer == nil {
			return
		}
	}
}

//HandlerStep is a single step executed by a handler. When a handler step
//renders a result, it shall set i.Writer = nil to ensure that the remaining
//steps do not get executed.
type HandlerStep func(i *Interaction)

//Interaction describes a single invocation of Handler.ServeHTTP().
type Interaction struct {
	Req    *http.Request
	writer http.ResponseWriter
	//Slots for data associated with a request, which may be stored by one step
	//and then used by later steps.
	Session     *sessions.Session
	CurrentUser *core.UserWithPerms
	FormSpec    *h.FormSpec
	FormState   *h.FormState
	TargetUser  *core.User  //only used by CRUD views editing a single user
	TargetGroup *core.Group //only used by CRUD views editing a single group
}

//WriteError wraps http.Error().
func (i *Interaction) WriteError(msg string, code int) {
	http.Error(i.writer, msg, code)
	i.writer = nil
}

//RedirectTo redirects to the given URL.
func (i *Interaction) RedirectTo(url string) {
	http.Redirect(i.writer, i.Req, url, http.StatusSeeOther)
	i.writer = nil
}

//RedirectWithFlashTo is like RedirectTo, but stores a flash to show on the next page.
func (i *Interaction) RedirectWithFlashTo(url string, f Flash) {
	i.Session.AddFlash(f)
	if i.SaveSession() {
		i.RedirectTo(url)
	}
}

//SaveSession calls i.Session.Save(). When false is returned, a 500 error was
//written and the calling handler step shall abort immediately.
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

//ShowView is a final handler step that uses a callback to render the requested
//page.
func ShowView(view func(i *Interaction) Page) HandlerStep {
	return func(i *Interaction) {
		if i.Session == nil {
			panic("ShowView must come after LoadSession")
		}
		view(i).Render(i.writer, i.Req, i.CurrentUser, i.Session)
	}
}

//RedirectTo is a handler step that always redirects to the given URL.
func RedirectTo(url string) HandlerStep {
	return func(i *Interaction) {
		i.RedirectTo(url)
	}
}

//TODO persist session key
var sessionStore = sessions.NewCookieStore(securecookie.GenerateRandomKey(32))

//LoadSession is a handler step that loads the session or starts a new one if
//there is no valid session.
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

//SaveSession is a handler step that calls Interaction.SaveSession.
func SaveSession(i *Interaction) {
	i.SaveSession()
}

//VerifyLogin is a handler step that checks the current session for a valid
//login, and redirects to /login if it cannot find one.
func VerifyLogin(e core.Engine) HandlerStep {
	return func(i *Interaction) {
		if i.Session == nil {
			panic("VerifyLogin must come after LoadSession")
		}
		uid, ok := i.Session.Values["uid"].(string)
		if !ok {
			i.RedirectTo("/login")
			return
		}
		i.CurrentUser = e.FindUser(uid)
		if !ok {
			i.RedirectTo("/login")
			return
		}
	}
}

//VerifyPermissions is a handler step that checks whether the current user has
//at least the given permissions.
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

//UseEmptyFormState is a handler step that initializes an empty i.FormState.
func UseEmptyFormState(i *Interaction) {
	i.FormState = &h.FormState{}
}

//ReadFormStateFromRequest is a handler step that initializes i.FormState from
//the incoming request. This is only suitable for POST handlers.
func ReadFormStateFromRequest(i *Interaction) {
	if i.FormSpec == nil {
		panic("ReadFormStateFromRequest requires a form to be selected")
	}
	if i.FormState == nil {
		i.FormState = &h.FormState{}
	}
	i.FormSpec.ReadState(i.Req, i.FormState)
}

//ShowForm is a final handler step that renders i.FormSpec with i.FormState.
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
	}
}

//ShowFormIfErrors is like ShowForm, but only renders an output if i.FormState
//is not valid.
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
