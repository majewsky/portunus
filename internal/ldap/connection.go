/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package ldap

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	goldap "github.com/go-ldap/ldap/v3"
	"github.com/sapcc/go-bits/logg"
)

// Connection is an abstract interface for a privileged connection to the LDAP
// server. It is used by type Adapter to effect changes in the LDAP database.
// In tests, this interface's real implementation can be swapped for a double.
type Connection interface {
	DNSuffix() string
	Add(goldap.AddRequest) error
	Modify(goldap.ModifyRequest) error
	Delete(goldap.DelRequest) error
}

// ConnectionOptions contains all configuration values that we need to connect
// to the LDAP server.
type ConnectionOptions struct {
	DNSuffix      string //e.g. "dc=example,dc=org"
	Password      string //for Portunus' service user
	TLSDomainName string //if empty, LDAP without TLS is used
}

type connectionImpl struct {
	opts   ConnectionOptions
	userDN string //e.g. "cn=portunus,dc=example,dc=org"
	conn   *goldap.Conn
}

// Connect establishes a connection to an LDAP server.
func Connect(opts ConnectionOptions) (Connection, error) {
	//NOTE: we don't do any further validation on the ConnectionOptions here
	//because portunus-orchestrator supplied these values and we trust in the
	//leadership of our glorious orchestrator
	c := &connectionImpl{
		opts:   opts,
		userDN: "cn=portunus," + opts.DNSuffix,
	}

	err := c.getConn(0, 5*time.Millisecond)
	return c, err
}

func (c *connectionImpl) getConn(retryCounter int, sleepInterval time.Duration) (err error) {
	//portunus-server is started in parallel with slapd, and we don't know
	//when slapd is finished -> when initially connecting to LDAP, retry up to 10
	//times with exponential backoff (about 5-6 seconds in total) to give slapd
	//enough time to start up
	if retryCounter == 10 {
		return errors.New("giving up on LDAP server after 10 connection attempts")
	}
	time.Sleep(sleepInterval)

	u := url.URL{Scheme: "ldap", Host: "localhost"}
	if c.opts.TLSDomainName != "" {
		u = url.URL{Scheme: "ldaps", Host: c.opts.TLSDomainName}
	}
	c.conn, err = goldap.DialURL(u.String(), nil)
	if err == nil {
		err = c.conn.Bind(c.userDN, c.opts.Password)
	}
	if err != nil {
		logg.Info("cannot connect to LDAP server (attempt %d/10): %s", retryCounter+1, err.Error())
		return c.getConn(retryCounter+1, sleepInterval*2)
	}

	logg.Info("connected to LDAP server")
	return nil
}

// DNSuffix implements the Connection interface.
func (c *connectionImpl) DNSuffix() string {
	return c.opts.DNSuffix
}

// Add implements the Connection interface.
func (c *connectionImpl) Add(req goldap.AddRequest) error {
	err := c.conn.Add(&req)
	if err == nil {
		logg.Info("LDAP object %s created", req.DN)
	} else {
		return fmt.Errorf("cannot create LDAP object %s: %w", req.DN, err)
	}
	return nil
}

// Modify implements the Connection interface.
func (c *connectionImpl) Modify(req goldap.ModifyRequest) error {
	err := c.conn.Modify(&req)
	if err == nil {
		logg.Info("LDAP object %s updated", req.DN)
	} else {
		return fmt.Errorf("cannot update LDAP object %s: %w", req.DN, err)
	}
	return nil
}

// Delete implements the Connection interface.
func (c *connectionImpl) Delete(req goldap.DelRequest) error {
	err := c.conn.Del(&req)
	if err == nil {
		logg.Info("LDAP object %s deleted", req.DN)
	} else {
		return fmt.Errorf("cannot delete LDAP object %s: %w", req.DN, err)
	}
	return nil
}
