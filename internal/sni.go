// ---------------------------------------------------------------------------------------
//
//	sni.go
//	------
//
//	Extracts the SNI (Server Name Indication) hostname from a TLS ClientHello
//	message without consuming the data. The buffered bytes are returned so they
//	can be replayed to the backend in passthrough mode or fed to crypto/tls
//	in terminate mode.
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
	"errors"
	"io"
	"net"
)

// ---------------------------------------------------------------------------------------
//
//	SNI Extraction
//
// ---------------------------------------------------------------------------------------

var (
	ErrNoSNI      = errors.New("no SNI hostname in ClientHello")
	ErrNotTLS     = errors.New("not a TLS handshake")
	ErrTruncated  = errors.New("truncated ClientHello")
	ErrReadFailed = errors.New("failed to read from connection")
)

// TLS alert descriptions (RFC 5246 section 7.2.2)
const (
	alertUnrecognizedName byte = 112
)

// ---------------------------------------------------------------------------------------
// sendTLSAlert sends a TLS fatal alert to the client before closing.
func sendTLSAlert(conn net.Conn, description byte) {
	// TLS alert record: content_type(21) + version(3,1) + length(0,2) + level(2=fatal) + description
	alert := []byte{21, 3, 1, 0, 2, 2, description}
	conn.Write(alert)
}

// ---------------------------------------------------------------------------------------
// PeekedConn wraps a net.Conn with buffered data that is read first before
// reading from the underlying connection. Used to replay the ClientHello.
type PeekedConn struct {
	net.Conn
	peeked []byte
	offset int
}

// ---------------------------------------------------------------------------------------
func NewPeekedConn(conn net.Conn, peeked []byte) *PeekedConn {
	return &PeekedConn{Conn: conn, peeked: peeked}
}

// ---------------------------------------------------------------------------------------
func (c *PeekedConn) Read(b []byte) (int, error) {
	if c.offset < len(c.peeked) {
		n := copy(b, c.peeked[c.offset:])
		c.offset += n
		return n, nil
	}
	return c.Conn.Read(b)
}

// ---------------------------------------------------------------------------------------
// ExtractSNI reads the TLS ClientHello from a connection and returns the
// SNI hostname and all bytes read (for replay via PeekedConn).
func ExtractSNI(conn net.Conn) (hostname string, buffered []byte, err error) {
	// Read the 5-byte TLS record header: type(1) + version(2) + length(2)
	var header [5]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return "", nil, ErrReadFailed
	}

	if header[0] != 22 {
		return "", header[:], ErrNotTLS
	}

	recordLen := int(header[3])<<8 | int(header[4])
	if recordLen < 42 || recordLen > 16384 {
		return "", header[:], ErrTruncated
	}

	// Single allocation: header + record body
	buffered = make([]byte, 5+recordLen)
	copy(buffered, header[:])
	if _, err := io.ReadFull(conn, buffered[5:]); err != nil {
		return "", buffered[:5], ErrTruncated
	}

	sni, err := parseClientHelloSNI(buffered[5:])
	if err != nil {
		return "", buffered, err
	}

	return sni, buffered, nil
}

// ---------------------------------------------------------------------------------------
// parseClientHelloSNI extracts the SNI from a TLS handshake message body.
func parseClientHelloSNI(data []byte) (string, error) {
	if len(data) < 1 || data[0] != 1 {
		return "", ErrNotTLS
	}

	// Skip: handshake type(1) + length(3) + client version(2) + random(32)
	pos := 38
	if pos >= len(data) {
		return "", ErrTruncated
	}

	// Session ID
	sessionIDLen := int(data[pos])
	pos += 1 + sessionIDLen
	if pos+2 > len(data) {
		return "", ErrTruncated
	}

	// Cipher suites
	cipherSuitesLen := int(data[pos])<<8 | int(data[pos+1])
	pos += 2 + cipherSuitesLen
	if pos+1 > len(data) {
		return "", ErrTruncated
	}

	// Compression methods
	compressionLen := int(data[pos])
	pos += 1 + compressionLen
	if pos+2 > len(data) {
		return "", ErrNoSNI
	}

	// Extensions
	extensionsLen := int(data[pos])<<8 | int(data[pos+1])
	pos += 2
	end := pos + extensionsLen
	if end > len(data) {
		return "", ErrTruncated
	}

	for pos+4 <= end {
		extType := int(data[pos])<<8 | int(data[pos+1])
		extLen := int(data[pos+2])<<8 | int(data[pos+3])
		pos += 4

		if pos+extLen > end {
			return "", ErrTruncated
		}

		if extType == 0 {
			return parseSNIExtension(data[pos : pos+extLen])
		}

		pos += extLen
	}

	return "", ErrNoSNI
}

// ---------------------------------------------------------------------------------------
// parseSNIExtension parses the SNI extension data to extract the hostname.
func parseSNIExtension(data []byte) (string, error) {
	if len(data) < 2 {
		return "", ErrTruncated
	}

	listLen := int(data[0])<<8 | int(data[1])
	if listLen+2 > len(data) {
		return "", ErrTruncated
	}

	pos := 2
	listEnd := 2 + listLen

	for pos+3 <= listEnd {
		nameType := data[pos]
		nameLen := int(data[pos+1])<<8 | int(data[pos+2])
		pos += 3

		if pos+nameLen > listEnd {
			return "", ErrTruncated
		}

		if nameType == 0 {
			return string(data[pos : pos+nameLen]), nil
		}

		pos += nameLen
	}

	return "", ErrNoSNI
}
