package provider

import (
	"context"
	"fmt"

	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
)

// DockerManager 管理 Docker 引擎连接
type DockerManager struct {
	client *client.Client
	host   string
}

// NewDockerManager 创建新的 Docker 管理器
func NewDockerManager(host, tlsCertPath string, tlsVerify bool, apiVersion string) (*DockerManager, error) {
	var opts []client.Opt

	if host != "" {
		opts = append(opts, client.WithHost(host))
	} else {
		opts = append(opts, client.FromEnv)
	}

	// 配置 TLS（如果指定）
	if tlsCertPath != "" {
		opts = append(opts, client.WithTLSClientConfig(tlsCertPath, "cert.pem", "key.pem"))
	}

	// 配置 API 版本
	if apiVersion != "" {
		opts = append(opts, client.WithVersion(apiVersion))
	} else {
		opts = append(opts, client.WithAPIVersionNegotiation())
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// 测试连接
	ctx := context.Background()
	_, err = cli.Ping(ctx)
	if err != nil {
		cli.Close()
		return nil, fmt.Errorf("failed to ping Docker daemon: %w", err)
	}

	logrus.Infof("Successfully connected to Docker daemon at %s", host)

	return &DockerManager{
		client: cli,
		host:   host,
	}, nil
}

// GetClient 获取 Docker 客户端
func (dm *DockerManager) GetClient() *client.Client {
	return dm.client
}

// Close 关闭 Docker 客户端连接
func (dm *DockerManager) Close() error {
	if dm.client != nil {
		return dm.client.Close()
	}
	return nil
}
