// ---------------------------------------------------------------------------------------
//
//	server.go
//	---------
//
//	TCP listener and connection accept loop. Routes incoming connections based
//	on SNI hostname lookup. Tracks active connections for graceful shutdown.
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
//	Version History
//	---------------
//	Mar 2026 - Created
//
// ---------------------------------------------------------------------------------------
package main

// ---------------------------------------------------------------------------------------
//
//	Imports
//
// ---------------------------------------------------------------------------------------

import (
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
	sniReadTimeout      = 10 * time.Second
	tlsHandshakeTimeout = 15 * time.Second
	backendDialTimeout  = 10 * time.Second
	acceptBackoffDelay  = 100 * time.Millisecond
)

// ---------------------------------------------------------------------------------------
//
//	Server
//
// ---------------------------------------------------------------------------------------

// Server manages the TCP listener and routes connections.
type Server struct {
	mu          sync.Mutex
	configStore *ConfigStore
	listener    net.Listener
	running     atomic.Bool
	activeConns sync.WaitGroup
	connCount   atomic.Int64
}

// ---------------------------------------------------------------------------------------
func NewServer(cs *ConfigStore) *Server {
	return &Server{configStore: cs}
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

	s.activeConns.Wait()
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

	conn.SetReadDeadline(time.Now().Add(sniReadTimeout))

	hostname, buffered, err := ExtractSNI(conn)
	if err != nil {
		if errors.Is(err, ErrNoSNI) {
			sendTLSAlert(conn, alertUnrecognizedName)
		}
		conn.Close()
		return
	}

	conn.SetReadDeadline(time.Time{})

	route := s.configStore.Lookup(hostname)
	if route == nil {
		sendTLSAlert(conn, alertUnrecognizedName)
		conn.Close()
		return
	}

	switch route.Mode {
	case ModePassthrough:
		HandlePassthrough(conn, buffered, route)
	case ModeTerminate:
		HandleTerminate(conn, buffered, route)
	default:
		conn.Close()
	}
}
