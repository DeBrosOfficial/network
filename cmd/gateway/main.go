package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.debros.io/DeBros/network/pkg/gateway"
	"git.debros.io/DeBros/network/pkg/logging"
	"go.uber.org/zap"
)

func setupLogger() *logging.ColoredLogger {
	logger, err := logging.NewColoredLogger(logging.ComponentGeneral, true)
	if err != nil {
		panic(err)
	}
	return logger
}

func main() {
	logger := setupLogger()

	// Load gateway config (flags/env)
	cfg := parseGatewayConfig(logger)

	// Initialize gateway (connect client, prepare routes)
	g, err := gateway.New(logger, cfg)
	if err != nil {
		logger.ComponentError(logging.ComponentGeneral, "failed to initialize gateway", zap.Error(err))
		os.Exit(1)
	}
	defer g.Close()

	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: g.Routes(),
	}

	// Start server
	go func() {
		logger.ComponentInfo(logging.ComponentGeneral, "Gateway HTTP server starting",
			zap.String("addr", cfg.ListenAddr),
			zap.String("namespace", cfg.ClientNamespace),
			zap.Int("bootstrap_peer_count", len(cfg.BootstrapPeers)),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.ComponentError(logging.ComponentGeneral, "HTTP server error", zap.Error(err))
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.ComponentInfo(logging.ComponentGeneral, "Shutting down gateway HTTP server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.ComponentError(logging.ComponentGeneral, "HTTP server shutdown error", zap.Error(err))
	}
	logger.ComponentInfo(logging.ComponentGeneral, "Gateway shutdown complete")
}
