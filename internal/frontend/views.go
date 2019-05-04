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
	"encoding/gob"
	"net/http"
	"strings"

	"github.com/gorilla/sessions"
	"github.com/majewsky/portunus/internal/core"
	h "github.com/majewsky/portunus/internal/html"
)

var standardHeadTags = h.Join(
	h.Tag("meta",
		h.Attr("charset", "utf-8"),
	),
	h.Tag("meta",
		h.Attr("http-equiv", "X-UA-Compatible"),
		h.Attr("content", "IE=edge"),
	),
	h.Tag("meta",
		h.Attr("name", "viewport"),
		h.Attr("content", "width=device-width, initial-scale=1"),
	),
	h.Tag("link",
		h.Attr("rel", "stylesheet"),
		h.Attr("href", "/static/css/portunus.css"),
	),
)

type flash struct {
	Type    string
	Message string
}

func init() {
	gob.Register(flash{})
}

//RedirectWithFlash is a shortcut for redirecting from a POST action to a GET
//view with a flash message.
func RedirectWithFlash(w http.ResponseWriter, r *http.Request, s *sessions.Session, target string, f flash) {
	s.AddFlash(f)
	err := s.Save(r, w)
	if err == nil {
		http.Redirect(w, r, target, http.StatusSeeOther)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type page struct {
	Status   int
	Title    string
	Contents h.RenderedHTML
}

func (p page) Render(w http.ResponseWriter, r *http.Request, currentUser *core.UserWithPerms, s *sessions.Session) {
	//prepare <head>
	titleText := "Portunus"
	if p.Title != "" {
		titleText = p.Title + " – Portunus"
	}
	headTag := h.Tag("head",
		standardHeadTags,
		h.Tag("title", h.Text(titleText)),
	)

	//prepare <nav>
	navFields := []h.TagArgument{
		h.Tag("li", h.Tag("h1", h.Text("Portunus"))),
	}
	addNavField := func(url string, title string) {
		linkArgs := []h.TagArgument{
			h.Text(title), h.Attr("href", url),
		}
		if strings.HasPrefix(r.URL.Path, url) {
			linkArgs = append(linkArgs, h.Attr("class", "current"))
		}
		navFields = append(navFields, h.Tag("li", h.Tag("a", linkArgs...)))
	}
	if currentUser == nil {
		addNavField("/login", "Login")
	} else {
		addNavField("/self", "My profile")
		if currentUser.Perms.Portunus.IsAdmin {
			addNavField("/users", "Users")
			addNavField("/groups", "Groups")
		}
		navFields = append(navFields, h.Tag("li", h.Attr("class", "spacer")))
		navFields = append(navFields, h.Tag("li",
			h.Tag("a", h.Attr("class", "current"), h.Text(currentUser.FullName())),
		))
		addNavField("/logout", "Logout")
	}

	//prepare flashes, if any
	var flashes []h.RenderedHTML
	for _, value := range s.Flashes() {
		if f, ok := value.(flash); ok {
			flashes = append(flashes, h.Tag("div",
				h.Attr("class", "flash flash-"+f.Type),
				h.Text(f.Message),
			))
		}
	}
	err := s.Save(r, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	//render final HTML
	htmlTag := h.Tag("html",
		headTag,
		h.Tag("body",
			h.Tag("nav", h.Tag("ul", navFields...)),
			h.Tag("main",
				h.Join(flashes...),
				p.Contents,
			),
		),
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(p.Status)
	w.Write([]byte(htmlTag.String()))
}

//RenderGroupMemberships renders a list of all groups the given user is part of.
func RenderGroupMemberships(user core.User, groups []core.Group, currentUser core.UserWithPerms) h.RenderedHTML {
	isAdmin := currentUser.Perms.Portunus.IsAdmin
	var groupMemberships []h.RenderedHTML
	for _, group := range groups {
		if !group.ContainsUser(user) {
			continue
		}
		if len(groupMemberships) > 0 {
			groupMemberships = append(groupMemberships, h.Text(", "))
		}
		if isAdmin {
			groupMemberships = append(groupMemberships, h.Tag("a",
				h.Attr("href", "/groups/"+group.Name+"/edit"),
				h.Text(group.LongName),
			))
		} else {
			groupMemberships = append(groupMemberships, h.Text(group.Name))
		}
	}
	return h.Join(groupMemberships...)
}
