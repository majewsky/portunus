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
	"encoding/hex"
	"fmt"
	math_rand "math/rand"
	"net/http"
)

//SelectFieldSpec is a FormField where values can be selected from a given set.
//It's rendered as a series of checkboxes.
type SelectFieldSpec struct {
	Name     string
	Label    string
	Options  []SelectOptionSpec
	ReadOnly bool
}

//ReadState implements the FormField interface.
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

//RenderField implements the FormField interface.
func (f SelectFieldSpec) RenderField(state FormState) RenderedHTML {
	s := state.Fields[f.Name]
	if s == nil {
		s = &FieldState{}
	}

	items := []TagArgument{Attr("class", "item-list")}
	for _, o := range f.Options {
		if f.ReadOnly {
			cssClasses := "item item-unchecked"
			if s.Selected[o.Value] {
				cssClasses = "item item-checked"
			}
			if o.Href == "" {
				items = append(items, Tag("span", Attr("class", cssClasses), Text(o.Label)))
			} else {
				items = append(items, Tag("a",
					Attr("href", o.Href),
					Attr("class", cssClasses),
					Text(o.Label),
				))
			}
		} else {
			id := getRandomID()
			inputArgs := []TagArgument{
				Attr("type", "checkbox"),
				Attr("name", f.Name),
				Attr("id", id),
				Attr("value", o.Value),
			}
			if s.Selected[o.Value] {
				inputArgs = append(inputArgs, EmptyAttr("checked"))
			}
			items = append(items,
				Tag("input", inputArgs...),
				Tag("label", Attr("for", id), Attr("class", "item"), Text(o.Label)),
			)
		}
	}

	labelArgs := []TagArgument{Text(f.Label)}
	if s.ErrorMessage != "" {
		labelArgs = append(labelArgs, Tag("span",
			Attr("class", "form-error"),
			Text(s.ErrorMessage),
		))
	}

	return Tag("div", Attr("class", "form-row"),
		Tag("div", labelArgs...),
		Tag("div", items...),
	)
}

//SelectOptionSpec describes an option that can be selected in a SelectFieldSpec.
type SelectOptionSpec struct {
	Value string
	Label string
	Href  string //only used for read-only fields
}

func getRandomID() string {
	var buf [10]byte
	math_rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}
