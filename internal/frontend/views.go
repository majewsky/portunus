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
	"html/template"
	"net/http"
	"strings"

	"github.com/gorilla/sessions"
	"github.com/majewsky/portunus/internal/core"
	h "github.com/majewsky/portunus/internal/html"
)

var mainSnippet = h.NewSnippet(`
	<!DOCTYPE html>
	<html>
		<head>
			<meta charset="utf-8">
			<meta http-equiv="X-UA-Compatible" content="IE=edge" />
			<meta name="viewport" content="width=device-width, initial-scale=1">
			<title>
				{{- if .Page.Title -}}
					{{ .Page.Title }} - Portunus
				{{- else -}}
					Portunus
				{{- end -}}
			</title>
			<link rel="stylesheet" type="text/css" href="/static/css/portunus.css" />
		</head>
		<body {{if .Page.Wide}}class="wide"{{end}}>
			<nav id="nav">
				<div id="nav-bar">
					<div id="nav-title">
						<img src="/static/img/logo-for-menubar.png" alt="Site logo">
					</div>
					<a id="nav-fold" href="#">
						<img src="/static/img/logo-for-menubar.png" alt="Site logo">
						<span>Close menu</span>
					</a>
					<a id="nav-unfold" href="#nav">
						<img src="/static/img/logo-for-menubar.png" alt="Site logo">
						<span>{{.Page.Title}} - Portunus</span>
					</a>
					<div class="nav-area" id="nav-left">
						{{ if .CurrentUser }}
							<a href="/self" class="nav-item {{if eq .CurrentSection "self"}}nav-item-current{{end}}">My profile</a>
							{{if .CurrentUser.Perms.Portunus.IsAdmin}}
								<a href="/users" class="nav-item {{if eq .CurrentSection "users"}}nav-item-current{{end}}">Users</a>
								<a href="/groups" class="nav-item {{if eq .CurrentSection "groups"}}nav-item-current{{end}}">Groups</a>
							{{end}}
						{{ else }}
							<a class="nav-item nav-item-current" href="/login">Login to Portunus</a>
						{{ end }}
					</div>
					<div class="nav-area" id="nav-right">
						{{ if .CurrentUserFullName }}
							<div class="nav-item nav-item-current">{{.CurrentUserFullName}}</div>
							<a class="nav-item" href="/logout">Logout</a>
						{{ end }}
					</div>
				</div>
			</nav>
			<main>
				{{range .Flashes}}<div class="flash flash-{{.Type}}">{{.Message}}</div>{{end}}
				{{.Page.Contents}}
			</main>
		</body>
	</html>
`)

//Flash is a flash message.
type Flash struct {
	Type    string //either "danger" or "success"
	Message string
}

func init() {
	gob.Register(Flash{})
}

//Page describes a HTML page produced by Portunus.
type Page struct {
	Status   int
	Title    string
	Contents template.HTML
	Wide     bool
}

//Render renders the given page.
func (p Page) Render(w http.ResponseWriter, r *http.Request, currentUser *core.UserWithPerms, s *sessions.Session) {
	data := struct {
		Page                Page
		CurrentUser         *core.UserWithPerms
		CurrentUserFullName string
		CurrentSection      string
		Navigation          template.HTML
		Flashes             []Flash
	}{
		Page:           p,
		CurrentUser:    currentUser,
		CurrentSection: strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)[0],
	}
	if currentUser != nil {
		data.CurrentUserFullName = currentUser.FullName()
	}

	for _, value := range s.Flashes() {
		if f, ok := value.(Flash); ok {
			data.Flashes = append(data.Flashes, f)
		}
	}
	err := s.Save(r, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(p.Status)
	w.Write([]byte(mainSnippet.Render(data)))
}
