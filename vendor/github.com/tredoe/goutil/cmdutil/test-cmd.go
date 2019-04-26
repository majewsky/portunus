// Copyright 2014 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmdutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tredoe/goutil/merrors"
)

// CommandInfo represents the command for testing.
type CommandInfo struct {
	Args   string // the arguments after of the command
	In     string // in the event that the command needs to read the input
	Out    string // output expected
	Stderr string // error expected
}

// TestCommand tests whether a command in the given directory returns the
// expected strings for both standard output and error.
//
// Returns an error implemented in package "github.com/tredoe/goutil/merrors".
func TestCommand(dir string, tests []CommandInfo) error {
	cmdFile := fmt.Sprintf(".%c_cmd_", filepath.Separator)

	if dir == "" || dir == "." {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		cmdFile += filepath.Base(wd)

	} else if dir != "" && dir[0] != '.' && dir[0] != filepath.Separator {
		dir = "." + string(filepath.Separator) + dir
		cmdFile += filepath.Base(dir)
	}

	if runtime.GOOS == "windows" {
		cmdFile += ".exe"
	}

	// "go build" generates an executable binary with the directory name
	out, err := exec.Command("go", "build", "-o", cmdFile, dir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s\n%s", out, err)
	}

	var listErr merrors.ListError

	for i, tt := range tests {
		var bufStdout, bufStderr bytes.Buffer
		cmd := new(exec.Cmd)

		if tt.Args != "" {
			cmd = exec.Command(cmdFile, strings.Split(tt.Args, " ")...)
		} else {
			cmd = exec.Command(cmdFile)
		}

		cmd.Stdout = &bufStdout
		cmd.Stderr = &bufStderr
		if tt.In != "" {
			cmd.Stdin = strings.NewReader(tt.In)
		}

		err = cmd.Run()
		cmdStderr := bufStderr.String()
		if err != nil && cmdStderr == "" {
			return err
		}

		if cmdStderr != tt.Stderr {
			listErr.Add(fmt.Sprintf("%d. Stderr => %q\n\n- Want    => %q\n* * *",
				i, cmdStderr, tt.Stderr))
		}

		cmdStdout := bufStdout.String()
		if cmdStdout != tt.Out {
			listErr.Add(fmt.Sprintf("%d. Stdout => %q\n\n- Want    => %q\n* * *",
				i, cmdStdout, tt.Out))
		}
	}

	if err = os.Remove(cmdFile); err != nil {
		listErr.Add(err.Error())
	}
	return listErr.Err()
}
