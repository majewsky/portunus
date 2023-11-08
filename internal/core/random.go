/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"crypto/rand"
	"fmt"
	"io"
)

// GenerateRandomKey creates a random key with the given length in bytes.
//
// It is like the function of the same name from gorilla/securecookie,
// except it panics on error, AS IT FUCKING SHOULD.
func GenerateRandomKey(length int) []byte {
	k := make([]byte, length)
	_, err := io.ReadFull(rand.Reader, k)
	if err != nil {
		panic(fmt.Sprintf("could not generate %d bytes of randomness: %s", length, err.Error()))
	}
	return k
}
