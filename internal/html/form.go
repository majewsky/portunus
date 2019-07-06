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

package h

import (
	"errors"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/gorilla/csrf"
)

////////////////////////////////////////////////////////////////////////////////
// state

//FormState describes the state of an HTML form.
type FormState struct {
	Fields map[string]*FieldState
}

//IsValid returns false if any field has a validation error.
func (s FormState) IsValid() bool {
	for _, field := range s.Fields {
		if field != nil && field.ErrorMessage != "" {
			return false
		}
	}
	return true
}

//FieldState describes the state of an <input> field within type FormState.
type FieldState struct {
	Value        string          //only used by InputFieldSpec
	Selected     map[string]bool //only used by SelectFieldSpec
	IsUnfolded   bool            //only used by FieldSet
	ErrorMessage string
}

////////////////////////////////////////////////////////////////////////////////
// type FormSpec

//FormField is something that can appear in an HTML form.
type FormField interface {
	ReadState(*http.Request, *FormState)
	RenderField(FormState) template.HTML
}

//FormSpec describes an HTML form that is submitted to a POST endpoint.
type FormSpec struct {
	PostTarget  string
	SubmitLabel string
	Fields      []FormField
}

//ReadState reads and validates the field value from r.PostForm, and stores it
//in the given FormState.
func (f FormSpec) ReadState(r *http.Request, s *FormState) {
	if s.Fields == nil {
		s.Fields = make(map[string]*FieldState)
	}
	for _, field := range f.Fields {
		field.ReadState(r, s)
	}
}

var formSpecSnippet = NewSnippet(`
	<form method="POST" action={{.Spec.PostTarget}}>
		{{.Fields}}
		<div class="button-row">
			<button type="submit" class="btn btn-primary">{{.Spec.SubmitLabel}}</button>
		</div>
	</form>
`)

//Render produces the HTML for this form.
func (f FormSpec) Render(r *http.Request, s FormState) template.HTML {
	data := struct {
		Spec   FormSpec
		Fields template.HTML
	}{
		Spec:   f,
		Fields: csrf.TemplateField(r),
	}
	for _, field := range f.Fields {
		data.Fields = data.Fields + field.RenderField(s)
	}
	return formSpecSnippet.Render(data)
}

////////////////////////////////////////////////////////////////////////////////
// type InputFieldSpec

//InputFieldSpec describes a single <input> field within type FormSpec.
type InputFieldSpec struct {
	Name             string
	Label            string
	InputType        string
	AutoFocus        bool
	AutocompleteMode string
	Rules            []ValidationRule
}

//ReadState reads and validates the field value from r.PostForm, and stores it
//in the given FormState.
func (f InputFieldSpec) ReadState(r *http.Request, formState *FormState) {
	s := FieldState{Value: r.PostForm.Get(f.Name)}
	for _, rule := range f.Rules {
		err := rule(s.Value)
		if err != nil {
			s.ErrorMessage = err.Error()
			break
		}
	}
	formState.Fields[f.Name] = &s
}

var inputFieldSnippet = NewSnippet(`
	<div class="form-row">
		<label for="{{.Spec.Name}}" class="row-label">
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

//RenderField produces the HTML for this field.
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
// type StaticField

//StaticField is a FormField with a static value.
type StaticField struct {
	Label string
	Value template.HTML
}

//ReadState implements the FormField interface.
func (f StaticField) ReadState(*http.Request, *FormState) {
}

var staticFieldSnippet = NewSnippet(`
	<div class="display-row">
		<div class="row-label">{{.Label}}</div>
		<div class="row-value">{{.Value}}</div>
	</div>
`)

//RenderField implements the FormField interface.
func (f StaticField) RenderField(FormState) template.HTML {
	if f.Label == "" {
		return f.Value
	}
	return staticFieldSnippet.Render(f)
}

////////////////////////////////////////////////////////////////////////////////
// type FieldSet

//FieldSet is a FormField that groups multiple FormFields together.
type FieldSet struct {
	Name       string
	Label      string
	Fields     []FormField
	IsFoldable bool
}

//ReadState implements the FormField interface.
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

//NOTE: This does not use <legend> because <legend> inside <fieldset> applies
//special layouting rules that make styling them with CSS unnecessarily hard.
var fieldSetSnippet = NewSnippet(`
	{{if .Spec.IsFoldable}}
		<input type="checkbox" class="for-fieldset" id="{{.Spec.Name}}" name="{{.Spec.Name}}" value="1" {{if .State.IsUnfolded}}checked{{end}}>
	{{end}}
	<fieldset>
		<label for="{{.Spec.Name}}">{{.Spec.Label}}</label>
		{{.Fields}}
	</fieldset>
`)

//RenderField implements the FormField interface.
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

////////////////////////////////////////////////////////////////////////////////
// type ValidationRule

//ValidationRule returns an error message if the given field value is invalid.
type ValidationRule func(string) error

//this regexp copied from useradd(8) manpage
const posixAccountNamePattern = `[a-z_][a-z0-9_-]*\$?`

var (
	errIsMissing      = errors.New("is missing")
	errLeadingSpaces  = errors.New("may not start with a space character")
	errTrailingSpaces = errors.New("may not end with a space character")

	errNotPosixAccountName = errors.New("is not an acceptable user/group name matching the pattern /" + posixAccountNamePattern + "/")
	posixAccountNameRx     = regexp.MustCompile(`^` + posixAccountNamePattern + `$`)
	errNotPosixUIDorGID    = errors.New("is not a number between 0 and 65535 inclusive")

	errNotAbsolutePath = errors.New("must be an absolute path, i.e. start with a /")
)

//MustNotBeEmpty is a ValidationRule.
func MustNotBeEmpty(val string) error {
	if strings.TrimSpace(val) == "" {
		return errIsMissing
	}
	return nil
}

//MustNotHaveSurroundingSpaces is a ValidationRule.
func MustNotHaveSurroundingSpaces(val string) error {
	if val != "" {
		if strings.TrimLeftFunc(val, unicode.IsSpace) != val {
			return errLeadingSpaces
		}
		if strings.TrimRightFunc(val, unicode.IsSpace) != val {
			return errTrailingSpaces
		}
	}
	return nil
}

//MustBePosixAccountName is a ValidationRule.
func MustBePosixAccountName(val string) error {
	if posixAccountNameRx.MatchString(val) {
		return nil
	}
	return errNotPosixAccountName
}

//MustBePosixUIDorGID is a ValidationRule.
func MustBePosixUIDorGID(val string) error {
	if val != "" {
		_, err := strconv.ParseUint(val, 10, 16)
		if err != nil {
			return errNotPosixUIDorGID
		}
	}
	return nil
}

//MustBeAbsolutePath is a ValidationRule.
func MustBeAbsolutePath(val string) error {
	if val != "" && !strings.HasPrefix(val, "/") {
		return errNotAbsolutePath
	}
	return nil
}
