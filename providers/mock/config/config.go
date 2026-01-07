package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config Mock provider 配置
type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Resource     ResourceConfig     `yaml:"resource"`
	ResourceTags []string           `yaml:"resource_tags"`
	TaskDuration TaskDurationConfig `yaml:"task_duration"` // 任务执行时间配置
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

// TaskDurationConfig 任务执行时间配置
type TaskDurationConfig struct {
	SmallMinMs  int `yaml:"small_min_ms"`  // 小任务最小执行时间（毫秒）
	SmallMaxMs  int `yaml:"small_max_ms"`  // 小任务最大执行时间（毫秒）
	MediumMinMs int `yaml:"medium_min_ms"` // 中任务最小执行时间（毫秒）
	MediumMaxMs int `yaml:"medium_max_ms"` // 中任务最大执行时间（毫秒）
	LargeMinMs  int `yaml:"large_min_ms"`  // 大任务最小执行时间（毫秒）
	LargeMaxMs  int `yaml:"large_max_ms"`  // 大任务最大执行时间（毫秒）
}

// ParseMemory 解析内存字符串为字节数
// 支持格式：8Gi, 8GB, 8192Mi, 8192MB, 8192, 8G, 8M 等
func (r *ResourceConfig) ParseMemory() (int64, error) {
	if r.Memory == "" {
		return 0, nil
	}

	// 使用与 docker provider 相同的解析逻辑
	return parseMemoryString(r.Memory)
}

// parseMemoryString 解析内存字符串（从 docker provider 复制）
func parseMemoryString(memoryStr string) (int64, error) {
	// 移除空格并转换为小写
	memoryStr = strings.TrimSpace(strings.ToLower(memoryStr))

	// 正则表达式匹配数字和单位
	re := regexp.MustCompile(`^(\d+)([kmgt]?i?b?)$`)
	matches := re.FindStringSubmatch(memoryStr)
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid memory format: %s, expected format like 8Gi, 8GB, 8192Mi", memoryStr)
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

// LoadConfig 加载配置文件
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

	// 如果任务执行时间配置未设置，使用默认值
	if config.TaskDuration.SmallMinMs == 0 && config.TaskDuration.SmallMaxMs == 0 {
		config.TaskDuration = getDefaultTaskDurationConfig()
	}

	return &config, nil
}

// getDefaultConfig 返回默认配置
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
		ResourceTags: []string{"cpu", "memory"},
		TaskDuration: getDefaultTaskDurationConfig(),
	}
}

// getDefaultTaskDurationConfig 返回默认的任务执行时间配置
func getDefaultTaskDurationConfig() TaskDurationConfig {
	return TaskDurationConfig{
		SmallMinMs:  50,
		SmallMaxMs:  200,
		MediumMinMs: 200,
		MediumMaxMs: 800,
		LargeMinMs:  800,
		LargeMaxMs:  2000,
	}
}
