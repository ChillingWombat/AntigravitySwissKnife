package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// HandlerFunc is the signature for JSON-RPC method handlers.
type HandlerFunc func(params json.RawMessage) (interface{}, *RPCError)

// Server implements a Unix domain socket JSON-RPC 2.0 server.
type Server struct {
	socketPath string
	listener   net.Listener
	handlers   map[string]HandlerFunc
	mu         sync.RWMutex
	running    int32
	wg         sync.WaitGroup
	connMu     sync.Mutex
	conns      map[net.Conn]struct{}
}

// NewServer initializes a new IPC server.
func NewServer(socketPath string) *Server {
	return &Server{
		socketPath: socketPath,
		handlers:   make(map[string]HandlerFunc),
		conns:      make(map[net.Conn]struct{}),
	}
}

// Register registers a handler function for a method name.
func (s *Server) Register(method string, handler HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = handler
}

// Start begins listening on the Unix domain socket with 0600 permissions.
func (s *Server) Start() error {
	dir := filepath.Dir(s.socketPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Check if an existing daemon is already actively listening on socketPath
	if _, err := os.Stat(s.socketPath); err == nil {
		conn, dialErr := net.DialTimeout("unix", s.socketPath, 300*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			return fmt.Errorf("daemon is already running and actively listening on socket: %s", s.socketPath)
		}
		// Socket file exists but no process is responding - clean up stale socket file
		_ = os.Remove(s.socketPath)
	}

	l, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to bind socket %s: %w", s.socketPath, err)
	}

	// Strict permission: 0600 (owner read/write only)
	if err := os.Chmod(s.socketPath, 0600); err != nil {
		l.Close()
		return fmt.Errorf("failed to chmod socket: %w", err)
	}

	s.listener = l
	atomic.StoreInt32(&s.running, 1)

	s.wg.Add(1)
	go s.acceptLoop()
	return nil
}

// Stop closes the listener, terminates active connections, and cleans up the socket file.
// Connection handlers block reading from their client, so they must be force-closed
// here; otherwise an idle client would keep shutdown waiting forever.
func (s *Server) Stop() error {
	if atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		if s.listener != nil {
			_ = s.listener.Close()
		}
		s.closeActiveConns()
		s.wg.Wait()
		_ = os.Remove(s.socketPath)
	}
	return nil
}

func (s *Server) closeActiveConns() {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	for c := range s.conns {
		_ = c.Close()
	}
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for atomic.LoadInt32(&s.running) == 1 {
		conn, err := s.listener.Accept()
		if err != nil {
			if atomic.LoadInt32(&s.running) == 0 {
				return
			}
			continue
		}
		s.connMu.Lock()
		s.conns[conn] = struct{}{}
		s.connMu.Unlock()
		// If Stop ran between Accept and registration, make sure this
		// connection still gets closed instead of parking a handler forever.
		if atomic.LoadInt32(&s.running) == 0 {
			_ = conn.Close()
		}
		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			defer func() {
				s.connMu.Lock()
				delete(s.conns, c)
				s.connMu.Unlock()
			}()
			s.handleConnection(c)
		}(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for atomic.LoadInt32(&s.running) == 1 {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				// connection error
			}
			return
		}

		if len(line) > 16*1024*1024 {
			resp := Response{
				JSONRPC: "2.0",
				Error: &RPCError{
					Code:    InvalidRequest,
					Message: "Request entity too large: message exceeds 16MB limit",
				},
				ID: nil,
			}
			respBytes, _ := json.Marshal(resp)
			respBytes = append(respBytes, '\n')
			_, _ = conn.Write(respBytes)
			continue
		}

		resp := s.dispatch(line)
		respBytes, err := json.Marshal(resp)
		if err == nil {
			respBytes = append(respBytes, '\n')
			_, _ = conn.Write(respBytes)
		}
	}
}

func (s *Server) dispatch(data []byte) Response {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return Response{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    ParseError,
				Message: "Parse error: invalid JSON",
			},
			ID: nil,
		}
	}

	if req.JSONRPC != "2.0" || req.Method == "" {
		return Response{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    InvalidRequest,
				Message: "Invalid Request: missing jsonrpc version or method",
			},
			ID: req.ID,
		}
	}

	s.mu.RLock()
	handler, exists := s.handlers[req.Method]
	s.mu.RUnlock()

	if !exists {
		return Response{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    MethodNotFound,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
			ID: req.ID,
		}
	}

	result, rpcErr := handler(req.Params)
	return Response{
		JSONRPC: "2.0",
		Result:  result,
		Error:   rpcErr,
		ID:      req.ID,
	}
}
