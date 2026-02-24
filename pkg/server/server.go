package server

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mchurichi/peek/pkg/query"
	"github.com/mchurichi/peek/pkg/storage"
)

//go:embed index.html
var indexHTML string

//go:embed van.min.js
var vanJS []byte

// Server represents the HTTP server
type Server struct {
	storage       *storage.BadgerStorage
	upgrader      websocket.Upgrader
	clients       map[*websocket.Conn]*client
	mu            sync.RWMutex
	defaultFilter query.Filter // Default filter applied to all queries (e.g., for fresh mode)
}

type client struct {
	conn      *websocket.Conn
	query     string
	filter    query.Filter
	timeRange *storage.TimeRange
	send      chan interface{} // carries *storage.LogEntry or pre-built map messages
	done      chan struct{}
}

// NewServer creates a new HTTP server
func NewServer(storage *storage.BadgerStorage, startTime *time.Time) *Server {
	s := &Server{
		storage: storage,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for local dev
			},
		},
		clients: make(map[*websocket.Conn]*client),
	}

	// If startTime is provided, create a default filter for fresh mode
	if startTime != nil {
		s.defaultFilter = &query.TimestampRangeFilter{
			Start: *startTime,
			End:   time.Time{}, // No end time
		}
	}

	return s
}

// Start starts the HTTP server
func (s *Server) Start(port int) error {
	mux := http.NewServeMux()

	// Serve static web UI
	mux.HandleFunc("/", s.handleIndex)

	// Serve bundled VanJS (avoids CDN dependency)
	mux.HandleFunc("/van.min.js", s.handleVanJS)

	// API endpoints
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/stats", s.handleStats)
	mux.HandleFunc("/query", s.handleQuery)
	mux.HandleFunc("/fields", s.handleFields)
	mux.HandleFunc("/logs", s.handleWebSocket)

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting server on http://localhost%s", addr)

	return http.ListenAndServe(addr, mux)
}

// handleVanJS serves the bundled VanJS library
func (s *Server) handleVanJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "max-age=86400")
	w.Write(vanJS)
}

// handleIndex serves the web UI
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(indexHTML))
}

// handleHealth handles GET /health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	stats, err := s.storage.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":         "ok",
		"logs_stored":    stats.TotalLogs,
		"db_size_bytes":  int64(stats.DBSizeMB * 1024 * 1024),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleStats handles GET /stats
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.storage.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"total_logs":  stats.TotalLogs,
		"db_size_mb":  stats.DBSizeMB,
		"levels":      stats.Levels,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleQuery handles POST /query
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Query  string `json:"query"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
		Start  string `json:"start"`
		End    string `json:"end"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Default values
	if req.Limit == 0 {
		req.Limit = 100
	}

	// Parse query
	queryStr := req.Query
	if queryStr == "" {
		queryStr = "*"
	}

	q, err := query.Parse(queryStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid query: %v", err), http.StatusBadRequest)
		return
	}

	// Apply default filter (e.g., for fresh mode)
	var filter query.Filter = q
	if s.defaultFilter != nil {
		filter = &query.AndFilter{
			Left:  s.defaultFilter,
			Right: q,
		}
	}

	// Parse optional time range parameters.
	var tr *storage.TimeRange
	var rangeStart, rangeEnd time.Time
	if req.Start != "" {
		if t, err := time.Parse(time.RFC3339, req.Start); err == nil {
			rangeStart = t
		}
	}
	if req.End != "" {
		if t, err := time.Parse(time.RFC3339, req.End); err == nil {
			rangeEnd = t
		}
	}
	if !rangeStart.IsZero() || !rangeEnd.IsZero() {
		tr = &storage.TimeRange{Start: rangeStart, End: rangeEnd}
		// Also apply the time range as a filter so boundary conditions are correct.
		filter = &query.AndFilter{
			Left:  filter,
			Right: &query.TimestampRangeFilter{Start: rangeStart, End: rangeEnd},
		}
	}

	// Execute query
	executionStart := time.Now()
	entries, total, err := s.storage.QueryWithTimeRange(filter, tr, req.Limit, req.Offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	took := time.Since(executionStart)
	
	// Ensure entries is never nil for JSON encoding
	if entries == nil {
		entries = []*storage.LogEntry{}
	}

	response := map[string]interface{}{
		"logs":    entries,
		"total":   total,
		"took_ms": took.Milliseconds(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleFields handles GET /fields
func (s *Server) handleFields(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	var start, end time.Time

	if v := q.Get("start"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			start = t
		}
	}
	if v := q.Get("end"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			end = t
		}
	}

	fields, err := s.storage.GetFields(start, end)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"fields": fields})
}

// handleWebSocket handles WS /logs
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	c := &client{
		conn: conn,
		send: make(chan interface{}, 100),
		done: make(chan struct{}),
	}

	s.mu.Lock()
	s.clients[conn] = c
	s.mu.Unlock()

	// Start sender goroutine
	go s.writePump(c)

	// Start reader goroutine
	go s.readPump(c)
}

// readPump reads messages from the WebSocket
func (s *Server) readPump(c *client) {
	defer func() {
		s.mu.Lock()
		delete(s.clients, c.conn)
		s.mu.Unlock()
		close(c.done)
		c.conn.Close()
	}()

	for {
		var msg struct {
			Action string `json:"action"`
			Query  string `json:"query"`
			Start  string `json:"start"`
			End    string `json:"end"`
		}

		if err := c.conn.ReadJSON(&msg); err != nil {
			break
		}

		if msg.Action == "subscribe" {
			queryStr := msg.Query
			if queryStr == "" {
				queryStr = "*"
			}

			c.query = queryStr

			// Parse query
			q, err := query.Parse(queryStr)
			if err != nil {
				log.Printf("Invalid query: %v", err)
				continue
			}

			// Apply default filter (e.g., for fresh mode)
			var filter query.Filter = q
			if s.defaultFilter != nil {
				filter = &query.AndFilter{
					Left:  s.defaultFilter,
					Right: q,
				}
			}

			// Parse optional time range.
			var tr *storage.TimeRange
			var rangeStart, rangeEnd time.Time
			if msg.Start != "" {
				if t, err := time.Parse(time.RFC3339, msg.Start); err == nil {
					rangeStart = t
				}
			}
			if msg.End != "" {
				if t, err := time.Parse(time.RFC3339, msg.End); err == nil {
					rangeEnd = t
				}
			}
			if !rangeStart.IsZero() || !rangeEnd.IsZero() {
				tr = &storage.TimeRange{Start: rangeStart, End: rangeEnd}
				filter = &query.AndFilter{
					Left:  filter,
					Right: &query.TimestampRangeFilter{Start: rangeStart, End: rangeEnd},
				}
			}

			c.filter = filter
			c.timeRange = tr

			// Send initial results
			go s.sendInitialResults(c, filter)

		} else if msg.Action == "unsubscribe" {
			c.filter = nil
			c.timeRange = nil
		}
	}
}

// writePump sends messages to the WebSocket — only goroutine allowed to write.
func (s *Server) writePump(c *client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Wrap raw log entries; pre-built maps pass through unchanged.
			var payload interface{}
			if entry, ok := msg.(*storage.LogEntry); ok {
				payload = map[string]interface{}{
					"type":  "log",
					"entry": entry,
				}
			} else {
				payload = msg
			}

			if err := c.conn.WriteJSON(payload); err != nil {
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.done:
			return
		}
	}
}

// sendInitialResults sends initial query results through the write channel.
func (s *Server) sendInitialResults(c *client, q query.Filter) {
	entries, total, err := s.storage.QueryWithTimeRange(q, c.timeRange, 100, 0)
	if err != nil {
		log.Printf("Query error: %v", err)
		return
	}

	msg := map[string]interface{}{
		"type":    "results",
		"logs":    entries,
		"total":   total,
		"took_ms": 0,
	}

	select {
	case c.send <- msg:
	case <-c.done:
	}
}

// BroadcastLog broadcasts a new log entry to all connected clients
func (s *Server) BroadcastLog(entry *storage.LogEntry) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, c := range s.clients {
		if c.filter == nil || c.filter.Match(entry) {
			select {
			case c.send <- entry:
			default:
				// Channel full, skip
			}
		}
	}
}

// StartBroadcastWorker starts a worker that broadcasts new logs
func (s *Server) StartBroadcastWorker() {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		lastCheck := time.Now()

		for range ticker.C {
			now := time.Now()

			// Query for new entries since last check
			entries := make([]*storage.LogEntry, 0, 100)
			s.storage.Scan(func(entry *storage.LogEntry) error {
				if entry.Timestamp.After(lastCheck) {
					entries = append(entries, entry)
				}
				return nil
			})

			// Broadcast new entries
			for _, entry := range entries {
				s.BroadcastLog(entry)
			}

			lastCheck = now
		}
	}()
}
