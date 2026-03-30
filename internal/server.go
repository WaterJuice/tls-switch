// ---------------------------------------------------------------------------------------
//
//	server.go
//	---------
//
//	TCP listener and connection accept loop. Routes incoming connections based
//	on SNI hostname lookup. Tracks active connections for graceful shutdown.
//	Emits connection events for Python-side logging.
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
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// ---------------------------------------------------------------------------------------
//
//	Constants
//
// ---------------------------------------------------------------------------------------

const (
	sniReadTimeout       = 10 * time.Second
	tlsHandshakeTimeout  = 15 * time.Second
	backendDialTimeout   = 10 * time.Second
	acceptBackoffDelay   = 100 * time.Millisecond
	shutdownDrainTimeout = 5 * time.Second
)

// HTTP 421 response for unknown hostnames (sent after a self-signed TLS handshake)
const http421Response = "HTTP/1.1 421 Misdirected Request\r\n" +
	"Content-Type: text/plain\r\n" +
	"Content-Length: 58\r\n" +
	"Connection: close\r\n" +
	"\r\n" +
	"421 Misdirected Request\n\nThis hostname is not configured.\n"

// ---------------------------------------------------------------------------------------
//
//	Server
//
// ---------------------------------------------------------------------------------------

// EventFunc is a callback for emitting events to the Python wrapper.
type EventFunc func(name string, data any)

// Server manages the TCP listener and routes connections.
type Server struct {
	mu          sync.Mutex
	configStore *ConfigStore
	listener    net.Listener
	running     atomic.Bool
	activeConns sync.WaitGroup
	connCount   atomic.Int64
	emitEvent   EventFunc
}

// ---------------------------------------------------------------------------------------
func NewServer(cs *ConfigStore, emitEvent EventFunc) *Server {
	return &Server{
		configStore: cs,
		emitEvent:   emitEvent,
	}
}

// ---------------------------------------------------------------------------------------
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running.Load() {
		return errors.New("server is already running")
	}

	cfg := s.configStore.Get()
	if cfg == nil {
		return ErrNotConfigured
	}

	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return err
	}

	s.listener = ln
	s.running.Store(true)

	go s.acceptLoop()
	return nil
}

// ---------------------------------------------------------------------------------------
func (s *Server) Stop() {
	s.mu.Lock()
	if !s.running.Load() {
		s.mu.Unlock()
		return
	}
	s.running.Store(false)
	if s.listener != nil {
		s.listener.Close()
		s.listener = nil
	}
	s.mu.Unlock()

	// Wait for active connections to drain, with a timeout
	done := make(chan struct{})
	go func() {
		s.activeConns.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(shutdownDrainTimeout):
	}
}

// ---------------------------------------------------------------------------------------
func (s *Server) IsRunning() bool {
	return s.running.Load()
}

// ---------------------------------------------------------------------------------------
func (s *Server) ActiveConnections() int64 {
	return s.connCount.Load()
}

// ---------------------------------------------------------------------------------------
//
//	Accept Loop
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
func (s *Server) acceptLoop() {
	for s.running.Load() {
		conn, err := s.listener.Accept()
		if err != nil {
			if !s.running.Load() {
				return
			}
			time.Sleep(acceptBackoffDelay)
			continue
		}

		s.activeConns.Add(1)
		s.connCount.Add(1)
		go s.handleConnection(conn)
	}
}

// ---------------------------------------------------------------------------------------
func (s *Server) handleConnection(conn net.Conn) {
	defer s.activeConns.Done()
	defer s.connCount.Add(-1)

	remoteAddr := conn.RemoteAddr().String()
	conn.SetReadDeadline(time.Now().Add(sniReadTimeout))

	hostname, buffered, err := ExtractSNI(conn)
	if err != nil {
		s.emitEvent("connection", map[string]string{
			"time":   time.Now().Format(time.RFC3339),
			"source": remoteAddr,
			"error":  "failed to extract SNI: " + err.Error(),
		})
		if errors.Is(err, ErrNoSNI) {
			sendTLSAlert(conn, alertUnrecognizedName)
		}
		conn.Close()
		return
	}

	conn.SetReadDeadline(time.Time{})

	route := s.configStore.Lookup(hostname)
	if route == nil {
		s.emitEvent("connection", map[string]string{
			"time":     time.Now().Format(time.RFC3339),
			"source":   remoteAddr,
			"hostname": hostname,
			"action":   "rejected",
			"reason":   "unknown hostname",
		})
		s.rejectWithHTTP(conn, buffered, hostname)
		return
	}

	s.emitEvent("connection", map[string]string{
		"time":     time.Now().Format(time.RFC3339),
		"source":   remoteAddr,
		"hostname": hostname,
		"mode":     route.Mode,
		"backend":  route.Backend,
	})

	switch route.Mode {
	case ModePassthrough:
		HandlePassthrough(conn, buffered, route)
	case ModeTerminate:
		HandleTerminate(conn, buffered, route)
	default:
		conn.Close()
	}
}

// ---------------------------------------------------------------------------------------
// rejectWithHTTP completes a TLS handshake using a cert borrowed from any
// configured host, then sends an HTTP 421 response. This lets browsers
// complete the TLS handshake and display the error page rather than showing
// a generic "can't connect" message.
func (s *Server) rejectWithHTTP(conn net.Conn, buffered []byte, hostname string) {
	defer conn.Close()

	tlsConfig := s.configStore.AnyTLSConfig()
	if tlsConfig == nil {
		sendTLSAlert(conn, alertUnrecognizedName)
		return
	}

	peeked := NewPeekedConn(conn, buffered)
	tlsConn := tls.Server(peeked, tlsConfig)
	defer tlsConn.Close()

	tlsConn.SetDeadline(time.Now().Add(tlsHandshakeTimeout))
	if err := tlsConn.Handshake(); err != nil {
		return
	}
	tlsConn.SetDeadline(time.Time{})

	tlsConn.Write([]byte(http421Response))
}
