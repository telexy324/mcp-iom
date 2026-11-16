package main

import (
	"bank-ops-mcp/internal/config"
	"bank-ops-mcp/internal/provider"
	"bank-ops-mcp/internal/server"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	c, e := config.Load()
	if e != nil {
		return e
	}
	var level slog.Level
	if e = level.UnmarshalText([]byte(c.LogLevel)); e != nil {
		return e
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)
	var p provider.OpsProvider
	if c.Mode == "mock" {
		p, e = provider.NewMock(c.MockDir)
		if e != nil {
			return e
		}
	} else {
		p = provider.NewHTTP(c)
	}
	srv := &http.Server{Addr: net.JoinHostPort(c.Addr, c.Port), Handler: server.New(c, p, log), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { log.Info("listening", "address", srv.Addr, "data_mode", c.Mode); done <- srv.ListenAndServe() }()
	select {
	case e := <-done:
		if !errors.Is(e, http.ErrServerClosed) {
			return e
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
	return nil
}
