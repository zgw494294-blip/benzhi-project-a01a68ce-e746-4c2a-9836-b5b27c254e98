package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"sensory-blind-review/internal/application"
	"sensory-blind-review/internal/archive"
	"sensory-blind-review/internal/httpui"
	"sensory-blind-review/internal/repository"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:19081", "监听地址")
	selfcheck := flag.Bool("selfcheck", false, "执行端到端自检并退出")
	dataDir := flag.String("data", "./data", "数据目录")
	flag.Parse()
	if envPort := strings.TrimSpace(os.Getenv("PORT")); envPort != "" && flag.Lookup("addr") != nil && *addr == "127.0.0.1:19081" {
		*addr = "127.0.0.1:" + envPort
	}
	if err := validateAddr(*addr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *selfcheck {
		if err := runSelfcheck(); err != nil {
			fmt.Fprintln(os.Stderr, "自检失败:", err)
			os.Exit(1)
		}
		fmt.Println("自检通过：盲样评审主流程可用")
		return
	}
	store, err := repository.Open(filepath.Clean(*dataDir))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	receipts, err := archive.NewRegistryAt(filepath.Clean(*dataDir))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	app := application.New(store, archive.NewService(), receipts)
	server := &http.Server{Addr: *addr, Handler: httpui.New(app).Handler(), ReadHeaderTimeout: 5 * time.Second}
	fmt.Println("盲样感官评审服务监听", *addr)
	if err := serve(server); err != nil {
		fmt.Fprintln(os.Stderr, "服务退出:", err)
		os.Exit(1)
	}
}

func validateAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("无效监听地址 %q", addr)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return fmt.Errorf("监听地址必须绑定回环地址")
	}
	if port == "80" || port == "8080" || port == "3000" {
		return fmt.Errorf("禁止使用常见低位端口")
	}
	return nil
}

func runSelfcheck() error {
	dir, err := os.MkdirTemp("", "sensory-selfcheck-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	store, err := repository.Open(dir)
	if err != nil {
		return err
	}
	receipts, err := archive.NewRegistryAt(dir)
	if err != nil {
		return err
	}
	app := application.New(store, archive.NewService(), receipts)
	return httpui.RunSelfcheck(app)
}

func serve(server *http.Server) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	select {
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}
