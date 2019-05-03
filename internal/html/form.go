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
	"net/http"

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
	Value        string
	ErrorMessage string
}

////////////////////////////////////////////////////////////////////////////////
// type FormSpec

//FormField is something that can appear in an HTML form.
type FormField interface {
	ReadState(*http.Request, *FormState)
	RenderField(FormState) RenderedHTML
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

//Render produces the HTML for this form.
func (f FormSpec) Render(r *http.Request, s FormState) RenderedHTML {
	formArgs := []TagArgument{
		Attr("method", "POST"),
		Attr("action", f.PostTarget),
		Embed(csrf.TemplateField(r)),
	}

	for _, field := range f.Fields {
		formArgs = append(formArgs, field.RenderField(s))
	}

	formArgs = append(formArgs,
		Tag("div", Attr("class", "button-row"),
			Tag("button",
				Attr("type", "submit"),
				Attr("class", "btn btn-primary"),
				Text(f.SubmitLabel),
			),
		),
	)
	return Tag("form", formArgs...)
}

////////////////////////////////////////////////////////////////////////////////
// type FieldSpec

//FieldSpec describes a single <input> field within type FormSpec.
type FieldSpec struct {
	Name      string
	Label     string
	InputType string
	AutoFocus bool
	Rules     []ValidationRule
}

//ReadState reads and validates the field value from r.PostForm, and stores it
//in the given FormState.
func (f FieldSpec) ReadState(r *http.Request, formState *FormState) {
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

//RenderField produces the HTML for this field.
func (f FieldSpec) RenderField(state FormState) RenderedHTML {
	s := state.Fields[f.Name]
	if s == nil {
		s = &FieldState{}
	}

	labelArgs := []TagArgument{
		Attr("for", f.Name),
		Text(f.Label),
	}
	inputArgs := []TagArgument{
		Attr("name", f.Name),
		Attr("type", f.InputType),
	}

	if s.Value != "" && f.InputType != "password" {
		inputArgs = append(inputArgs, Attr("value", s.Value))
	}

	if f.AutoFocus {
		inputArgs = append(inputArgs, EmptyAttr("autofocus"))
	}

	if s.ErrorMessage != "" {
		labelArgs = append(labelArgs, Tag("span",
			Attr("class", "form-error"),
			Text(s.ErrorMessage),
		))
		inputArgs = append(inputArgs, Attr("class", "form-error"))
	}

	return Tag("div", Attr("class", "form-row"),
		Tag("label", labelArgs...),
		Tag("input", inputArgs...),
	)
}

////////////////////////////////////////////////////////////////////////////////
// type StaticField

//StaticField is a FormField with a static value.
type StaticField struct {
	Label      string
	CSSClasses string
	Value      RenderedHTML
}

//ReadState implements the FormField interface.
func (f StaticField) ReadState(*http.Request, *FormState) {
}

//RenderField implements the FormField interface.
func (f StaticField) RenderField(FormState) RenderedHTML {
	return Tag("div", Attr("class", "display-row"),
		Tag("div", Attr("class", "row-label"), Text(f.Label)),
		Tag("div", Attr("class", "row-value "+f.CSSClasses), f.Value),
	)
}

////////////////////////////////////////////////////////////////////////////////
// type ValidationRule

//ValidationRule returns an error message if the given field value is invalid.
type ValidationRule func(string) error

var errIsMissing = errors.New("is missing")

//MustNotBeEmpty is a ValidationRule.
func MustNotBeEmpty(val string) error {
	if val == "" {
		return errIsMissing
	}
	return nil
}
