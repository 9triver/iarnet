package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config Unikernel provider 配置
type Config struct {
	Server             ServerConfig    `yaml:"server"`
	Resource           ResourceConfig  `yaml:"resource"`
	ResourceTags       []string        `yaml:"resource_tags"`
	SupportedLanguages []string        `yaml:"supported_languages"`
	DNS                DNSConfig       `yaml:"dns"`       // DNS 配置
	WebSocket          WebSocketConfig `yaml:"websocket"` // WebSocket 服务器配置
	Unikernel          UnikernelConfig `yaml:"unikernel"` // Unikernel 配置
}

// ServerConfig gRPC 服务器配置
type ServerConfig struct {
	Port int `yaml:"port"`
}

// ResourceConfig 资源容量配置
type ResourceConfig struct {
	CPU    int64  `yaml:"cpu"`    // CPU 容量，单位：millicores (1000 millicores = 1 core)
	Memory string `yaml:"memory"` // 内存容量，支持格式：8Gi, 8GB, 8192Mi, 8192MB 等
	GPU    int64  `yaml:"gpu"`    // GPU 数量
}

// DNSConfig DNS 配置
type DNSConfig struct {
	Hosts map[string]string `yaml:"hosts"` // 主机名到 IP 地址的映射，例如：{"host.internal": "localhost"}
}

// WebSocketConfig WebSocket 服务器配置
type WebSocketConfig struct {
	Port int `yaml:"port"` // WebSocket 服务器端口
}

// UnikernelConfig Unikernel 配置
type UnikernelConfig struct {
	BaseDir      string `yaml:"base_dir"`       // unikernel 代码基础目录（mirage-websocket submodule 所在目录）
	Solo5HvtPath string `yaml:"solo5_hvt_path"` // solo5-hvt 可执行文件路径，为空则从 PATH 中查找
}

// ParseMemory 解析内存字符串为字节数
// 支持格式：8Gi, 8GB, 8192Mi, 8192MB, 8192, 8G, 8M 等
func (r *ResourceConfig) ParseMemory() (int64, error) {
	if r.Memory == "" {
		return 0, nil
	}

	// 使用与 process provider 相同的解析逻辑
	memoryStr := r.Memory
	// 移除空格并转换为小写
	memoryStr = strings.TrimSpace(strings.ToLower(memoryStr))

	// 正则表达式匹配数字和单位
	re := regexp.MustCompile(`^(\d+)([kmgt]?i?b?)$`)
	matches := re.FindStringSubmatch(memoryStr)
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid memory format: %s, expected format like 8Gi, 8GB, 8192Mi", r.Memory)
	}

	value, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory value: %s", matches[1])
	}

	unit := matches[2]
	var multiplier int64

	switch unit {
	case "b", "":
		multiplier = 1
	case "kb", "k":
		multiplier = 1000
	case "kib", "ki":
		multiplier = 1024
	case "mb", "m":
		multiplier = 1000 * 1000
	case "mib", "mi":
		multiplier = 1024 * 1024
	case "gb", "g":
		multiplier = 1000 * 1000 * 1000
	case "gib", "gi":
		multiplier = 1024 * 1024 * 1024
	case "tb", "t":
		multiplier = 1000 * 1000 * 1000 * 1000
	case "tib", "ti":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown memory unit: %s", unit)
	}

	return value * multiplier, nil
}

func LoadConfig(path string) (*Config, error) {
	var config Config

	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	} else {
		config = getDefaultConfig()
	}

	return &config, nil
}

func getDefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Port: 50051,
		},
		Resource: ResourceConfig{
			CPU:    0,
			Memory: "",
			GPU:    0,
		},
		ResourceTags:       []string{"cpu", "memory"},
		SupportedLanguages: []string{"unikernel"}, // 默认支持 unikernel
		WebSocket: WebSocketConfig{
			Port: 8080, // 默认 WebSocket 端口
		},
		Unikernel: UnikernelConfig{
			BaseDir:      "", // 默认使用 providers/unikernel 目录
			Solo5HvtPath: "", // 默认从 PATH 中查找
		},
	}
}
