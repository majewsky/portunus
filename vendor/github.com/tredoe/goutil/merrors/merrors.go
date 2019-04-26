// Copyright 2014 Jonas mg
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Based in code from 'http://golang.org/src/pkg/go/scanner/errors.go'

// Package merrors implements functions to manipulate multiple errors.
package merrors

import (
	"fmt"
	"io"
	"strings"
)

// In a ListError or MapError, an error is represented by an Error.
type Error string

// Error implements the error interface.
func (e Error) Error() string {
	return string(e)
}

// ListError is a list of errors.
type ListError []Error

// Add adds an Error with given position and error message to a ListError.
func (e *ListError) Add(msg string) {
	*e = append(*e, Error(msg))
}

// A ListError implements the error interface.
func (e ListError) Error() string {
	switch len(e) {
	case 0:
		return "no errors"
	case 1:
		return e[0].Error()
	}

	listOut := make([]string, 0)
	for i, v := range e {
		if i == 4 {
			break
		}
		listOut = append(listOut, v.Error())
	}

	out := strings.Join(listOut, "\n")
	if len(e) > 4 {
		out += fmt.Sprintf("\n(and %d more errors)", len(e)-4)
	}
	return out
}

// Err returns an error equivalent to this error list.
// If the list is empty, Err returns nil.
func (e ListError) Err() error {
	if len(e) == 0 {
		return nil
	}
	return e
}

// MapError maps a string key to an error.
type MapError map[string]Error

// Set sets the key to value.
func (e MapError) Set(key, value string) {
	e[key] = Error(value)
}

// Err returns an error equivalent to this error map.
// If the map is empty, Err returns nil.
func (e MapError) Err() error {
	if len(e) == 0 {
		return nil
	}
	return e
}

// An Error implements the error interface.
func (e MapError) Error() string {
	switch len(e) {
	case 0:
		return "no errors"
	case 1:
		for _, v := range e {
			return fmt.Sprintf("%s", v)
		}
	}

	listOut := make([]string, 0)
	i := 0
	for _, v := range e {
		if i == 4 {
			break
		}
		listOut = append(listOut, v.Error())
		i++
	}

	out := strings.Join(listOut, "\n")
	if len(e) > 4 {
		out += fmt.Sprintf("\n(and %d more errors)", len(e)-4)
	}
	return out
}

// PrintError is a utility function that prints a list of errors to w, one error per line,
// if the err parameter is a ListError or a MapError. Otherwise it prints the err string.
func PrintError(w io.Writer, err error) {
	if listErr, ok := err.(ListError); ok {
		for _, e := range listErr {
			fmt.Fprintf(w, "%s\n", e)
		}
	} else if mapErr, ok := err.(MapError); ok {
		for _, e := range mapErr {
			fmt.Fprintf(w, "%s\n", e)
		}
	} else if err != nil {
		fmt.Fprintf(w, "%s\n", err)
	}
}
