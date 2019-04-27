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
	"fmt"
	"html"
	"net/http"
	"path"

	"github.com/gorilla/mux"
	"github.com/majewsky/portunus/internal/static"
)

//HTTPHandler returns the main http.Handler.
func HTTPHandler() http.Handler {
	r := mux.NewRouter()
	r.Methods("GET").Path(`/static/{path:.+}`).HandlerFunc(staticHandler)
	r.Methods("GET").Path(`/`).HandlerFunc(entryHandler)
	return r
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

func entryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(
		tag("html",
			tag("head",
				tag("link",
					attr("rel", "stylesheet"),
					attr("href", "/static/css/portunus.css"),
				),
			),
			tag("body",
				tag("nav",
					tag("ul",
						tag("li", tag("h1", text("Portunus"))),
						tag("li", tag("a", attr("href", "#"), attr("class", "current"), text("Users"))),
						tag("li", tag("a", attr("href", "#"), text("Groups"))),
					),
				),
				tag("main",
					tag("table",
						tag("thead",
							tag("tr",
								tag("th", text("User ID")),
								tag("th", text("Name")),
								tag("th", text("Groups")),
								tag("th", attr("class", "actions"),
									tag("a",
										attr("href", "#"),
										attr("class", "btn btn-primary"),
										text("New user"),
									),
								),
							),
						),
						tag("tbody",
							tag("tr",
								tag("td", text("jane")),
								tag("td", text("Jane Doe")),
								tag("td",
									tag("a", attr("href", "#"), text("Administrators")),
									text(", "),
									tag("a", attr("href", "#"), text("Users")),
								),
								tag("td", attr("class", "actions"),
									tag("a", attr("href", "#"), text("Edit")),
									text(" · "),
									tag("a", attr("href", "#"), text("Delete")),
								),
							),
							tag("tr",
								tag("td", text("john")),
								tag("td", text("John Doe")),
								tag("td",
									tag("a", attr("href", "#"), text("Users")),
								),
								tag("td", attr("class", "actions"),
									tag("a", attr("href", "#"), text("Edit")),
									text(" · "),
									tag("a", attr("href", "#"), text("Delete")),
								),
							),
						),
					),
				),
			),
		),
	))
}

type htmlAttr struct {
	Key   string
	Value string
}

type htmlText string

type htmlTagArgument interface {
	isHTMLTagArgument()
}

func (htmlAttr) isHTMLTagArgument() {}
func (htmlText) isHTMLTagArgument() {}

func attr(key, value string) htmlAttr {
	return htmlAttr{key, value}
}

func text(str string) htmlText {
	return htmlText(html.EscapeString(str))
}

func tag(name string, args ...htmlTagArgument) htmlText {
	var (
		attrText, childText string
	)
	for _, arg := range args {
		switch arg := arg.(type) {
		case htmlAttr:
			attrText += fmt.Sprintf(` %s="%s"`, arg.Key, html.EscapeString(arg.Value))
		case htmlText:
			childText += string(arg)
		default:
			panic(fmt.Sprintf("unexpected argument of type %T in tag()", arg))
		}
	}
	return htmlText(fmt.Sprintf("<%s%s>%s</%s>", name, attrText, childText, name))
}
