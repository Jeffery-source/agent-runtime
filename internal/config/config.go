package config

import (
	"encoding/json"
	"os"
	"time"
)

// Config 是 Agent Runtime 的启动配置。
type Config struct {
	Server    ServerConfig  `json:"server"`
	Gateway   GatewayConfig `json:"gateway"`
	Workers   int           `json:"workers"`
	QueueSize int           `json:"queue_size"`
	DataDir   string        `json:"data_dir"`
}

type ServerConfig struct {
	Addr string `json:"addr"`
}

type GatewayConfig struct {
	BaseURL string `json:"base_url"`
	Timeout string `json:"timeout"`
	Retries int    `json:"retries"`
}

// Default 返回带默认值的配置。
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Addr: ":8080",
		},
		Gateway: GatewayConfig{
			Timeout: "30s",
		},
		Workers:   4,
		QueueSize: 128,
		DataDir:   "./data",
	}
}

// Load 从 JSON 文件加载配置；文件不存在时返回默认配置。
func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// GatewayTimeout 解析网关超时，非法值回退为 30s。
func (c *Config) GatewayTimeout() time.Duration {
	if c.Gateway.Timeout == "" {
		return 30 * time.Second
	}
	d, err := time.ParseDuration(c.Gateway.Timeout)
	if err != nil {
		return 30 * time.Second
	}
	return d
}
