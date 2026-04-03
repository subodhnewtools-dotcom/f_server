// Package rpc provides JSON-RPC 2.0 server over Unix domain sockets.
package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/pocketserver/pocketd/internal/config"
	"github.com/pocketserver/pocketd/internal/metrics"
	"github.com/pocketserver/pocketd/internal/supervisor"
)

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// Error represents a JSON-RPC 2.0 error.
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error implements the error interface for RPC Error.
func (e *Error) Error() string {
	return fmt.Sprintf("RPC %d: %s", e.Code, e.Message)
}

// Error codes as per JSON-RPC 2.0 specification.
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32000
)

// Server is the JSON-RPC 2.0 server.
type Server struct {
	socketPath string
	listener   net.Listener
	handlers   map[string]Handler
	config     *config.Config
	supervisor *supervisor.Supervisor
	collector  *metrics.Collector
	mu         sync.RWMutex
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

// Handler is a function that handles a JSON-RPC method.
type Handler func(ctx context.Context, params json.RawMessage) (interface{}, error)

// NewServer creates a new RPC server.
func NewServer(socketPath string, cfg *config.Config, sup *supervisor.Supervisor, coll *metrics.Collector) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		socketPath: socketPath,
		handlers:   make(map[string]Handler),
		config:     cfg,
		supervisor: sup,
		collector:  coll,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Register registers a handler for a method.
func (s *Server) Register(method string, handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = handler
}

// Start starts the RPC server.
func (s *Server) Start() error {
	// Remove existing socket file if it exists
	if _, err := os.Stat(s.socketPath); err == nil {
		os.Remove(s.socketPath)
	}

	// Create listener
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.socketPath, err)
	}
	s.listener = listener

	// Register built-in handlers
	s.registerDaemonHandlers()
	s.registerMetricsHandlers()

	// Accept connections
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-s.ctx.Done():
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					if s.ctx.Err() != nil {
						return
					}
					continue
				}
				s.wg.Add(1)
				go func() {
					defer s.wg.Done()
					s.handleConnection(conn)
				}()
			}
		}
	}()

	return nil
}

// Stop stops the RPC server gracefully.
func (s *Server) Stop(ctx context.Context) error {
	s.cancel()

	if s.listener != nil {
		s.listener.Close()
	}

	// Wait for all connections to close with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for connections to close")
	}
}

// handleConnection handles a single client connection.
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	decoder := json.NewDecoder(reader)
	encoder := json.NewEncoder(conn)

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			var req Request
			if err := decoder.Decode(&req); err != nil {
				if err == io.EOF {
					return
				}
				// Send parse error
				resp := Response{
					JSONRPC: "2.0",
					ID:      "",
					Error: &Error{
						Code:    ParseError,
						Message: "Parse error: " + err.Error(),
					},
				}
				_ = encoder.Encode(resp)
				return
			}

			// Validate request
			if req.JSONRPC != "2.0" {
				resp := Response{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &Error{
						Code:    InvalidRequest,
						Message: "Invalid Request: jsonrpc must be 2.0",
					},
				}
				_ = encoder.Encode(resp)
				continue
			}

			if req.Method == "" {
				resp := Response{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &Error{
						Code:    InvalidRequest,
						Message: "Invalid Request: method is required",
					},
				}
				_ = encoder.Encode(resp)
				continue
			}

			// Handle the request
			result, err := s.handleMethod(s.ctx, req.Method, req.Params)
			resp := Response{
				JSONRPC: "2.0",
				ID:      req.ID,
			}

			if err != nil {
				rpcErr, ok := err.(*Error)
				if !ok {
					rpcErr = &Error{
						Code:    InternalError,
						Message: err.Error(),
					}
				}
				resp.Error = rpcErr
			} else {
				resp.Result = result
			}

			if err := encoder.Encode(resp); err != nil {
				return
			}
		}
	}
}

// handleMethod dispatches a method to its handler.
func (s *Server) handleMethod(ctx context.Context, method string, params json.RawMessage) (interface{}, error) {
	s.mu.RLock()
	handler, ok := s.handlers[method]
	s.mu.RUnlock()

	if !ok {
		return nil, &Error{
			Code:    MethodNotFound,
			Message: "Method not found: " + method,
		}
	}

	// Recover from panics in handlers
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic in handler %s: %v\n", method, r)
		}
	}()

	return handler(ctx, params)
}

// registerDaemonHandlers registers daemon.* methods.
func (s *Server) registerDaemonHandlers() {
	s.Register("daemon.status", s.handleDaemonStatus)
	s.Register("daemon.start", s.handleDaemonStart)
	s.Register("daemon.stop", s.handleDaemonStop)
	s.Register("daemon.restart", s.handleDaemonRestart)
}

// registerMetricsHandlers registers metrics.* methods.
func (s *Server) registerMetricsHandlers() {
	s.Register("metrics.snapshot", s.handleMetricsSnapshot)
	s.Register("metrics.services", s.handleMetricsServices)
}

// handleDaemonStatus handles daemon.status method.
func (s *Server) handleDaemonStatus(ctx context.Context, params json.RawMessage) (interface{}, error) {
	services := s.supervisor.GetStatus()
	
	// Build response from service info
	serviceList := make([]map[string]interface{}, len(services))
	for i, svc := range services {
		serviceList[i] = map[string]interface{}{
			"name":   svc.Name,
			"status": svc.Status,
			"pid":    svc.PID,
		}
	}
	
	return map[string]interface{}{
		"uptime_s":  0, // TODO: track supervisor start time
		"pid":       os.Getpid(),
		"version":   "1.0.0",
		"services":  serviceList,
	}, nil
}

// handleDaemonStart handles daemon.start method.
func (s *Server) handleDaemonStart(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if err := s.supervisor.Start(); err != nil {
		return nil, &Error{
			Code:    InternalError,
			Message: "Failed to start daemon: " + err.Error(),
		}
	}
	return map[string]interface{}{"ok": true}, nil
}

// handleDaemonStop handles daemon.stop method.
func (s *Server) handleDaemonStop(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p struct {
		Graceful bool `json:"graceful"`
	}
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &Error{
				Code:    InvalidParams,
				Message: "Invalid params: " + err.Error(),
			}
		}
	}

	if err := s.supervisor.Stop(p.Graceful); err != nil {
		return nil, &Error{
			Code:    InternalError,
			Message: "Failed to stop daemon: " + err.Error(),
		}
	}
	return map[string]interface{}{"ok": true}, nil
}

// handleDaemonRestart handles daemon.restart method.
func (s *Server) handleDaemonRestart(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if err := s.supervisor.Restart(); err != nil {
		return nil, &Error{
			Code:    InternalError,
			Message: "Failed to restart daemon: " + err.Error(),
		}
	}
	return map[string]interface{}{"ok": true}, nil
}

// handleMetricsSnapshot handles metrics.snapshot method.
func (s *Server) handleMetricsSnapshot(ctx context.Context, params json.RawMessage) (interface{}, error) {
	servicePIDs := s.getServicePIDs()
	snap, err := s.collector.Collect(ctx, servicePIDs)
	if err != nil {
		return nil, &Error{
			Code:    InternalError,
			Message: "Failed to collect metrics: " + err.Error(),
		}
	}
	return snap, nil
}

// handleMetricsServices handles metrics.services method.
func (s *Server) handleMetricsServices(ctx context.Context, params json.RawMessage) (interface{}, error) {
	servicePIDs := s.getServicePIDs()
	metrics := s.collector.GetServiceMetrics(servicePIDs)
	return metrics, nil
}

// getServicePIDs extracts PIDs from supervisor status.
func (s *Server) getServicePIDs() map[string]int {
	pids := make(map[string]int)
	services := s.supervisor.GetStatus()
	for _, svc := range services {
		if svc.PID > 0 {
			pids[svc.Name] = svc.PID
		}
	}
	return pids
}
