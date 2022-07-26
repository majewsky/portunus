// Copyright 2010 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package user

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tredoe/goutil/reflectutil"
	"github.com/tredoe/osutil/config/shconf"
	"github.com/tredoe/osutil/user/crypt"
)

// TODO: handle des, bcrypt and rounds in SHA2.

// TODO: Idea: store struct "configData" to run configData.Init() only when
// the configuration files have been modified.

// == System configuration files.

const fileLogin = "/etc/login.defs"

type confLogin struct {
	PASS_MIN_DAYS int
	PASS_MAX_DAYS int
	PASS_MIN_LEN  int
	PASS_WARN_AGE int

	SYS_UID_MIN int
	SYS_UID_MAX int
	SYS_GID_MIN int
	SYS_GID_MAX int

	UID_MIN int
	UID_MAX int
	GID_MIN int
	GID_MAX int

	ENCRYPT_METHOD       string // upper
	SHA_CRYPT_MIN_ROUNDS int
	SHA_CRYPT_MAX_ROUNDS int
	// or
	CRYPT_PREFIX string // $2a$
	CRYPT_ROUNDS int    // 8
}

const fileUseradd = "/etc/default/useradd"

type confUseradd struct {
	HOME  string // Default to '/home'
	SHELL string // Default to '/bin/sh'
}

// == Optional files.

// Used in systems derivated from Debian: Ubuntu, Mint.
const fileAdduser = "/etc/adduser.conf"

type confAdduser struct {
	FIRST_SYSTEM_UID int
	LAST_SYSTEM_UID  int
	FIRST_SYSTEM_GID int
	LAST_SYSTEM_GID  int

	FIRST_UID int
	LAST_UID  int
	FIRST_GID int
	LAST_GID  int
}

// Used in Arch, Manjaro, OpenSUSE.
// But it is only used by 'pam_unix2.so'.
const filePasswd = "/etc/default/passwd"

// TODO: to see the other options of that file.
type confPasswd struct {
	CRYPT string // lower
}

// Used in systems derivated from Red Hat: CentOS, Fedora, Mageia, PCLinuxOS.
const fileLibuser = "/etc/libuser.conf"

type confLibuser struct {
	login_defs  string
	crypt_style string // lower

	// For SHA2
	hash_rounds_min int
	hash_rounds_max int
}

// * * *

var debug bool // For testing

// A configData represents the configuration used to add users and groups.
type configData struct {
	login   confLogin
	useradd confUseradd

	crypter crypt.Crypter
	sync.Once
}

var config configData

// init sets the configuration data.
func (c *configData) init() error {
	_confLogin := &confLogin{}

	cfg, err := shconf.ParseFile(fileLogin)
	if err != nil {
		return err
	}
	if err = cfg.Unmarshal(_confLogin); err != nil {
		return err
	}
	if debug {
		fmt.Printf("\n* %s\n", fileLogin)
		reflectutil.PrintStruct(_confLogin)
	}

	if _confLogin.PASS_MAX_DAYS == 0 {
		_confLogin.PASS_MAX_DAYS = 99999
	}
	if _confLogin.PASS_WARN_AGE == 0 {
		_confLogin.PASS_WARN_AGE = 7
	}

	cfg, err = shconf.ParseFile(fileUseradd)
	if err != nil {
		return err
	}
	_confUseradd := &confUseradd{}
	if err = cfg.Unmarshal(_confUseradd); err != nil {
		return err
	}
	if debug {
		fmt.Printf("\n* %s\n", fileUseradd)
		reflectutil.PrintStruct(_confUseradd)
	}

	if _confUseradd.HOME == "" {
		_confUseradd.HOME = "/home"
	}
	if _confUseradd.SHELL == "" {
		_confUseradd.SHELL = "/bin/sh"
	}
	config.useradd = *_confUseradd

	// Optional files

	found, err := exist(fileAdduser) // Based in Debian.
	if found {
		cfg, err := shconf.ParseFile(fileAdduser)
		if err != nil {
			return err
		}
		_confAdduser := &confAdduser{}
		if err = cfg.Unmarshal(_confAdduser); err != nil {
			return err
		}
		if debug {
			fmt.Printf("\n* %s\n", fileAdduser)
			reflectutil.PrintStruct(_confAdduser)
		}

		if _confLogin.SYS_UID_MIN == 0 || _confLogin.SYS_UID_MAX == 0 ||
			_confLogin.SYS_GID_MIN == 0 || _confLogin.SYS_GID_MAX == 0 ||
			_confLogin.UID_MIN == 0 || _confLogin.UID_MAX == 0 ||
			_confLogin.GID_MIN == 0 || _confLogin.GID_MAX == 0 {

			_confLogin.SYS_UID_MIN = _confAdduser.FIRST_SYSTEM_UID
			_confLogin.SYS_UID_MAX = _confAdduser.LAST_SYSTEM_UID
			_confLogin.SYS_GID_MIN = _confAdduser.FIRST_SYSTEM_GID
			_confLogin.SYS_GID_MAX = _confAdduser.LAST_SYSTEM_GID

			_confLogin.UID_MIN = _confAdduser.FIRST_UID
			_confLogin.UID_MAX = _confAdduser.LAST_UID
			_confLogin.GID_MIN = _confAdduser.FIRST_GID
			_confLogin.GID_MAX = _confAdduser.LAST_GID
		}
	} else if err != nil {
		return err

	} else if found, err = exist(fileLibuser); found { // Based in Red Hat.
		cfg, err := shconf.ParseFile(fileLibuser)
		if err != nil {
			return err
		}
		_confLibuser := &confLibuser{}
		if err = cfg.Unmarshal(_confLibuser); err != nil {
			return err
		}
		if debug {
			fmt.Printf("\n* %s\n", fileLibuser)
			reflectutil.PrintStruct(_confLibuser)
		}

		if _confLibuser.login_defs != fileLogin {
			_confLogin.ENCRYPT_METHOD = _confLibuser.crypt_style
			_confLogin.SHA_CRYPT_MIN_ROUNDS = _confLibuser.hash_rounds_min
			_confLogin.SHA_CRYPT_MAX_ROUNDS = _confLibuser.hash_rounds_max
		}
	} else if err != nil {
		return err

	} /*else if found, err = exist(filePasswd); found {
		cfg, err := shconf.ParseFile(filePasswd)
		if err != nil {
			return err
		}
		_confPasswd := &confPasswd{}
		if err = cfg.Unmarshal(_confPasswd); err != nil {
			return err
		}
		if debug {
			fmt.Printf("\n* %s\n", filePasswd)
			reflectutil.PrintStruct(_confPasswd)
		}

		if _confPasswd.CRYPT != "" {
			_confLogin.ENCRYPT_METHOD = _confPasswd.CRYPT
		}
	} else if err != nil {
		return err
	}*/

	switch strings.ToUpper(_confLogin.ENCRYPT_METHOD) {
	case "MD5":
		c.crypter = crypt.New(crypt.MD5)
	case "SHA256":
		c.crypter = crypt.New(crypt.SHA256)
	case "SHA512":
		c.crypter = crypt.New(crypt.SHA512)
	case "":
		if c.crypter, err = lookupCrypter(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("user: requested cryp function is unavailable: %s",
			c.login.ENCRYPT_METHOD)
	}

	if _confLogin.SYS_UID_MIN == 0 || _confLogin.SYS_UID_MAX == 0 ||
		_confLogin.SYS_GID_MIN == 0 || _confLogin.SYS_GID_MAX == 0 ||
		_confLogin.UID_MIN == 0 || _confLogin.UID_MAX == 0 ||
		_confLogin.GID_MIN == 0 || _confLogin.GID_MAX == 0 {

		_confLogin.SYS_UID_MIN = 100
		_confLogin.SYS_UID_MAX = 999
		_confLogin.SYS_GID_MIN = 100
		_confLogin.SYS_GID_MAX = 999

		_confLogin.UID_MIN = 1000
		_confLogin.UID_MAX = 29999
		_confLogin.GID_MIN = 1000
		_confLogin.GID_MAX = 29999
	}

	config.login = *_confLogin
	return nil
}

// loadConfig loads user configuration.
// It has to be loaded before of edit some file.
func loadConfig() {
	config.Do(func() {
		//checkRoot()
		if err := config.init(); err != nil {
			panic(err)
		}
	})
}
