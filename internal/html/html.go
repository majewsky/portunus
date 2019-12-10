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

//Package h provides utilties for rendering HTML forms.
package h

import (
	"bytes"
	"html"
	"html/template"
	"strings"
)

//Snippet provides a convenience API around html/template.Template.
type Snippet struct {
	T *template.Template
}

//NewSnippet parses html/template code into a Snippet.
func NewSnippet(input string) Snippet {
	return Snippet{template.Must(template.New("").Parse(strings.TrimSpace(input)))}
}

//Render renders the snippet with the given data.
func (s Snippet) Render(data interface{}) template.HTML {
	var buf bytes.Buffer
	err := s.T.Execute(&buf, data)
	if err != nil {
		return template.HTML(`<div class="flash flash-danger">` + html.EscapeString(err.Error()) + `</div>`)
	}
	return template.HTML(buf.String())
}
