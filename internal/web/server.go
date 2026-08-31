package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type Config struct {
	Registry          string
	From              string
	Vantage           string
	MechanismBindings string
	Address           string
}

type Server struct {
	http *http.Server
}

func New(config Config) (*Server, error) {
	handler, err := newHandler(config)
	if err != nil {
		return nil, err
	}
	return &Server{http: &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}}, nil
}

func (s *Server) ListenAndServe(ctx context.Context, address string, output io.Writer) error {
	listener, err := listenLoopback(address)
	if err != nil {
		return err
	}
	defer listener.Close()
	fmt.Fprintf(output, "Locus Web listening at http://%s\n", listener.Addr())

	result := make(chan error, 1)
	go func() { result <- s.http.Serve(listener) }()
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.http.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return ctx.Err()
	}
}

func listenLoopback(address string) (net.Listener, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid web address %q: %w", address, err)
	}
	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return nil, fmt.Errorf("web address must use a loopback host")
		}
	}
	return net.Listen("tcp", address)
}
