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

//Package h provides a terse syntax for inline HTML rendering. For instance:
//
//	headerRow := h.Tag("tr",
//		h.Tag("th", h.Text("User ID")),
//		h.Tag("th", h.Text("Name")),
//		h.Tag("th", h.Text("Groups")),
//		h.Tag("th", h.Attr("class", "actions"),
//			h.Tag("a",
//				h.Attr("href", "#"),
//				h.Attr("class", "btn btn-primary"),
//				h.Text("New user"),
//			),
//		),
//	)
//
package h

import (
	"fmt"
	"html"
	"html/template"
)

//TagArgument is implemented by Attribute and RenderedHTML. Therefore, these two
//types can be given as arguments to Tag().
type TagArgument interface {
	IsHTMLTagArgument()
}

//Attribute represents an attribute of an HTML tag.
type Attribute struct {
	Key   string
	Value string
}

//Attr constructs an Attribute.
func Attr(key, value string) Attribute {
	return Attribute{key, value}
}

//IsHTMLTagArgument implements the TagArgument interface.
func (Attribute) IsHTMLTagArgument() {}

//RenderedHTML contains HTML code.
type RenderedHTML struct {
	//This could be implemented as `type RenderedHTML string`, but then it would
	//be possible to cast unescaped text into this type without going through
	//UnsafeText().
	plain string
}

//IsHTMLTagArgument implements the TagArgument interface.
func (RenderedHTML) IsHTMLTagArgument() {}

//Text escapes the given text for usage in HTML.
func Text(str string) RenderedHTML {
	return RenderedHTML{html.EscapeString(str)}
}

//UnsafeText allows to include the given text in rendered HTML without extra escaping.
//
//WARNING: When using this function, the caller is responsible for ensuring
//that no XSS vulnerabilities are introduced.
func UnsafeText(str string) RenderedHTML {
	return RenderedHTML{str}
}

//Embed converts template.HTML into RenderedHTML, thus allowing HTML generated
//by other libraries to be used by this library.
func Embed(str template.HTML) RenderedHTML {
	return RenderedHTML{string(str)}
}

//Tag renders an HTML tag. Attributes and child nodes can be given in the
//`args` list.
func Tag(name string, args ...TagArgument) RenderedHTML {
	var (
		attrText, childText string
	)
	for _, arg := range args {
		switch arg := arg.(type) {
		case Attribute:
			attrText += fmt.Sprintf(` %s="%s"`, arg.Key, html.EscapeString(arg.Value))
		case RenderedHTML:
			childText += arg.plain
		default:
			panic(fmt.Sprintf("unexpected argument of type %T in tag()", arg))
		}
	}
	return RenderedHTML{fmt.Sprintf("<%s%s>%s</%s>", name, attrText, childText, name)}
}

//Join concatenates multiple pieces of RenderedHTML into one.
func Join(args ...RenderedHTML) RenderedHTML {
	var result string
	for _, arg := range args {
		result += arg.plain
	}
	return RenderedHTML{result}
}

//String returns the rendered HTML as a string.
func (h RenderedHTML) String() string {
	return h.plain
}
