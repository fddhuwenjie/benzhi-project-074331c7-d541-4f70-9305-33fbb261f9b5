package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type config struct {
	addr      string
	dataDir   string
	selfCheck bool
}

func parseConfig() (config, error) {
	defaultAddr := "127.0.0.1:19081"
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		if _, err := strconv.Atoi(port); err != nil {
			return config{}, fmt.Errorf("PORT 必须是端口号: %w", err)
		}
		defaultAddr = net.JoinHostPort("127.0.0.1", port)
	}
	var cfg config
	flag.StringVar(&cfg.addr, "addr", defaultAddr, "HTTP 监听地址")
	flag.StringVar(&cfg.dataDir, "data-dir", "./data", "本地持久化目录")
	flag.BoolVar(&cfg.selfCheck, "self-check", false, "运行真实 HTTP 端到端自检后退出")
	flag.Parse()
	if err := validateAddress(cfg.addr); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(cfg.dataDir) == "" {
		return config{}, fmt.Errorf("data-dir 不能为空")
	}
	return cfg, nil
}

func validateAddress(addr string) error {
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("addr 必须采用 host:port 格式: %w", err)
	}
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("addr 必须包含明确主机")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("addr 端口非法")
	}
	return nil
}
