// ---------------------------------------------------------------------------------------
//
//	proxy.go
//	--------
//
//	Bidirectional data forwarding for both passthrough and terminate modes.
//	Uses io.Copy for efficient zero-copy forwarding where the OS supports it.
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
//	Version History
//	---------------
//	Mar 2026 - Created
//
// ---------------------------------------------------------------------------------------
package internal

// ---------------------------------------------------------------------------------------
//
//	Imports
//
// ---------------------------------------------------------------------------------------

import (
	"crypto/tls"
	"io"
	"net"
	"time"
)

// ---------------------------------------------------------------------------------------
//
//	Interfaces
//
// ---------------------------------------------------------------------------------------

// closeWriter is implemented by connections that support half-close.
type closeWriter interface {
	CloseWrite() error
}

// ---------------------------------------------------------------------------------------
//
//	Proxy Functions
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
func HandlePassthrough(clientConn net.Conn, buffered []byte, route *HostRoute) {
	defer clientConn.Close()

	backendConn, err := net.DialTimeout("tcp", route.Backend, backendDialTimeout)
	if err != nil {
		return
	}
	defer backendConn.Close()

	if err := writeProxyHeader(backendConn, route, clientConn); err != nil {
		return
	}

	if _, err := backendConn.Write(buffered); err != nil {
		return
	}

	bidirectionalCopy(clientConn, backendConn)
}

// ---------------------------------------------------------------------------------------
func HandleTerminate(clientConn net.Conn, buffered []byte, route *HostRoute) {
	defer clientConn.Close()

	peeked := NewPeekedConn(clientConn, buffered)

	tlsConn := tls.Server(peeked, route.TLSConfig)

	tlsConn.SetDeadline(time.Now().Add(tlsHandshakeTimeout))

	if err := tlsConn.Handshake(); err != nil {
		tlsConn.Close()
		return
	}

	tlsConn.SetDeadline(time.Time{})

	backendConn, err := net.DialTimeout("tcp", route.Backend, backendDialTimeout)
	if err != nil {
		tlsConn.Close()
		return
	}
	defer backendConn.Close()
	defer tlsConn.Close()

	if err := writeProxyHeader(backendConn, route, clientConn); err != nil {
		return
	}

	bidirectionalCopy(tlsConn, backendConn)
}

// ---------------------------------------------------------------------------------------
func bidirectionalCopy(a, b net.Conn) {
	done := make(chan struct{}, 1)

	go func() {
		io.Copy(b, a)
		if cw, ok := b.(closeWriter); ok {
			cw.CloseWrite()
		}
		done <- struct{}{}
	}()

	io.Copy(a, b)
	if cw, ok := a.(closeWriter); ok {
		cw.CloseWrite()
	}
	<-done
}
