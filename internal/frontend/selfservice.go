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

	"github.com/majewsky/portunus/internal/core"
	h "github.com/majewsky/portunus/internal/html"
)

func getSelfHandler(e core.Engine) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentUser, perms, ok := checkAuth(w, r, e, core.Permissions{})
		if !ok {
			return
		}

		f := SelfServiceForm{
			User:      currentUser,
			AllGroups: e.ListGroups(),
		}
		WriteHTMLPage(w, http.StatusOK, "Users",
			h.Join(
				RenderNavbarForUser(currentUser, perms, r),
				h.Tag("main", f.Render(r)),
			),
		)
	})
}

func postSelfHandler(e core.Engine) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	})
}
