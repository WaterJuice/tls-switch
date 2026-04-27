// ---------------------------------------------------------------------------------------
//
//	proxyproto.go
//	-------------
//
//	PROXY protocol v1 (text) and v2 (binary) header emission. The header is
//	written to the backend connection immediately after dialling, before any
//	other bytes flow, so the backend can recover the original client address.
//
//	Spec: https://www.haproxy.org/download/3.0/doc/proxy-protocol.txt
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
//	Version History
//	---------------
//	Apr 2026 - Created
//
// ---------------------------------------------------------------------------------------
package internal

// ---------------------------------------------------------------------------------------
//
//	Imports
//
// ---------------------------------------------------------------------------------------

import (
	"encoding/binary"
	"fmt"
	"net"
)

// ---------------------------------------------------------------------------------------
//
//	Constants
//
// ---------------------------------------------------------------------------------------

var proxyV2Signature = []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A}

const (
	proxyV2VerCmd   byte = 0x21
	proxyV2AfInet4  byte = 0x11
	proxyV2AfInet6  byte = 0x21
	proxyV2AfUnspec byte = 0x00
)

// ---------------------------------------------------------------------------------------
//
//	Public API
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// writeProxyHeader writes a PROXY protocol header to the freshly-dialled backend
// describing the client→listener TCP relationship. No-ops if PROXY protocol is
// not configured for the route. Must be called before any other bytes flow.
func writeProxyHeader(backend net.Conn, route *HostRoute, client net.Conn) error {
	if route.ProxyProtocol == ProxyProtocolOff {
		return nil
	}

	src, srcOK := client.RemoteAddr().(*net.TCPAddr)
	dst, dstOK := client.LocalAddr().(*net.TCPAddr)
	if !srcOK || !dstOK {
		return writeProxyHeaderUnspec(backend, route.ProxyProtocol)
	}

	if route.ProxyProtocol == ProxyProtocolV1 {
		return writeProxyHeaderV1(backend, src, dst)
	}
	return writeProxyHeaderV2(backend, src, dst)
}

// ---------------------------------------------------------------------------------------
//
//	Internals
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
func writeProxyHeaderUnspec(backend net.Conn, version string) error {
	if version == ProxyProtocolV1 {
		_, err := backend.Write([]byte("PROXY UNKNOWN\r\n"))
		return err
	}
	buf := make([]byte, 16)
	copy(buf, proxyV2Signature)
	buf[12] = proxyV2VerCmd
	buf[13] = proxyV2AfUnspec
	_, err := backend.Write(buf)
	return err
}

// ---------------------------------------------------------------------------------------
func writeProxyHeaderV1(backend net.Conn, src, dst *net.TCPAddr) error {
	srcV4 := src.IP.To4()
	dstV4 := dst.IP.To4()

	var family, srcIP, dstIP string
	if srcV4 != nil && dstV4 != nil {
		family = "TCP4"
		srcIP = srcV4.String()
		dstIP = dstV4.String()
	} else {
		family = "TCP6"
		srcIP = ipToV6String(src.IP)
		dstIP = ipToV6String(dst.IP)
	}

	line := fmt.Sprintf("PROXY %s %s %s %d %d\r\n", family, srcIP, dstIP, src.Port, dst.Port)
	_, err := backend.Write([]byte(line))
	return err
}

// ---------------------------------------------------------------------------------------
// ipToV6String formats an IP for use in a TCP6 PROXY v1 header. Go's IP.String()
// returns dotted-quad for v4-mapped addresses, which would be invalid against a
// TCP6 family token; this function forces the RFC 5952 ::ffff:x.x.x.x form.
func ipToV6String(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return fmt.Sprintf("::ffff:%d.%d.%d.%d", v4[0], v4[1], v4[2], v4[3])
	}
	return ip.String()
}

// ---------------------------------------------------------------------------------------
func writeProxyHeaderV2(backend net.Conn, src, dst *net.TCPAddr) error {
	const headerLen = 16

	srcV4 := src.IP.To4()
	dstV4 := dst.IP.To4()
	useV4 := srcV4 != nil && dstV4 != nil

	var familyByte byte
	var payloadLen int
	if useV4 {
		familyByte = proxyV2AfInet4
		payloadLen = 12
	} else {
		familyByte = proxyV2AfInet6
		payloadLen = 36
	}

	buf := make([]byte, headerLen+payloadLen)
	copy(buf, proxyV2Signature)
	buf[12] = proxyV2VerCmd
	buf[13] = familyByte
	binary.BigEndian.PutUint16(buf[14:16], uint16(payloadLen))

	off := headerLen
	if useV4 {
		copy(buf[off:off+4], srcV4)
		off += 4
		copy(buf[off:off+4], dstV4)
		off += 4
	} else {
		copy(buf[off:off+16], src.IP.To16())
		off += 16
		copy(buf[off:off+16], dst.IP.To16())
		off += 16
	}
	binary.BigEndian.PutUint16(buf[off:off+2], uint16(src.Port))
	off += 2
	binary.BigEndian.PutUint16(buf[off:off+2], uint16(dst.Port))

	_, err := backend.Write(buf)
	return err
}
