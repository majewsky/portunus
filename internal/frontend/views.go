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
	"net/http"
	"strings"

	"github.com/gorilla/csrf"
	"github.com/majewsky/portunus/internal/core"
	h "github.com/majewsky/portunus/internal/html"
)

var standardHeadTags = h.Join(
	h.Tag("meta",
		h.Attr("charset", "utf-8"),
	),
	h.Tag("meta",
		h.Attr("http-equiv", "X-UA-Compatible"),
		h.Attr("content", "IE=edge"),
	),
	h.Tag("meta",
		h.Attr("name", "viewport"),
		h.Attr("content", "width=device-width, initial-scale=1"),
	),
	h.Tag("link",
		h.Attr("rel", "stylesheet"),
		h.Attr("href", "/static/css/portunus.css"),
	),
)

//WriteHTMLPage writes a complete HTML page into w.
func WriteHTMLPage(w http.ResponseWriter, status int, title string, bodyContents h.RenderedHTML) {
	titleText := "Portunus"
	if title != "" {
		titleText = title + " – Portunus"
	}

	headTag := h.Tag("head",
		standardHeadTags,
		h.Tag("title", h.Text(titleText)),
	)

	htmlTag := h.Tag("html",
		headTag,
		h.Tag("body", bodyContents),
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	w.Write([]byte(htmlTag.String()))
}

//NavbarItem is an item that appears in the top navbar.
type NavbarItem struct {
	URL    string
	Title  string
	Active bool
}

//RenderNavbarForUser returns the top navbar for a logged-in user.
func RenderNavbarForUser(user core.UserWithPerms, r *http.Request) h.RenderedHTML {
	items := []NavbarItem{{
		URL:    "/self",
		Title:  "My profile",
		Active: strings.HasPrefix(r.URL.Path, "/self"),
	}}
	if user.Perms.Portunus.IsAdmin {
		items = append(items,
			NavbarItem{
				URL:    "/users",
				Title:  "Users",
				Active: strings.HasPrefix(r.URL.Path, "/users"),
			},
			NavbarItem{
				URL:    "/groups",
				Title:  "Groups",
				Active: strings.HasPrefix(r.URL.Path, "/groups"),
			},
		)
	}
	return RenderNavbar(user.FullName(), items...)
}

//RenderNavbar renders the top navbar that appears in every view.
func RenderNavbar(currentUserID string, items ...NavbarItem) h.RenderedHTML {
	fields := []h.TagArgument{
		h.Tag("li", h.Tag("h1", h.Text("Portunus"))),
	}
	for _, item := range items {
		linkArgs := []h.TagArgument{
			h.Text(item.Title), h.Attr("href", item.URL),
		}
		if item.Active {
			linkArgs = append(linkArgs, h.Attr("class", "current"))
		}
		fields = append(fields, h.Tag("li", h.Tag("a", linkArgs...)))
	}

	if currentUserID != "" {
		fields = append(fields, h.Tag("li", h.Attr("class", "spacer")))
		fields = append(fields, h.Tag("li", h.Tag("a", h.Attr("class", "current"), h.Text(currentUserID))))
		fields = append(fields, h.Tag("li", h.Tag("a",
			h.Attr("href", "/logout"),
			h.Text("Logout"),
		)))
	}

	return h.Tag("nav", h.Tag("ul", fields...))
}

//FormField represents the state of an <input> field.
type FormField struct {
	Value        string
	ErrorMessage string
}

//Render returns the HTML for this form field.
func (f FormField) Render(inputType, name, label string) h.RenderedHTML {
	labelArgs := []h.TagArgument{h.Attr("for", name), h.Text(label)}
	inputCSSClass := ""
	if f.ErrorMessage != "" {
		labelArgs = append(labelArgs, h.Tag("span",
			h.Attr("class", "form-error"),
			h.Text(f.ErrorMessage),
		))
		inputCSSClass = "form-error"
	}
	return h.Tag("div", h.Attr("class", "form-row"),
		h.Tag("label", labelArgs...),
		h.Tag("input",
			h.Attr("class", inputCSSClass),
			h.Attr("name", name),
			h.Attr("type", inputType),
			h.Attr("value", f.Value),
		),
	)
}

//RenderDisplayField renders something that looks like a FormField, but is readonly.
func RenderDisplayField(label string, value ...h.RenderedHTML) h.RenderedHTML {
	args := []h.TagArgument{h.Attr("class", "row-value")}
	for _, v := range value {
		args = append(args, v)
	}

	return h.Tag("div", h.Attr("class", "display-row"),
		h.Tag("div", h.Attr("class", "row-label"), h.Text(label)),
		h.Tag("div", args...),
	)
}

//LoginForm represents the state of the login form.
type LoginForm struct {
	UserName FormField
	Password FormField
}

//Render returns the HTML for this form field.
func (f LoginForm) Render(r *http.Request) h.RenderedHTML {
	return h.Tag("form", h.Attr("method", "POST"), h.Attr("action", "/login"),
		h.Embed(csrf.TemplateField(r)),
		f.UserName.Render("text", "uid", "User ID"),
		f.Password.Render("password", "password", "Password"),
		h.Tag("div", h.Attr("class", "button-row"),
			h.Tag("button", h.Attr("type", "submit"), h.Attr("class", "btn btn-primary"), h.Text("Login")),
		),
	)
}

//SelfServiceForm represents the state of the self-service form.
type SelfServiceForm struct {
	User      core.UserWithPerms
	Password1 FormField
	Password2 FormField
}

//Render returns the HTML for this form field.
func (f SelfServiceForm) Render(r *http.Request) h.RenderedHTML {
	return h.Tag("form", h.Attr("method", "POST"), h.Attr("action", "/self"),
		h.Embed(csrf.TemplateField(r)),
		RenderDisplayField("Login name", h.Tag("code", h.Text(f.User.LoginName))),
		RenderDisplayField("Full name",
			//TODO: allow flipped order (family name first)
			h.Tag("span", h.Attr("class", "given-name"), h.Text(f.User.GivenName)),
			h.Text(" "),
			h.Tag("span", h.Attr("class", "family-name"), h.Text(f.User.FamilyName)),
		),
		RenderDisplayField("Group memberships", RenderGroupMemberships(f.User.User, f.User.GroupMemberships, f.User)),
		f.Password1.Render("password", "password1", "New password"),
		f.Password2.Render("password", "password1", "Repeat password"),
		h.Tag("div", h.Attr("class", "button-row"),
			h.Tag("button", h.Attr("type", "submit"), h.Attr("class", "btn btn-primary"), h.Text("Change password")),
		),
	)
}

//RenderGroupMemberships renders a list of all groups the given user is part of.
func RenderGroupMemberships(user core.User, groups []core.Group, currentUser core.UserWithPerms) h.RenderedHTML {
	//TODO use links only if user has perms.Portunus.IsAdmin
	isAdmin := currentUser.Perms.Portunus.IsAdmin
	var groupMemberships []h.RenderedHTML
	for _, group := range groups {
		if !group.ContainsUser(user) {
			continue
		}
		if len(groupMemberships) > 0 {
			groupMemberships = append(groupMemberships, h.Text(", "))
		}
		if isAdmin {
			groupMemberships = append(groupMemberships, h.Tag("a",
				h.Attr("href", "/groups/"+group.Name),
				h.Text(group.Name),
			))
		} else {
			groupMemberships = append(groupMemberships, h.Text(group.Name))
		}
	}
	return h.Join(groupMemberships...)
}
