// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sapcc/go-bits/logg"
	"github.com/sapcc/go-bits/must"
)

func readLDAPTracerTLSConfig() *tls.Config {
	if os.Getenv("PORTUNUS_SLAPD_TLS_DOMAIN_NAME") == "" {
		return nil
	}
	var (
		certPath = os.Getenv("PORTUNUS_SLAPD_TLS_CERTIFICATE")
		keyPath  = os.Getenv("PORTUNUS_SLAPD_TLS_PRIVATE_KEY")
	)
	return &tls.Config{
		Certificates: []tls.Certificate{
			must.Return(tls.LoadX509KeyPair(certPath, keyPath)),
		},
	}
}

// Does not return until `ctx` expires. Call with `go`.
func runLDAPTracer(ctx context.Context, tlsConfig *tls.Config) {
	listenAddr := os.Getenv("PORTUNUS_SERVER_TRACER_LISTEN")

	// listen on TCP socket
	listener := must.Return((&net.ListenConfig{
		KeepAlive: 60 * time.Second,
	}).Listen(ctx, "tcp", listenAddr))
	if tlsConfig != nil {
		listener = tls.NewListener(listener, tlsConfig)
	}
	defer func() {
		err := listener.Close()
		if err != nil {
			logg.Error("LDAP tracer: while shutting down listener: " + err.Error())
		}
	}()

	logg.Info("LDAP tracer listening on %s...", listenAddr)

	// accept connections
	var connCounter atomic.Int64
	for {
		conn, err := listener.Accept()
		if err != nil {
			logg.Error("LDAP tracer: while accepting connection: " + err.Error())
		}
		connIdx := connCounter.Add(1)
		go handleLDAPTracerConnection(ctx, connIdx, conn)
	}
}

func handleLDAPTracerConnection(ctx context.Context, connIdx int64, clientConn net.Conn) {
	logg.Info("LDAP tracer: connection %d: accepted from %s", connIdx, clientConn.RemoteAddr())
	defer func() {
		err := clientConn.Close()
		if err != nil {
			logg.Error("LDAP tracer: while shutting down connection %d: %s", connIdx, err.Error())
		}
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// connect to server
	var (
		serverConn net.Conn
		err        error
	)
	if tlsDomainName := os.Getenv("PORTUNUS_SLAPD_TLS_DOMAIN_NAME"); tlsDomainName != "" {
		serverConn, err = ((&tls.Dialer{}).DialContext(ctx, "tcp", tlsDomainName+":636"))
	} else {
		serverConn, err = ((&net.Dialer{}).DialContext(ctx, "tcp", "localhost:389"))
	}
	if err != nil {
		logg.Error("LDAP tracer: connection %d: could not connect to server: %s", connIdx, err.Error())
		return
	}

	// proxy between client and server
	var wg sync.WaitGroup
	wg.Go(func() {
		handleLDAPTracerProxying(ctx, cancel, connIdx, "client", clientConn, "server", serverConn)
	})
	wg.Go(func() {
		handleLDAPTracerProxying(ctx, cancel, connIdx, "server", serverConn, "client", clientConn)
	})

	wg.Wait()
}

func handleLDAPTracerProxying(ctx context.Context, cancel func(), connIdx int64, readerName string, reader net.Conn, writerName string, writer net.Conn) {
	var reqBuf [32768]byte // completely excessive buffer size to make sure that we always catch full requests (to simplify the parser implementation, once there is one)
	for ctx.Err() == nil {
		msgLen, err := reader.Read(reqBuf[:])
		if err != nil {
			if err != io.EOF {
				logg.Error("LDAP tracer: connection %d: read from %s failed: %s", connIdx, readerName, err.Error())
			}
			cancel()
		}
		if msgLen == 0 {
			continue
		}
		logg.Info("LDAP tracer: connection %d: forwarding %d bytes from %s to %s: %v", connIdx, msgLen, readerName, writerName, formatMessageBytesForDisplay(reqBuf[:msgLen]))
		_, err = writer.Write(reqBuf[:msgLen])
		if err != nil {
			logg.Error("LDAP tracer: connection %d: write to %s failed: %s", connIdx, writerName, err.Error())
			cancel()
		}
	}
}

// Formats arbitrary bytes from an LDAP message into a printable form, representing printable ASCII characters as themselves and all other bytes by their hexcode.
func formatMessageBytesForDisplay(buf []byte) string {
	var (
		out       strings.Builder
		isColored bool
	)
	for _, b := range buf {
		if b > 0x20 && b < 0x7F { // printable characters (not including space = 0x20)
			if isColored {
				_, _ = out.WriteString("\x1B[0m")
				isColored = false
			}
			_ = out.WriteByte(b)
		} else {
			if !isColored {
				_, _ = out.WriteString("\x1B[0;36m")
				isColored = true
			}
			fmt.Fprintf(&out, "{%02x}", b)
		}
	}

	if isColored {
		_, _ = out.WriteString("\x1B[0m")
	}
	return out.String()
}
