package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/b4moss/es4/packages/go/pkg/es4"
)

// Server is the HTTP Es4 Server (single process = one Store).
type Server struct {
	db  *es4.DB
	txs *txRegistry
	mux *http.ServeMux
}

// New wraps db with the public HTTP surface (/livez, /readyz, /v1/...).
// Canonical paths never end with '/'; trailing-slash requests are not
// registered and therefore return 404 with no redirect.
func New(db *es4.DB) *Server {
	s := &Server{
		db:  db,
		txs: newTxRegistry(),
		mux: http.NewServeMux(),
	}
	s.routes()
	return s
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// Close rolls back any open server-tracked transactions. The caller closes db.
func (s *Server) Close() {
	s.txs.rollbackAll()
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /livez", s.handleLivez)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)

	s.mux.HandleFunc("PUT /v1/keys/{key...}", s.handleStateKey)
	s.mux.HandleFunc("GET /v1/keys/{key...}", s.handleStateKey)
	s.mux.HandleFunc("DELETE /v1/keys/{key...}", s.handleStateKey)
	s.mux.HandleFunc("HEAD /v1/keys/{key...}", s.handleStateKey)
	// Empty-key equivalents → invalid_key (not trailing-slash 404).
	s.mux.HandleFunc("PUT /v1/keys", s.handleEmptyKey)
	s.mux.HandleFunc("GET /v1/keys", s.handleEmptyKey)
	s.mux.HandleFunc("DELETE /v1/keys", s.handleEmptyKey)
	s.mux.HandleFunc("HEAD /v1/keys", s.handleEmptyKey)

	s.mux.HandleFunc("POST /v1/clear", s.handleClear)

	s.mux.HandleFunc("POST /v1/tx", s.handleTxBegin)

	s.mux.HandleFunc("PUT /v1/tx/{id}/keys/{key...}", s.handleTxKey)
	s.mux.HandleFunc("GET /v1/tx/{id}/keys/{key...}", s.handleTxKey)
	s.mux.HandleFunc("DELETE /v1/tx/{id}/keys/{key...}", s.handleTxKey)
	s.mux.HandleFunc("HEAD /v1/tx/{id}/keys/{key...}", s.handleTxKey)
	s.mux.HandleFunc("PUT /v1/tx/{id}/keys", s.handleTxEmptyKey)
	s.mux.HandleFunc("GET /v1/tx/{id}/keys", s.handleTxEmptyKey)
	s.mux.HandleFunc("DELETE /v1/tx/{id}/keys", s.handleTxEmptyKey)
	s.mux.HandleFunc("HEAD /v1/tx/{id}/keys", s.handleTxEmptyKey)

	s.mux.HandleFunc("POST /v1/tx/{id}/clear", s.handleTxClear)
	s.mux.HandleFunc("POST /v1/tx/{id}/commit", s.handleTxCommit)
	s.mux.HandleFunc("POST /v1/tx/{id}/rollback", s.handleTxRollback)
}

func (s *Server) handleLivez(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if !s.db.Ready() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) requireReady(w http.ResponseWriter) bool {
	if s.db.Ready() {
		return true
	}
	writeError(w, http.StatusServiceUnavailable, CodeNotReady, "restore not ready")
	return false
}

func (s *Server) handleEmptyKey(w http.ResponseWriter, _ *http.Request) {
	if !s.requireReady(w) {
		return
	}
	writeError(w, http.StatusBadRequest, CodeInvalidKey, "empty key")
}

func (s *Server) handleStateKey(w http.ResponseWriter, r *http.Request) {
	if !s.requireReady(w) {
		return
	}
	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, CodeInvalidKey, "empty key")
		return
	}
	ctx := r.Context()
	switch r.Method {
	case http.MethodPut:
		body, err := readJSONBody(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeInvalidValue, err.Error())
			return
		}
		if err := s.db.Set(ctx, key, body); err != nil {
			mapStateError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		val, err := s.db.Get(ctx, key)
		if err != nil {
			mapStateError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(val)
	case http.MethodDelete:
		if err := s.db.Delete(ctx, key); err != nil {
			mapStateError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	case http.MethodHead:
		ok, err := s.db.Exists(ctx, key)
		if err != nil {
			mapStateError(w, err)
			return
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleClear(w http.ResponseWriter, r *http.Request) {
	if !s.requireReady(w) {
		return
	}
	if err := s.db.Clear(r.Context()); err != nil {
		mapStateError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleTxBegin(w http.ResponseWriter, r *http.Request) {
	if !s.requireReady(w) {
		return
	}
	tx, err := s.db.BeginTx(r.Context())
	if err != nil {
		mapStateError(w, err)
		return
	}
	id := s.txs.begin(tx)
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

func (s *Server) txOrError(w http.ResponseWriter, id string) (es4.Tx, bool) {
	tx, conflict, notFound := s.txs.lookup(id)
	if notFound {
		writeError(w, http.StatusNotFound, CodeNotFound, "transaction not found")
		return nil, false
	}
	if conflict {
		writeError(w, http.StatusConflict, CodeConflict, "transaction finished")
		return nil, false
	}
	return tx, true
}

func (s *Server) handleTxEmptyKey(w http.ResponseWriter, r *http.Request) {
	if !s.requireReady(w) {
		return
	}
	if _, ok := s.txOrError(w, r.PathValue("id")); !ok {
		return
	}
	writeError(w, http.StatusBadRequest, CodeInvalidKey, "empty key")
}

func (s *Server) handleTxKey(w http.ResponseWriter, r *http.Request) {
	if !s.requireReady(w) {
		return
	}
	tx, ok := s.txOrError(w, r.PathValue("id"))
	if !ok {
		return
	}
	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, CodeInvalidKey, "empty key")
		return
	}
	ctx := r.Context()
	switch r.Method {
	case http.MethodPut:
		body, err := readJSONBody(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeInvalidValue, err.Error())
			return
		}
		if err := tx.Set(ctx, key, body); err != nil {
			mapStateError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		val, err := tx.Get(ctx, key)
		if err != nil {
			mapStateError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(val)
	case http.MethodDelete:
		if err := tx.Delete(ctx, key); err != nil {
			mapStateError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	case http.MethodHead:
		ok, err := tx.Exists(ctx, key)
		if err != nil {
			mapStateError(w, err)
			return
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTxClear(w http.ResponseWriter, r *http.Request) {
	if !s.requireReady(w) {
		return
	}
	tx, ok := s.txOrError(w, r.PathValue("id"))
	if !ok {
		return
	}
	if err := tx.Clear(r.Context()); err != nil {
		mapStateError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleTxCommit(w http.ResponseWriter, r *http.Request) {
	if !s.requireReady(w) {
		return
	}
	id := r.PathValue("id")
	tx, ok := s.txOrError(w, id)
	if !ok {
		return
	}
	if err := tx.Commit(); err != nil {
		mapStateError(w, err)
		return
	}
	s.txs.markDone(id)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleTxRollback(w http.ResponseWriter, r *http.Request) {
	if !s.requireReady(w) {
		return
	}
	id := r.PathValue("id")
	tx, ok := s.txOrError(w, id)
	if !ok {
		return
	}
	if err := tx.Rollback(); err != nil {
		mapStateError(w, err)
		return
	}
	s.txs.markDone(id)
	w.WriteHeader(http.StatusOK)
}

func readJSONBody(r *http.Request) (json.RawMessage, error) {
	defer r.Body.Close()
	data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("empty body")
	}
	if !json.Valid(data) {
		return nil, errors.New("invalid JSON")
	}
	return json.RawMessage(data), nil
}
