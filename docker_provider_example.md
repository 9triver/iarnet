# Docker Provider 使用说明

## 概述

DockerProvider 是 iarnet 系统的一个资源提供者实现，它可以从 Docker 守护进程获取实时的资源使用情况和容量信息。

## 功能特性

- **实时资源监控**: 获取所有运行中 Docker 容器的 CPU、内存和 GPU 使用情况
- **容量检测**: 自动检测系统总容量和可用容量
- **GPU 支持**: 检测 NVIDIA GPU 资源（需要 nvidia-docker 运行时）
- **错误处理**: 优雅处理 Docker 连接失败和容器状态异常
- **工厂模式**: Manager 负责创建和管理 provider，调用者只需指定 provider 类型
- **命名空间式访问**: 使用 ProviderType.Docker 的方式访问，提供更好的代码组织和可读性
- **类型安全**: 避免字符串硬编码，提供编译时类型检查
- **自动生成 ID**: Manager 自动为 provider 生成唯一的递增 ID（如 docker-1, docker-2, ...）
- **本地 Provider 引用**: 每个 DockerProvider 实例都包含对本地 Docker provider 的引用，便于访问本地资源
- **灵活的连接配置**: 支持本地和远程 Docker 连接，包括 TLS 安全连接

## 前置条件

1. **Docker 守护进程**: 确保 Docker 守护进程正在运行
2. **Docker API 访问**: 应用需要能够访问 Docker API（通常通过 Unix socket 或 TCP）
3. **权限**: 运行应用的用户需要有访问 Docker 的权限
4. **远程连接**（可选）：如需连接远程 Docker，确保目标主机开启 Docker API 并配置 TLS 证书

## 配置参数

### DockerConfig 结构

```go
type DockerConfig struct {
    // Host 是 Docker daemon 的主机地址（例如："tcp://192.168.1.100:2376"）
    // 如果为空，则使用本地 Docker daemon
    Host string
    
    // TLSCertPath 是 TLS 证书目录的路径
    // 远程安全连接时必需
    TLSCertPath string
    
    // TLSVerify 启用 TLS 验证
    TLSVerify bool
    
    // APIVersion 指定要使用的 Docker API 版本
    // 如果为空，则使用版本协商
    APIVersion string
}
```

### 配置示例

**本地连接（默认）**：
```go
// 方式1：传递 nil，自动使用本地连接
rm.RegisterProvider(resource.ProviderType.Docker, nil)

// 方式2：传递空的 DockerConfig
dockerConfig := resource.DockerConfig{}
rm.RegisterProvider(resource.ProviderType.Docker, dockerConfig)
```

**远程连接（无 TLS）**：
```go
dockerConfig := resource.DockerConfig{
    Host: "tcp://192.168.1.100:2375",
}
```

**远程连接（带 TLS）**：
```go
dockerConfig := resource.DockerConfig{
    Host:        "tcp://192.168.1.100:2376",
    TLSCertPath: "/path/to/certs",
    TLSVerify:   true,
}
```

## 配置

### 环境变量

```bash
# Docker API 端点（可选，默认使用系统默认值）
export DOCKER_HOST=unix:///var/run/docker.sock

# Docker API 版本（可选）
export DOCKER_API_VERSION=1.41
```

### Windows 配置

在 Windows 上，确保 Docker Desktop 正在运行，并且启用了 "Expose daemon on tcp://localhost:2375 without TLS" 选项（仅用于开发环境）。

## 使用方式

### 自动集成

DockerProvider 会在应用启动时自动初始化和注册：

```go
// 在 main.go 中自动执行
dockerProvider, err := resource.NewDockerProvider()
if err != nil {
    logrus.Warnf("Failed to initialize Docker provider: %v", err)
    // 继续使用静态资源限制
} else {
    providerID := rm.RegisterProvider(dockerProvider)
    logrus.Infof("Docker provider registered successfully with ID: %s", providerID)
}
```

### 手动使用

```go
package main

import (
    "context"
    "log"
    "github.com/9triver/iarnet/internal/resource"
)

func main() {
    // 创建资源管理器
    rm := resource.NewManager(map[string]string{
        "cpu": "4",
        "memory": "8Gi",
        "gpu": "1",
    })
    
    // 注册 Docker Provider（通过类型）
	providerID, err := rm.RegisterProvider(resource.ProviderType.Docker)
	if err != nil {
		log.Printf("Failed to register Docker provider: %v", err)
	} else {
		log.Printf("Docker provider registered with ID: %s", providerID)
	}

    // 获取 provider 实例
     provider, err := rm.GetProvider(providerID)
     if err != nil {
         log.Fatal(err)
     }
 
     // 获取容量信息
     capacity, err := provider.GetCapacity(context.Background())
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Total CPU: %.2f cores", capacity.Total.CPU)
    log.Printf("Total Memory: %.2f GB", capacity.Total.Memory)
    log.Printf("Total GPU: %.0f units", capacity.Total.GPU)

    // 获取实时使用情况
    usage, err := provider.GetRealTimeUsage(context.Background())
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Used CPU: %.2f cores", usage.CPU)
    log.Printf("Used Memory: %.2f GB", usage.Memory)
    log.Printf("Used GPU: %.0f units", usage.GPU)

    // 启动监控
    rm.StartMonitoring()
}
```

## API 使用方法

### 本地 Docker 连接

```go
package main

import (
     "context"
     "log"
     
     "github.com/9triver/iarnet/internal/resource"
)

func main() {
     // 创建资源管理器
     rm := resource.NewManager(map[string]string{
         "cpu":    "4",
         "memory": "8Gi",
         "gpu":    "1",
     })
     
     // 注册本地 Docker provider（使用 nil 配置，自动使用本地连接）
     providerID, err := rm.RegisterProvider(resource.ProviderType.Docker, nil)
     if err != nil {
         log.Fatal(err)
     }
     
     log.Printf("Provider registered with ID: %s", providerID)
     
     // 获取 provider 实例
     provider, err := rm.GetProvider(providerID)
     if err != nil {
         log.Fatal(err)
     }
 
     // 获取容量信息
     capacity, err := provider.GetCapacity(context.Background())
     if err != nil {
         log.Fatal(err)
     }
     
     log.Printf("Total capacity: CPU=%.2f, Memory=%.2fGB, GPU=%.2f", 
         capacity.Total.CPU, capacity.Total.Memory, capacity.Total.GPU)
}
```

### 远程 Docker 连接

```go
package main

import (
     "context"
     "log"
     
     "github.com/9triver/iarnet/internal/resource"
)

func main() {
     // 创建资源管理器
     rm := resource.NewManager(map[string]string{
         "cpu":    "8",
         "memory": "16Gi",
         "gpu":    "2",
     })
     
     // 配置远程 Docker 连接
     dockerConfig := resource.DockerConfig{
         Host:        "tcp://192.168.1.100:2376",
         TLSCertPath: "/path/to/certs",
         TLSVerify:   true,
         APIVersion:  "1.41",
     }
     
     // 注册远程 Docker provider
     providerID, err := rm.RegisterProvider(resource.ProviderType.Docker, dockerConfig)
     if err != nil {
         log.Fatal(err)
     }
     
     log.Printf("Remote Docker provider registered with ID: %s", providerID)
     
     // 获取 provider 实例
     provider, err := rm.GetProvider(providerID)
     if err != nil {
         log.Fatal(err)
     }
 
     // 获取容量信息
     capacity, err := provider.GetCapacity(context.Background())
     if err != nil {
         log.Fatal(err)
     }
     
     log.Printf("Remote capacity: CPU=%.2f, Memory=%.2fGB, GPU=%.2f", 
         capacity.Total.CPU, capacity.Total.Memory, capacity.Total.GPU)
}
```

### 访问本地 Provider 引用

```go
package main

import (
     "context"
     "log"
     
     "github.com/9triver/iarnet/internal/resource"
)

func main() {
     rm := resource.NewManager(map[string]string{
         "cpu":    "4",
         "memory": "8Gi",
     })
     
     // 注册本地 Docker provider
     localProviderID, err := rm.RegisterProvider(resource.ProviderType.Docker, nil)
     if err != nil {
         log.Fatal(err)
     }
     
     // 注册远程 Docker provider
     remoteConfig := resource.DockerConfig{
         Host: "tcp://192.168.1.100:2376",
     }
     remoteProviderID, err := rm.RegisterProvider(resource.ProviderType.Docker, remoteConfig)
     if err != nil {
         log.Fatal(err)
     }
     
     // 获取远程 provider
     remoteProvider, err := rm.GetProvider(remoteProviderID)
     if err != nil {
         log.Fatal(err)
     }
     
     // 通过远程 provider 访问本地 provider 引用
     dockerProvider := remoteProvider.(*resource.DockerProvider)
     localProvider := dockerProvider.GetLocalProvider()
     
     if localProvider != nil {
         log.Printf("Local provider ID: %s", localProvider.GetProviderID())
         
         // 可以通过本地 provider 获取本地资源信息
         localCapacity, err := localProvider.GetCapacity(context.Background())
         if err == nil {
             log.Printf("Local capacity: CPU=%.2f, Memory=%.2fGB", 
                 localCapacity.Total.CPU, localCapacity.Total.Memory)
         }
     }
     
     // 也可以直接创建新的本地 provider
     newLocalProvider, err := resource.GetLocalDockerProvider()
     if err != nil {
         log.Printf("Failed to create local provider: %v", err)
     } else {
         log.Printf("New local provider ID: %s", newLocalProvider.GetProviderID())
     }
}
```

## API 端点

启用 DockerProvider 后，以下 API 端点将返回实时的 Docker 资源信息：

- `GET /api/resources/capacity` - 获取资源容量信息
- `GET /api/resources/usage` - 获取实时资源使用情况

## 故障排除

### 常见问题

1. **连接失败**
   ```
   Failed to initialize Docker provider: Cannot connect to the Docker daemon
   ```
   - 确保 Docker 守护进程正在运行
   - 检查 DOCKER_HOST 环境变量
   - 验证用户权限

2. **权限拒绝**
   ```
   permission denied while trying to connect to the Docker daemon socket
   ```
   - 将用户添加到 docker 组：`sudo usermod -aG docker $USER`
   - 重新登录或重启会话

3. **GPU 检测失败**
   - 确保安装了 nvidia-docker 运行时
   - 检查 NVIDIA 驱动程序是否正确安装
   - 验证 Docker 配置中是否启用了 nvidia 运行时

### 调试模式

启用详细日志记录：

```bash
export LOG_LEVEL=debug
./iarnet -config config.yaml
```

## 性能考虑

- DockerProvider 会缓存容器信息以减少 API 调用
- 大量容器可能会影响性能，建议监控 API 响应时间
- GPU 检测是轻量级的，不会显著影响性能

## 安全注意事项

- 确保 Docker API 访问受到适当保护
- 在生产环境中避免暴露未加密的 Docker API
- 定期更新 Docker 和相关依赖项