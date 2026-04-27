// ---------------------------------------------------------------------------------------
//
//	proxyproto_test.go
//	------------------
//
//	Byte-exact tests for PROXY protocol v1/v2 header emission.
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
// ---------------------------------------------------------------------------------------
package internal

// ---------------------------------------------------------------------------------------
//
//	Imports
//
// ---------------------------------------------------------------------------------------

import (
	"bytes"
	"net"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------------------
//
//	Test helpers
//
// ---------------------------------------------------------------------------------------

// mockConn captures Write calls into an embedded bytes.Buffer and returns
// configurable RemoteAddr/LocalAddr for use as a fake client connection.
type mockConn struct {
	bytes.Buffer
	remote net.Addr
	local  net.Addr
}

func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return m.remote }
func (m *mockConn) LocalAddr() net.Addr                { return m.local }
func (m *mockConn) SetDeadline(_ time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(_ time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(_ time.Time) error { return nil }

// ---------------------------------------------------------------------------------------
//
//	V1 (text) tests
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
func TestWriteProxyHeaderV1_TCP4(t *testing.T) {
	backend := &mockConn{}
	src := &net.TCPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 12345}
	dst := &net.TCPAddr{IP: net.IPv4(5, 6, 7, 8), Port: 443}
	if err := writeProxyHeaderV1(backend, src, dst); err != nil {
		t.Fatal(err)
	}
	want := "PROXY TCP4 1.2.3.4 5.6.7.8 12345 443\r\n"
	if got := backend.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------------------
func TestWriteProxyHeaderV1_TCP6(t *testing.T) {
	backend := &mockConn{}
	src := &net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 12345}
	dst := &net.TCPAddr{IP: net.ParseIP("::1"), Port: 443}
	if err := writeProxyHeaderV1(backend, src, dst); err != nil {
		t.Fatal(err)
	}
	want := "PROXY TCP6 2001:db8::1 ::1 12345 443\r\n"
	if got := backend.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------------------
// V4-mapped V6 (`::ffff:1.2.3.4`) must be emitted as plain TCP4 — common when a
// dual-stack listener accepts an IPv4 client.
func TestWriteProxyHeaderV1_MappedV4(t *testing.T) {
	backend := &mockConn{}
	src := &net.TCPAddr{IP: net.ParseIP("::ffff:1.2.3.4"), Port: 12345}
	dst := &net.TCPAddr{IP: net.ParseIP("::ffff:5.6.7.8"), Port: 443}
	if err := writeProxyHeaderV1(backend, src, dst); err != nil {
		t.Fatal(err)
	}
	want := "PROXY TCP4 1.2.3.4 5.6.7.8 12345 443\r\n"
	if got := backend.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------------------
// When src and dst have different families (real v6 vs v4-mapped v6), both
// addresses must be rendered in v6 form against a TCP6 family token. The
// v4-mapped one uses the RFC 5952 ::ffff:x.x.x.x notation rather than the
// dotted-quad that Go's IP.String() would otherwise return.
func TestWriteProxyHeaderV1_MismatchedFamilies(t *testing.T) {
	backend := &mockConn{}
	src := &net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 12345}
	dst := &net.TCPAddr{IP: net.ParseIP("::ffff:10.0.0.1"), Port: 443}
	if err := writeProxyHeaderV1(backend, src, dst); err != nil {
		t.Fatal(err)
	}
	want := "PROXY TCP6 2001:db8::1 ::ffff:10.0.0.1 12345 443\r\n"
	if got := backend.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------------------
//
//	V2 (binary) tests
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
func TestWriteProxyHeaderV2_Inet(t *testing.T) {
	backend := &mockConn{}
	src := &net.TCPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 0xABCD}
	dst := &net.TCPAddr{IP: net.IPv4(5, 6, 7, 8), Port: 443}
	if err := writeProxyHeaderV2(backend, src, dst); err != nil {
		t.Fatal(err)
	}
	want := []byte{
		0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A,
		0x21,
		0x11,
		0x00, 0x0C,
		0x01, 0x02, 0x03, 0x04,
		0x05, 0x06, 0x07, 0x08,
		0xAB, 0xCD,
		0x01, 0xBB,
	}
	if got := backend.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

// ---------------------------------------------------------------------------------------
func TestWriteProxyHeaderV2_Inet6(t *testing.T) {
	backend := &mockConn{}
	src := &net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 12345}
	dst := &net.TCPAddr{IP: net.ParseIP("::1"), Port: 443}
	if err := writeProxyHeaderV2(backend, src, dst); err != nil {
		t.Fatal(err)
	}
	want := []byte{
		0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A,
		0x21,
		0x21,
		0x00, 0x24,
		0x20, 0x01, 0x0D, 0xB8, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x30, 0x39,
		0x01, 0xBB,
	}
	if got := backend.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

// ---------------------------------------------------------------------------------------
//
//	Unspec / unknown-family fallback
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
func TestWriteProxyHeaderUnspec_V1(t *testing.T) {
	backend := &mockConn{}
	if err := writeProxyHeaderUnspec(backend, ProxyProtocolV1); err != nil {
		t.Fatal(err)
	}
	want := "PROXY UNKNOWN\r\n"
	if got := backend.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------------------
func TestWriteProxyHeaderUnspec_V2(t *testing.T) {
	backend := &mockConn{}
	if err := writeProxyHeaderUnspec(backend, ProxyProtocolV2); err != nil {
		t.Fatal(err)
	}
	want := []byte{
		0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A,
		0x21, 0x00, 0x00, 0x00,
	}
	if got := backend.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

// ---------------------------------------------------------------------------------------
//
//	Dispatcher (writeProxyHeader)
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
func TestWriteProxyHeader_OffNoOps(t *testing.T) {
	backend := &mockConn{}
	client := &mockConn{
		remote: &net.TCPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 12345},
		local:  &net.TCPAddr{IP: net.IPv4(5, 6, 7, 8), Port: 443},
	}
	route := &HostRoute{ProxyProtocol: ProxyProtocolOff}
	if err := writeProxyHeader(backend, route, client); err != nil {
		t.Fatal(err)
	}
	if backend.Len() != 0 {
		t.Errorf("expected no bytes written, got %d (%x)", backend.Len(), backend.Bytes())
	}
}

// ---------------------------------------------------------------------------------------
func TestWriteProxyHeader_DispatchesV1(t *testing.T) {
	backend := &mockConn{}
	client := &mockConn{
		remote: &net.TCPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 12345},
		local:  &net.TCPAddr{IP: net.IPv4(5, 6, 7, 8), Port: 443},
	}
	route := &HostRoute{ProxyProtocol: ProxyProtocolV1}
	if err := writeProxyHeader(backend, route, client); err != nil {
		t.Fatal(err)
	}
	want := "PROXY TCP4 1.2.3.4 5.6.7.8 12345 443\r\n"
	if got := backend.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------------------
func TestWriteProxyHeader_DispatchesV2(t *testing.T) {
	backend := &mockConn{}
	client := &mockConn{
		remote: &net.TCPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 12345},
		local:  &net.TCPAddr{IP: net.IPv4(5, 6, 7, 8), Port: 443},
	}
	route := &HostRoute{ProxyProtocol: ProxyProtocolV2}
	if err := writeProxyHeader(backend, route, client); err != nil {
		t.Fatal(err)
	}
	got := backend.Bytes()
	if !bytes.HasPrefix(got, proxyV2Signature) {
		t.Fatalf("expected v2 signature prefix, got %x", got)
	}
	if len(got) != 16+12 {
		t.Errorf("expected 28 bytes (header + INET payload), got %d", len(got))
	}
}

// ---------------------------------------------------------------------------------------
// When the client connection's RemoteAddr/LocalAddr aren't *net.TCPAddr
// (impossible via the production accept loop, but spec-mandated to handle), the
// dispatcher must fall back to the unspec emitter rather than panic on the type
// assertion.
func TestWriteProxyHeader_NonTCPAddrFallsBackToUnspec(t *testing.T) {
	backend := &mockConn{}
	client := &mockConn{
		remote: &net.UnixAddr{Name: "/tmp/sock", Net: "unix"},
		local:  &net.UnixAddr{Name: "/tmp/sock", Net: "unix"},
	}

	t.Run("v1", func(t *testing.T) {
		backend.Reset()
		route := &HostRoute{ProxyProtocol: ProxyProtocolV1}
		if err := writeProxyHeader(backend, route, client); err != nil {
			t.Fatal(err)
		}
		if got := backend.String(); got != "PROXY UNKNOWN\r\n" {
			t.Errorf("got %q, want %q", got, "PROXY UNKNOWN\r\n")
		}
	})

	t.Run("v2", func(t *testing.T) {
		backend.Reset()
		route := &HostRoute{ProxyProtocol: ProxyProtocolV2}
		if err := writeProxyHeader(backend, route, client); err != nil {
			t.Fatal(err)
		}
		want := []byte{
			0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A,
			0x21, 0x00, 0x00, 0x00,
		}
		if got := backend.Bytes(); !bytes.Equal(got, want) {
			t.Errorf("got  %x\nwant %x", got, want)
		}
	})
}
