/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
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
