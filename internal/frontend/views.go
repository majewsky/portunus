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

//Flash is a flash message.
type Flash struct {
	Type    string //either "error" or "success"
	Message string
}

func init() {
	gob.Register(Flash{})
}

//Page describes a HTML page produced by Portunus.
type Page struct {
	Status   int
	Title    string
	Contents h.RenderedHTML
	Wide     bool
}

//Render renders the given page.
func (p Page) Render(w http.ResponseWriter, r *http.Request, currentUser *core.UserWithPerms, s *sessions.Session) {
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
		if f, ok := value.(Flash); ok {
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
	mainCSSClass := ""
	if p.Wide {
		mainCSSClass = "wide"
	}
	htmlTag := h.Tag("html",
		headTag,
		h.Tag("body",
			h.Tag("nav", h.Tag("ul", navFields...)),
			h.Tag("main",
				h.Attr("class", mainCSSClass),
				h.Join(flashes...),
				p.Contents,
			),
		),
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(p.Status)
	w.Write([]byte(htmlTag.String()))
}
