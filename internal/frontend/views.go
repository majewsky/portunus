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
