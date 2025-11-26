# IARNet

## 1 构建镜像

### 1.1 Component 镜像

#### 背景说明

Component 镜像是 IARNet 分布式计算系统中的核心执行单元，用于执行用户定义的函数任务。每个 Component 容器运行一个 Actor，负责：

- **接收任务**：通过 ZeroMQ 接收来自控制器的函数定义和调用请求
- **执行函数**：在隔离的容器环境中执行用户定义的 Python 函数
- **数据交互**：通过 gRPC 与 Store 服务通信，获取函数参数和保存执行结果
- **消息通信**：采用消息驱动的异步处理模式，支持高并发任务执行

Component 镜像在 Provider 管理的 dind 环境中运行，为分布式计算提供隔离的执行环境。

#### 代码目录

- **Python 实现**：`containers/component/python/` - Actor、消息收发、任务执行逻辑
- **构建脚本**：`containers/component/python/build.sh`

#### 构建方式

```sh
cd containers/scripts && ./build-component.sh
```

### Runner 镜像

#### 背景说明

Runner 镜像是应用程序的执行环境，用于运行用户提交的应用程序代码。Runner 容器提供：

- **应用执行环境**：包含特定编程语言的运行时环境（如 Python 3.11）
- **依赖管理**：自动安装应用程序所需的依赖包
- **代码执行**：在隔离环境中执行用户应用程序代码
- **日志收集**：将应用程序日志发送到日志服务

Runner 镜像在 iarnet 节点内部运行，为应用程序提供隔离的执行环境，支持多租户和资源隔离。

#### 代码目录

- **Go 主程序**：`containers/images/runner/` - Runner 主程序，负责环境初始化、依赖安装和代码执行
- **构建脚本**：`containers/images/runner/build.sh`

#### 构建方式

```sh
cd containers/images/runner && ./build.sh
```

## 启动后端

```sh
go run cmd/main.go --config=config.yaml
```

## 启动前端

```sh
cd web
npm install
npm run dev
```

## 保留仓库用于调试

创建应用时，如果将 Git 仓库 URL 填写为 `test.test`，系统不会执行 `git clone`。取而代之的是，会把项目根目录下 `testrepo/` 中的示例代码直接复制到对应应用的 workspace，方便快速体验应用生命周期。可以修改 `testrepo/` 内的文件以定制这份模板。
