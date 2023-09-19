/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package test

import (
	"testing"

	"github.com/sapcc/go-bits/errext"
)

// ExpectNoError fails the test if err is not nil.
func ExpectNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Error(err.Error())
	}
}

// ExpectNoErrors fails the test if errs is not empty.
func ExpectNoErrors(t *testing.T, errs errext.ErrorSet) {
	t.Helper()
	for _, err := range errs {
		t.Error(err.Error())
	}
}
