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

package core

import "github.com/tredoe/osutil/user/crypt/sha256_crypt"

//HashPasswordForLDAP produces a password hash in the format expected by LDAP,
//like the libc function crypt(3).
func HashPasswordForLDAP(password string) string {
	//according to documentation, Crypter.Generate() will never return any errors
	//when the second argument is nil
	result, _ := sha256_crypt.New().Generate([]byte(password), nil)
	return "{CRYPT}" + result
}
