package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"dendro-chronology-workbench/internal/repository"
	"dendro-chronology-workbench/internal/web"
	"dendro-chronology-workbench/internal/workflow"
)

type config struct {
	address   string
	dataDir   string
	selfCheck bool
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("服务退出", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	if cfg.selfCheck {
		temp, err := os.MkdirTemp("", "dendro-workbench-self-check-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(temp)
		cfg.dataDir = temp
	}
	store, err := repository.Open(cfg.dataDir)
	if err != nil {
		return fmt.Errorf("打开本地证据库: %w", err)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	service := workflow.New(store)
	api := web.New(service, logger)
	listener, err := net.Listen("tcp", cfg.address)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", cfg.address, err)
	}
	httpServer := &http.Server{Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	serveErr := make(chan error, 1)
	go func() {
		err := httpServer.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()
	logger.Info("树木年轮年代裁定工作台已启动", "address", listener.Addr().String(), "data_dir", cfg.dataDir)
	if cfg.selfCheck {
		checkCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		err := runSelfCheck(checkCtx, "http://"+listener.Addr().String())
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		stopErr := httpServer.Shutdown(shutdownCtx)
		if err != nil {
			return err
		}
		if stopErr != nil {
			return stopErr
		}
		fmt.Println("SELF_CHECK_OK: 完成含异常整改、幂等重放、修订冲突、独立复核与封存的真实 HTTP 全流程")
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}

func parseConfig(args []string) (config, error) {
	fs := flag.NewFlagSet("dendro-workbench", flag.ContinueOnError)
	cfg := config{}
	fs.StringVar(&cfg.address, "addr", "127.0.0.1:19081", "HTTP 监听地址，仅允许回环地址")
	fs.StringVar(&cfg.dataDir, "data-dir", filepath.Join(".", "data"), "本地证据数据目录")
	fs.BoolVar(&cfg.selfCheck, "self-check", false, "运行真实 HTTP 全流程自检后退出")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if fs.NArg() != 0 {
		return cfg, fmt.Errorf("不支持位置参数")
	}
	addrSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "addr" {
			addrSet = true
		}
	})
	if !addrSet {
		if portText := os.Getenv("PORT"); portText != "" {
			port, err := strconv.Atoi(portText)
			if err != nil || port < 1024 || port > 65535 {
				return cfg, fmt.Errorf("PORT 必须是 1024 至 65535 的端口号")
			}
			cfg.address = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		}
	}
	if err := validateAddress(cfg.address); err != nil {
		return cfg, err
	}
	if cfg.dataDir == "" {
		return cfg, fmt.Errorf("data-dir 不能为空")
	}
	return cfg, nil
}

func validateAddress(addr string) error {
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("addr 必须为 host:port: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("addr 仅允许 127.0.0.1 或其他明确回环 IP，拒绝 %q", host)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1024 || port > 65535 {
		return fmt.Errorf("addr 端口必须在 1024 至 65535 之间")
	}
	return nil
}
