package http_server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/example/click-counter/internal/entity"
	"github.com/example/click-counter/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

type Server struct {
	log     *zap.Logger
	addr    string
	agg     service.AggregatorPort
	stats   service.StatsReaderPort
	maxDays int
	httpSrv *http.Server
}

func NewServer(log *zap.Logger, addr string, agg service.AggregatorPort, stats service.StatsReaderPort, maxDays int) *Server {
	s := &Server{log: log, addr: addr, agg: agg, stats: stats, maxDays: maxDays}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(zapLogger(log))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	r.Get("/counter/{bannerID}", s.handleCounter())
	r.Post("/stats/{bannerID}", s.handleStats())

	s.httpSrv = &http.Server{Addr: addr, Handler: r}
	return s
}

func (s *Server) Start() error {
	s.log.Info("http listen", zap.String("addr", s.addr))
	return s.httpSrv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

func zapLogger(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)
			log.Info("http",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Int("bytes", ww.BytesWritten()),
				zap.String("request_id", middleware.GetReqID(r.Context())),
				zap.Duration("latency", time.Since(start)),
			)
		})
	}
}

func (s *Server) handleCounter() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseBannerID(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.agg.Inc(id, time.Now())
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseBannerID(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req entity.StatsRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		from, err := parseISO(req.From)
		if err != nil {
			http.Error(w, "invalid from", http.StatusBadRequest)
			return
		}
		to, err := parseISO(req.To)
		if err != nil {
			http.Error(w, "invalid to", http.StatusBadRequest)
			return
		}
		if !to.After(from) {
			http.Error(w, "to must be after from", http.StatusBadRequest)
			return
		}

		if s.maxDays > 0 && to.Sub(from) > (time.Hour*24*time.Duration(s.maxDays)) {
			http.Error(w, "range too large", http.StatusBadRequest)
			return
		}

		from = from.Truncate(time.Minute).UTC()
		to = to.Truncate(time.Minute).UTC()

		pts, err := s.stats.QueryRange(r.Context(), id, from, to)
		if err != nil {
			s.log.Error("query", zap.Error(err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entity.StatsResponse{Stats: pts})
	}
}

func parseBannerID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "bannerID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid bannerID")
	}
	return id, nil
}

func parseISO(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("empty")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, errors.New("bad time")
}
