/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package h

import (
	"html/template"
	"net/http"

	"github.com/gorilla/csrf"
	"github.com/majewsky/portunus/internal/core"
	"github.com/sapcc/go-bits/errext"
)

////////////////////////////////////////////////////////////////////////////////
// state

// FormState describes the state of an HTML form.
type FormState struct {
	Fields        map[string]*FieldState
	ErrorMessages []string //errors that do not apply to specific fields
}

// IsValid returns false if any field has a validation error.
func (s FormState) IsValid() bool {
	if len(s.ErrorMessages) > 0 {
		return false
	}
	for _, field := range s.Fields {
		if field != nil && field.ErrorMessage != "" {
			return false
		}
	}
	return true
}

// FillErrorsFrom distributes all core.ValidationError instances matching the
// given ObjectRef into the individual FieldState.ErrorMessage fields,
// and collects all other errors in the main FormState.ErrorMessages field.
func (s *FormState) FillErrorsFrom(errs errext.ErrorSet, oref core.ObjectRef) {
	for _, err := range errs {
		switch err := err.(type) {
		case core.ValidationError:
			fn := err.FieldRef.Name
			if err.FieldRef.Object == oref && s.Fields[fn] != nil && s.Fields[fn].ErrorMessage == "" {
				s.Fields[fn].ErrorMessage = err.FieldError.Error()
			} else {
				s.ErrorMessages = append(s.ErrorMessages, err.Error())
			}
		default:
			s.ErrorMessages = append(s.ErrorMessages, err.Error())
		}
	}
}

// FieldState describes the state of an <input> field within type FormState.
type FieldState struct {
	Value        string          //only used by InputFieldSpec
	Selected     map[string]bool //only used by SelectFieldSpec
	IsUnfolded   bool            //only used by FieldSet
	ErrorMessage string
}

// GetValueOrSetError returns the field's value.
// If it is empty, the ErrorMessage is filled.
func (s *FieldState) GetValueOrSetError() string {
	if s.Value == "" {
		s.ErrorMessage = "must not be empty"
	}
	return s.Value
}

////////////////////////////////////////////////////////////////////////////////
// type FormSpec

// FormField is something that can appear in an HTML form.
type FormField interface {
	ReadState(*http.Request, *FormState)
	RenderField(FormState) template.HTML
}

// FormSpec describes an HTML form that is submitted to a POST endpoint.
type FormSpec struct {
	PostTarget  string
	SubmitLabel string
	Fields      []FormField
}

// ReadState reads and validates the field value from r.PostForm, and stores it
// in the given FormState.
func (f FormSpec) ReadState(r *http.Request, s *FormState) {
	if s.Fields == nil {
		s.Fields = make(map[string]*FieldState)
	}
	for _, field := range f.Fields {
		field.ReadState(r, s)
	}
}

var formSpecSnippet = NewSnippet(`
	{{- range .ErrorMessages }}
		<div class="flash flash-danger">{{ . }}</div>
	{{- end }}
	<form method="POST" action={{.Spec.PostTarget}}>
		{{.Fields}}
		<div class="button-row">
			<button type="submit" class="button button-primary">{{.Spec.SubmitLabel}}</button>
		</div>
	</form>
`)

// Render produces the HTML for this form.
func (f FormSpec) Render(r *http.Request, s FormState) template.HTML {
	data := struct {
		Spec          FormSpec
		Fields        template.HTML
		ErrorMessages []string
	}{
		Spec:          f,
		Fields:        csrf.TemplateField(r),
		ErrorMessages: s.ErrorMessages,
	}
	for _, field := range f.Fields {
		data.Fields = data.Fields + field.RenderField(s)
	}
	return formSpecSnippet.Render(data)
}

////////////////////////////////////////////////////////////////////////////////
// type InputFieldSpec

// InputFieldSpec describes a single <input> field within type FormSpec.
type InputFieldSpec struct {
	Name             string
	Label            string
	InputType        string
	AutoFocus        bool
	AutocompleteMode string
}

// ReadState reads and validates the field value from r.PostForm, and stores it
// in the given FormState.
func (f InputFieldSpec) ReadState(r *http.Request, formState *FormState) {
	formState.Fields[f.Name] = &FieldState{Value: r.PostForm.Get(f.Name)}
}

var inputFieldSnippet = NewSnippet(`
	<div class="form-row">
		<label for="{{.Spec.Name}}">
			{{.Spec.Label}}
			{{if .State.ErrorMessage}}
				<span class="form-error">{{.State.ErrorMessage}}</span>
			{{end}}
		</label>
		<input
			name="{{.Spec.Name}}" type="{{.Spec.InputType}}"
			{{ if and (ne .State.Value "") (ne .Spec.InputType "password") }}value="{{.State.Value}}"{{ end }}
			{{ if .Spec.AutoFocus }}autofocus{{ end }}
			class="row-input {{if .State.ErrorMessage}}form-error{{end}}"
			autocomplete="{{if .Spec.AutocompleteMode}}{{.Spec.AutocompleteMode}}{{else}}off{{end}}"
		/>
	</div>
`)

// RenderField produces the HTML for this field.
func (f InputFieldSpec) RenderField(state FormState) template.HTML {
	data := struct {
		Spec  InputFieldSpec
		State *FieldState
	}{
		Spec:  f,
		State: state.Fields[f.Name],
	}
	if data.State == nil {
		data.State = &FieldState{}
	}
	return inputFieldSnippet.Render(data)
}

////////////////////////////////////////////////////////////////////////////////
// type MultilineInputFieldSpec

// MultilineInputFieldSpec describes a single <input> field within type FormSpec.
type MultilineInputFieldSpec struct {
	Name  string
	Label string
}

// ReadState reads and validates the field value from r.PostForm, and stores it
// in the given FormState.
func (f MultilineInputFieldSpec) ReadState(r *http.Request, formState *FormState) {
	formState.Fields[f.Name] = &FieldState{Value: r.PostForm.Get(f.Name)}
}

var multilineInputFieldSnippet = NewSnippet(`
	<div class="form-row">
		<label for="{{.Spec.Name}}">
			{{.Spec.Label}}
			{{if .State.ErrorMessage}}
				<span class="form-error">{{.State.ErrorMessage}}</span>
			{{end}}
		</label>
		<textarea
			name="{{.Spec.Name}}"
			class="row-input {{if .State.ErrorMessage}}form-error{{end}}"
			autocomplete="off">
				{{- .State.Value -}}
		</textarea>
	</div>
`)

// RenderField produces the HTML for this field.
func (f MultilineInputFieldSpec) RenderField(state FormState) template.HTML {
	data := struct {
		Spec  MultilineInputFieldSpec
		State *FieldState
	}{
		Spec:  f,
		State: state.Fields[f.Name],
	}
	if data.State == nil {
		data.State = &FieldState{}
	}
	return multilineInputFieldSnippet.Render(data)
}

////////////////////////////////////////////////////////////////////////////////
// type StaticField

// StaticField is a FormField with a static value.
type StaticField struct {
	Label string
	Value template.HTML
}

// ReadState implements the FormField interface.
func (f StaticField) ReadState(*http.Request, *FormState) {
}

var staticFieldSnippet = NewSnippet(`
	<div class="form-row">
		<label>{{.Label}}</label>
		<div class="row-value">{{.Value}}</div>
	</div>
`)

// RenderField implements the FormField interface.
func (f StaticField) RenderField(FormState) template.HTML {
	if f.Label == "" {
		return f.Value
	}
	return staticFieldSnippet.Render(f)
}

////////////////////////////////////////////////////////////////////////////////
// type FieldSet

// FieldSet is a FormField that groups multiple FormFields together.
type FieldSet struct {
	Name       string
	Label      string
	Fields     []FormField
	IsFoldable bool
}

// ReadState implements the FormField interface.
func (fs FieldSet) ReadState(r *http.Request, s *FormState) {
	if fs.IsFoldable {
		isUnfolded := r.PostForm.Get(fs.Name) == "1"
		s.Fields[fs.Name] = &FieldState{IsUnfolded: isUnfolded}
		if !isUnfolded {
			return
		}
	}

	for _, f := range fs.Fields {
		f.ReadState(r, s)
	}
}

// NOTE: This does not use <legend> because <legend> inside <fieldset> applies
// special layouting rules that make styling them with CSS unnecessarily hard.
var fieldSetSnippet = NewSnippet(`
	{{if .Spec.IsFoldable}}
		<input type="checkbox" class="for-fieldset" id="{{.Spec.Name}}" name="{{.Spec.Name}}" value="1" {{if .State.IsUnfolded}}checked{{end}}>
	{{end}}
	<fieldset>
		<label for="{{.Spec.Name}}">{{.Spec.Label}}</label>
		{{.Fields}}
	</fieldset>
`)

// RenderField implements the FormField interface.
func (fs FieldSet) RenderField(state FormState) template.HTML {
	data := struct {
		Spec   FieldSet
		State  *FieldState
		Fields template.HTML
	}{
		Spec:  fs,
		State: state.Fields[fs.Name],
	}
	if data.State == nil {
		data.State = &FieldState{}
	}

	for _, f := range fs.Fields {
		data.Fields = data.Fields + f.RenderField(state)
	}

	return fieldSetSnippet.Render(data)
}
