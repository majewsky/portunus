/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package h

import (
	"fmt"
	"html/template"
	"net/http"
)

// SelectFieldSpec is a FormField where values can be selected from a given set.
// It's rendered as a series of checkboxes.
type SelectFieldSpec struct {
	Name     string
	Label    string
	Options  []SelectOptionSpec
	ReadOnly bool
}

// ReadState implements the FormField interface.
func (f SelectFieldSpec) ReadState(r *http.Request, formState *FormState) {
	if f.ReadOnly {
		return
	}

	isValidValue := make(map[string]bool)
	for _, o := range f.Options {
		isValidValue[o.Value] = true
	}

	s := FieldState{Selected: make(map[string]bool)}
	for _, value := range r.PostForm[f.Name] {
		s.Selected[value] = true
		if !isValidValue[value] {
			s.ErrorMessage = fmt.Sprintf("does not have the option %q", value)
		}
	}
	formState.Fields[f.Name] = &s
}

var selectFieldSnippet = NewSnippet(`
	<div class="form-row item-list">
		<label>
			{{.Spec.Label}}
			{{if .State.ErrorMessage}}
				<span class="form-error">{{.State.ErrorMessage}}</span>
			{{end}}
		</label>
		{{- range $idx, $opt := .Spec.Options -}}
			{{- $id := printf "%s-%d" $.Spec.Name $idx -}}
			<input
				type="checkbox" id="{{$id}}"
				{{if $.Spec.ReadOnly}}
					readonly
				{{else}}
					name="{{$.Spec.Name}}" value="{{$opt.Value}}"
				{{end}}
				{{if index $.State.Selected $opt.Value}} checked {{end}}
			/><label {{if not $.Spec.ReadOnly}} for="{{$id}}" {{end}}>{{$opt.Label}}</label>
		{{- end -}}
	</div>
`)

// RenderField implements the FormField interface.
func (f SelectFieldSpec) RenderField(state FormState) template.HTML {
	data := struct {
		Spec  SelectFieldSpec
		State *FieldState
	}{
		Spec:  f,
		State: state.Fields[f.Name],
	}
	if data.State == nil {
		data.State = &FieldState{}
	}

	return selectFieldSnippet.Render(data)
}

// SelectOptionSpec describes an option that can be selected in a SelectFieldSpec.
type SelectOptionSpec struct {
	Value string
	Label string
}
