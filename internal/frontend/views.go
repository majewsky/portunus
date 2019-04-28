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
	"net/http"

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

//WriteHTMLPage writes a complete HTML page into w.
func WriteHTMLPage(w http.ResponseWriter, status int, title string, bodyContents h.RenderedHTML) {
	titleText := "Portunus"
	if title != "" {
		titleText = title + " – Portunus"
	}

	headTag := h.Tag("head",
		standardHeadTags,
		h.Tag("title", h.Text(titleText)),
	)

	htmlTag := h.Tag("html",
		headTag,
		h.Tag("body", bodyContents),
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	w.Write([]byte(htmlTag.String()))
}

//NavbarItem is an item that appears in the top navbar.
type NavbarItem struct {
	URL    string
	Title  string
	Active bool
}

//RenderNavbar renders the top navbar that appears in every view.
func RenderNavbar(currentUserID string, items ...NavbarItem) h.RenderedHTML {
	fields := []h.TagArgument{
		h.Tag("li", h.Tag("h1", h.Text("Portunus"))),
	}
	for _, item := range items {
		linkArgs := []h.TagArgument{
			h.Text(item.Title), h.Attr("href", item.URL),
		}
		if item.Active {
			linkArgs = append(linkArgs, h.Attr("class", "current"))
		}
		fields = append(fields, h.Tag("li", h.Tag("a", linkArgs...)))
	}

	if currentUserID != "" {
		fields = append(fields, h.Tag("li", h.Attr("class", "spacer")))
		fields = append(fields, h.Tag("li", h.Tag("a", h.Attr("class", "current"), h.Text(currentUserID))))
		fields = append(fields, h.Tag("li", h.Tag("a",
			h.Attr("href", "/logout"),
			h.Text("Logout"),
		)))
	}

	return h.Tag("nav", h.Tag("ul", fields...))
}
