package control

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/OrenVill/Argus/internal/config"
	"github.com/OrenVill/Argus/internal/meta"
	iredis "github.com/OrenVill/Argus/internal/redis"
)

// Server exposes the §8 control API.
type Server struct {
	cfg          config.Control
	meta         meta.Meta
	store        *iredis.Store
	defaultGrace time.Duration
	httpServer   *http.Server
}

// NewServer builds a control API listener (not started).
func NewServer(cfg config.Control, m meta.Meta, store *iredis.Store, defaultGrace time.Duration) *Server {
	return &Server{
		cfg:          cfg,
		meta:         m,
		store:        store,
		defaultGrace: defaultGrace,
	}
}

// Start listens until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	api := chi.NewRouter()
	api.Use(s.bearerMiddleware)
	api.Post("/api/v1/templates/{hash}/status", s.handleTemplateStatus)
	api.Post("/api/v1/deploy/grace-window", s.handleGraceWindow)
	r.Mount("/", api)

	s.httpServer = &http.Server{
		Addr:    s.cfg.Listen,
		Handler: r,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.httpServer.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *Server) bearerMiddleware(next http.Handler) http.Handler {
	token := strings.TrimSpace(s.cfg.BearerToken)
	if token == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(auth, prefix) || strings.TrimSpace(auth[len(prefix):]) != token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusBody struct {
	Status string `json:"status"`
}

func (s *Server) handleTemplateStatus(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	var body statusBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := s.store.SetTemplateStatus(r.Context(), s.meta, hash, body.Status); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type graceBody struct {
	Service  string `json:"service"`
	Duration string `json:"duration"`
}

func (s *Server) handleGraceWindow(w http.ResponseWriter, r *http.Request) {
	var body graceBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	service := strings.TrimSpace(body.Service)
	if service == "" {
		service = s.meta.Service
	}
	dur := s.defaultGrace
	if body.Duration != "" {
		parsed, err := time.ParseDuration(body.Duration)
		if err != nil {
			http.Error(w, "invalid duration", http.StatusBadRequest)
			return
		}
		dur = parsed
	}
	deadline := time.Now().UTC().Add(dur)
	ttl := dur + time.Hour
	if err := s.store.SetGraceDeadline(r.Context(), s.meta, service, deadline, ttl); err != nil {
		http.Error(w, fmt.Sprintf("set grace: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
