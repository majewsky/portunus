/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package crypt

/*
#cgo LDFLAGS: -lcrypt
#include "lowlevel.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"strings"
	"unsafe"
)

// Returns a non-nil error if our libcrypt does not have all required capabilities.
func llFeatureTest() error {
	output := C.feature_test()
	defer C.free(unsafe.Pointer(output))

	message := C.GoString(output)
	if message == "" {
		return nil
	}
	return errors.New(message)
}

// Wraps the C library function crypt_preferred_method().
func llPreferredMethod() string {
	return C.GoString(C.crypt_preferred_method())
}

// Wraps the C library function crypt_r(). This function can be used for both
// hashing and verifying, like this: (error handling elided)
//
//	// to hash
//	hash := llCrypt(plainTextPassword, lowlevel_gensalt(prefix))
//	// to verify
//	ok := llCrypt(plainTextPassword, storedHash) == storedHash
//
// The resulting hash will have the `prefix` argument from GenerateSalt() as a prefix.
func llCrypt(phrase, setting string) (string, error) {
	phraseInput := C.CString(phrase)
	defer C.free(unsafe.Pointer(phraseInput))

	settingInput := C.CString(setting)
	defer C.free(unsafe.Pointer(settingInput))

	// We call crypt_r() through a wrapper that ensures that the return value is
	// allocated on the heap. We don't use crypt_ra() since it's easier to deal
	// with an individual C string than an entire struct crypt_data.
	output, err := C.wrap_crypt_r(phraseInput, settingInput)
	defer C.free(unsafe.Pointer(output))
	if err != nil {
		return "", err
	}

	result := C.GoString(output)
	if result == "" || strings.HasPrefix(result, "*") {
		return "", fmt.Errorf("invalid result: %q", result)
	}
	return result, nil
}

// Wraps the C library function crypt_gensalt_rn(). If called with an empty `prefix`,
// this autoselects the preferred hash algorithm. Otherwise, the hash algorithm
// specified by the `prefix` will be used.
func llGenerateSalt(prefix string) (string, error) {
	prefixInput := C.CString(prefix)
	defer C.free(unsafe.Pointer(prefixInput))

	output, err := C.wrap_crypt_gensalt_rn(prefixInput)
	defer C.free(unsafe.Pointer(output))
	if err != nil {
		return "", err
	}

	return C.GoString(output), nil
}
