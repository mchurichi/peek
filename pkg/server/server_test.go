package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mchurichi/peek/pkg/storage"
)

func newTestStorage(t *testing.T) *storage.BadgerStorage {
	t.Helper()
	db, err := storage.NewBadgerStorage(storage.Config{DBPath: t.TempDir(), RetentionSize: 1024 * 1024 * 100, RetentionDays: 7})
	if err != nil {
		t.Fatalf("NewBadgerStorage() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func storeLog(t *testing.T, db *storage.BadgerStorage, id, level, msg string, ts time.Time, fields map[string]interface{}) {
	t.Helper()
	if err := db.Store(&storage.LogEntry{ID: id, Timestamp: ts, Level: level, Message: msg, Fields: fields, Raw: msg}); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
}

func TestHTTPHandlers(t *testing.T) {
	db := newTestStorage(t)
	now := time.Now().UTC()
	storeLog(t, db, "1", "INFO", "started", now.Add(-2*time.Minute), map[string]interface{}{"service": "api"})
	storeLog(t, db, "2", "ERROR", "failed", now.Add(-time.Minute), map[string]interface{}{"service": "worker"})

	s := NewServer(db, nil)

	tests := []struct {
		name       string
		method     string
		target     string
		body       string
		handler    func(http.ResponseWriter, *http.Request)
		wantStatus int
		check      func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:       "health",
			method:     http.MethodGet,
			target:     "/health",
			handler:    s.handleHealth,
			wantStatus: http.StatusOK,
			check: func(t *testing.T, rr *httptest.ResponseRecorder) {
				t.Helper()
				if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
					t.Fatalf("content-type = %q", ct)
				}
			},
		},
		{name: "stats", method: http.MethodGet, target: "/stats", handler: s.handleStats, wantStatus: http.StatusOK},
		{name: "query method check", method: http.MethodGet, target: "/query", handler: s.handleQuery, wantStatus: http.StatusMethodNotAllowed},
		{name: "query invalid json", method: http.MethodPost, target: "/query", body: "{", handler: s.handleQuery, wantStatus: http.StatusBadRequest},
		{name: "query invalid lucene", method: http.MethodPost, target: "/query", body: `{"query":"level:[bad"}`, handler: s.handleQuery, wantStatus: http.StatusBadRequest},
		{
			name:       "query defaults and time range",
			method:     http.MethodPost,
			target:     "/query",
			wantStatus: http.StatusOK,
			handler:    s.handleQuery,
			body:       mustJSON(t, map[string]interface{}{"query": "*", "start": now.Add(-90 * time.Second).Format(time.RFC3339), "end": now.Add(5 * time.Second).Format(time.RFC3339)}),
			check: func(t *testing.T, rr *httptest.ResponseRecorder) {
				t.Helper()
				var resp struct {
					Logs  []storage.LogEntry `json:"logs"`
					Total int                `json:"total"`
				}
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if resp.Total != 1 || len(resp.Logs) != 1 {
					t.Fatalf("unexpected query result: total=%d len=%d", resp.Total, len(resp.Logs))
				}
			},
		},
		{name: "fields", method: http.MethodGet, target: "/fields?start=invalid&end=invalid", handler: s.handleFields, wantStatus: http.StatusOK},
		{name: "fields method not allowed", method: http.MethodPost, target: "/fields", handler: s.handleFields, wantStatus: http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.target, bytes.NewBufferString(tt.body))
			rr := httptest.NewRecorder()
			tt.handler(rr, req)
			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
			}
			if tt.check != nil {
				tt.check(t, rr)
			}
		})
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	return string(b)
}

func TestWebSocketSubscribeAndBroadcast(t *testing.T) {
	db := newTestStorage(t)
	now := time.Now().UTC()
	storeLog(t, db, "1", "ERROR", "boom", now.Add(-time.Second), map[string]interface{}{"service": "api"})
	storeLog(t, db, "2", "INFO", "ok", now, map[string]interface{}{"service": "api"})

	s := NewServer(db, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("/logs", s.handleWebSocket)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/logs"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]string{"action": "subscribe", "query": "level:ERROR"}); err != nil {
		t.Fatalf("WriteJSON subscribe: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var first map[string]interface{}
	if err := conn.ReadJSON(&first); err != nil {
		t.Fatalf("ReadJSON initial: %v", err)
	}
	if first["type"] != "results" {
		t.Fatalf("expected results message, got %#v", first)
	}

	s.BroadcastLog(&storage.LogEntry{ID: "3", Timestamp: time.Now(), Level: "INFO", Message: "skip", Fields: map[string]interface{}{}, Raw: "skip"})
	s.BroadcastLog(&storage.LogEntry{ID: "4", Timestamp: time.Now(), Level: "ERROR", Message: "pass", Fields: map[string]interface{}{}, Raw: "pass"})

	var msg map[string]interface{}
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("ReadJSON broadcast: %v", err)
	}
	if msg["type"] != "log" {
		t.Fatalf("expected log message, got %#v", msg)
	}

	if err := conn.WriteJSON(map[string]string{"action": "unsubscribe"}); err != nil {
		t.Fatalf("WriteJSON unsubscribe: %v", err)
	}
}

func TestStaticHandlers(t *testing.T) {
	db := newTestStorage(t)
	s := NewServer(db, nil)

	tests := []struct {
		name       string
		target     string
		handler    func(http.ResponseWriter, *http.Request)
		wantStatus int
		check      func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:       "index",
			target:     "/",
			handler:    s.handleIndex,
			wantStatus: http.StatusOK,
			check: func(t *testing.T, rr *httptest.ResponseRecorder) {
				t.Helper()
				if !strings.Contains(rr.Body.String(), "<") {
					t.Fatalf("unexpected index response")
				}
			},
		},
		{
			name:       "vanjs",
			target:     "/van.min.js",
			handler:    s.handleVanJS,
			wantStatus: http.StatusOK,
			check: func(t *testing.T, rr *httptest.ResponseRecorder) {
				t.Helper()
				if rr.Header().Get("Cache-Control") == "" {
					t.Fatalf("expected cache-control header")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			rr := httptest.NewRecorder()
			tt.handler(rr, req)
			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d", rr.Code)
			}
			tt.check(t, rr)
		})
	}
}

func TestStartBroadcastWorkerSendsNewEntries(t *testing.T) {
	db := newTestStorage(t)
	s := NewServer(db, nil)

	c := &client{
		send:   make(chan interface{}, 10),
		done:   make(chan struct{}),
		filter: &storage.AllFilter{},
	}

	s.mu.Lock()
	s.clients[nil] = c
	s.mu.Unlock()

	s.StartBroadcastWorker()
	time.Sleep(120 * time.Millisecond)
	storeLog(t, db, "late", "INFO", "late log", time.Now().UTC(), map[string]interface{}{})

	select {
	case <-c.send:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for broadcast worker message")
	}

	close(c.done)
}

func TestStartServesHTTP(t *testing.T) {
	db := newTestStorage(t)
	s := NewServer(db, nil)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	go func() {
		_ = s.Start(port)
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not become ready")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

type failingWriter struct {
	header http.Header
}

func (w *failingWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}
func (w *failingWriter) WriteHeader(statusCode int) {}
func (w *failingWriter) Write([]byte) (int, error)  { return 0, fmt.Errorf("write failed") }

func TestHandlerErrorsWhenStorageUnavailable(t *testing.T) {
	db, err := storage.NewBadgerStorage(storage.Config{DBPath: t.TempDir(), RetentionSize: 1024 * 1024 * 100, RetentionDays: 7})
	if err != nil {
		t.Fatalf("NewBadgerStorage() error = %v", err)
	}
	s := NewServer(db, nil)
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	tests := []struct {
		name       string
		handler    func(http.ResponseWriter, *http.Request)
		method     string
		target     string
		body       string
		wantStatus int
	}{
		{name: "health", handler: s.handleHealth, method: http.MethodGet, target: "/health", wantStatus: http.StatusInternalServerError},
		{name: "stats", handler: s.handleStats, method: http.MethodGet, target: "/stats", wantStatus: http.StatusInternalServerError},
		{name: "query", handler: s.handleQuery, method: http.MethodPost, target: "/query", body: `{"query":"*"}`, wantStatus: http.StatusInternalServerError},
		{name: "fields", handler: s.handleFields, method: http.MethodGet, target: "/fields", wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.target, bytes.NewBufferString(tt.body))
			tt.handler(rr, req)
			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d", rr.Code)
			}
		})
	}
}

func TestNewServerWithStartTimeAndVanJSWriteFailure(t *testing.T) {
	db := newTestStorage(t)
	start := time.Now().Add(-time.Minute)
	s := NewServer(db, &start)
	if s.defaultFilter == nil {
		t.Fatalf("expected default filter in fresh mode")
	}

	fw := &failingWriter{}
	s.handleVanJS(fw, httptest.NewRequest(http.MethodGet, "/van.min.js", nil))
}

func TestHandleWebSocketUpgradeFailureAndInitialResultsError(t *testing.T) {
	db, err := storage.NewBadgerStorage(storage.Config{DBPath: t.TempDir(), RetentionSize: 1024 * 1024 * 100, RetentionDays: 7})
	if err != nil {
		t.Fatalf("NewBadgerStorage() error = %v", err)
	}
	s := NewServer(db, nil)

	// Regular HTTP request without websocket headers should fail upgrade path gracefully.
	s.handleWebSocket(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/logs", nil))

	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	c := &client{send: make(chan interface{}, 1), done: make(chan struct{})}
	s.sendInitialResults(c, &storage.AllFilter{})
	select {
	case <-c.send:
		t.Fatalf("did not expect initial results when storage query fails")
	default:
	}

	close(c.done)
	s.sendInitialResults(c, &storage.AllFilter{})
}

func TestWritePumpExitsWhenDoneClosed(t *testing.T) {
	s := &Server{}
	c := &client{send: make(chan interface{}, 1), done: make(chan struct{})}
	close(c.done)
	s.writePump(c)
}

func TestSendInitialResultsHonorsDoneChannel(t *testing.T) {
	db := newTestStorage(t)
	storeLog(t, db, "seed", "INFO", "seed", time.Now().UTC(), map[string]interface{}{})
	s := NewServer(db, nil)
	c := &client{send: make(chan interface{}, 1), done: make(chan struct{})}
	close(c.done)
	s.sendInitialResults(c, &storage.AllFilter{})
}
