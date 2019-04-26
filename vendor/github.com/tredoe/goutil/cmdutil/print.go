// Copyright 2014 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmdutil

import (
	"fmt"
	"os"
)

var (
	StdoutPrefix = " * "
	StderrPrefix = " [!] "
)

// SetPrefix sets prefix for standard output and error.
// The prefixes have a value by default.
func SetPrefix(stdout, stderr string) {
	if stdout != "" {
		StdoutPrefix = stdout
	}
	if stderr != "" {
		StderrPrefix = stderr
	}
}

// Error is equivalent to Print() but in Stderr.
func Error(v ...interface{}) {
	if StderrPrefix != "" {
		fmt.Fprint(os.Stderr, StderrPrefix)
	}
	fmt.Fprint(os.Stderr, v...)
}

// Errorf is equivalent to Printf() but in Stderr.
func Errorf(format string, v ...interface{}) {
	if StderrPrefix != "" {
		fmt.Fprint(os.Stderr, StderrPrefix)
	}
	fmt.Fprintf(os.Stderr, format, v...)
}

// Errorln is equivalent to Println() but in Stderr.
func Errorln(v ...interface{}) {
	if StderrPrefix != "" {
		fmt.Fprint(os.Stderr, StderrPrefix)
	}
	fmt.Fprintln(os.Stderr, v...)
}

// Fatal is equivalent to Print() followed by a call to os.Exit(1).
func Fatal(v ...interface{}) {
	if StderrPrefix != "" {
		fmt.Fprint(os.Stderr, StderrPrefix)
	}
	fmt.Fprint(os.Stderr, v...)
	os.Exit(1)
}

// Fatalf is equivalent to Printf() followed by a call to os.Exit(1).
func Fatalf(format string, v ...interface{}) {
	if StderrPrefix != "" {
		fmt.Fprint(os.Stderr, StderrPrefix)
	}
	fmt.Fprintf(os.Stderr, format, v...)
	os.Exit(1)
}

// Fatalln is equivalent to Println() followed by a call to os.Exit(1).
func Fatalln(v ...interface{}) {
	if StderrPrefix != "" {
		fmt.Fprint(os.Stderr, StderrPrefix)
	}
	fmt.Fprintln(os.Stderr, v...)
	os.Exit(1)
}

// Print is equivalent to Print().
func Print(v ...interface{}) {
	if StdoutPrefix != "" {
		fmt.Print(StdoutPrefix)
	}
	fmt.Print(v...)
}

// Printf is equivalent to Printf().
func Printf(format string, v ...interface{}) {
	if StdoutPrefix != "" {
		fmt.Print(StdoutPrefix)
	}
	fmt.Printf(format, v...)
}

// Println is equivalent to Println().
func Println(v ...interface{}) {
	if StdoutPrefix != "" {
		fmt.Print(StdoutPrefix)
	}
	fmt.Println(v...)
}
