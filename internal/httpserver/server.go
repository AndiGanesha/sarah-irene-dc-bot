package httpserver

import (
	"context"
	"net/http"
	"net/http/pprof"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	addr string
	mux  *http.ServeMux
	hsrv *http.Server
}

func New(addr string, reg *prometheus.Registry) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200); w.Write([]byte("ok")) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200); w.Write([]byte("ready")) })
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	hsrv := &http.Server{Addr: addr, Handler: mux}
	return &Server{addr: addr, mux: mux, hsrv: hsrv}
}

func (s *Server) Start() error {
	return s.hsrv.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	return s.hsrv.Shutdown(ctx)
}
