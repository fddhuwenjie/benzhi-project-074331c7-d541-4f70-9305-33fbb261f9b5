package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tactile-atlas-gate/internal/repository"
	"tactile-atlas-gate/internal/validation"
	"tactile-atlas-gate/internal/web"
	"tactile-atlas-gate/internal/workflow"
)

func main() {
	if err := run(); err != nil {
		log.Printf("服务退出: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := parseConfig()
	if err != nil {
		return err
	}
	dataDir := cfg.dataDir
	if cfg.selfCheck {
		dataDir, err = os.MkdirTemp("", "tactile-atlas-self-check-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dataDir)
	}
	store, err := repository.NewFileStore(dataDir)
	if err != nil {
		return fmt.Errorf("初始化存储: %w", err)
	}
	service := workflow.New(store, validation.NewEngine())
	server := &http.Server{
		Addr:              cfg.addr,
		Handler:           web.NewHandler(service),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	listener, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", cfg.addr, err)
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	if cfg.selfCheck {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		checkErr := runSelfCheck(ctx, listener.Addr().String())
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		shutdownErr := server.Shutdown(shutdownCtx)
		serverErr := <-serveErr
		if checkErr != nil {
			return checkErr
		}
		if shutdownErr != nil {
			return shutdownErr
		}
		if !errors.Is(serverErr, http.ErrServerClosed) {
			return serverErr
		}
		log.Printf("自检通过：完整发布流程与母版摘要有效")
		return nil
	}
	log.Printf("触觉导览图制版校验工作台监听于 http://%s", cfg.addr)
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err = <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-signalCtx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	}
}
