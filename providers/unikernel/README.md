# Unikernel Provider

Unikernel provider 用于在 iarnet 平台上部署和运行 Unikernel 函数。

## 前置要求

### 1. 安装 Solo5

Solo5 是运行 Unikernel 的底层运行时环境。需要安装 `solo5-hvt` 可执行文件。

#### 方法一：从源码编译安装（推荐）

```bash
# 安装依赖项
# Ubuntu/Debian:
sudo apt-get update
sudo apt-get install -y build-essential gcc make pkg-config libc6-dev libseccomp-dev

# CentOS/RHEL:
sudo yum install -y gcc make pkgconfig glibc-devel libseccomp-devel

# 克隆 Solo5 仓库
git clone https://github.com/Solo5/solo5.git
cd solo5

# 配置构建环境（必须先运行此步骤）
./configure.sh

# 构建和安装
make
sudo make install

# 验证安装
solo5-hvt --version
```

**注意**：必须先运行 `./configure.sh` 进行配置，然后才能执行 `make`。如果直接运行 `make` 会提示 `Makeconf not found` 错误。

#### 方法二：使用包管理器（如果可用）

某些 Linux 发行版可能提供预编译的 solo5 包，可以通过包管理器安装：

```bash
# 检查是否有可用的包
apt search solo5  # Debian/Ubuntu
yum search solo5  # CentOS/RHEL
```

### 2. 安装 OCaml 和 MirageOS 工具链

**重要**：需要 OCaml 5.1 或更高版本（因为代码使用了 `Map.of_list` 函数，这是 OCaml 5.1+ 的功能）。

**注意**：`ocaml-solo5` 包对 OCaml 版本有特定要求：
- OCaml 5.2.0: 不兼容（没有对应的 ocaml-solo5 版本）
- OCaml 5.2.1: 需要 `ocaml-solo5.1.0.1`
- OCaml 5.3.0: 需要 `ocaml-solo5.1.1.0` 或更高版本
- OCaml 5.4.0: 需要 `ocaml-solo5.1.2.0` 或更高版本

#### 检查当前 OCaml 版本

```bash
ocaml -version
```

如果版本低于 5.1，或者版本是 5.2.0（不兼容），需要升级：

#### 升级 OCaml 到兼容版本

**推荐使用 OCaml 5.2.1 或 5.3.0**：

```bash
# 选项 1: 使用 OCaml 5.2.1（推荐，稳定版本）
opam switch create 5.2.1

# 选项 2: 使用 OCaml 5.3.0（最新稳定版本）
opam switch create 5.3.0

# 激活新的 switch
eval $(opam env)

# 验证版本
ocaml -version  # 应该显示 5.2.1 或 5.3.0

# 安装 MirageOS 和相关依赖
opam install mirage

# 安装 ocaml-solo5（会自动安装兼容的版本）
opam install ocaml-solo5
```

**如果还没有安装 OPAM**：

```bash
# 安装 OPAM (OCaml 包管理器)
# Ubuntu/Debian:
sudo apt-get install -y opam

# CentOS/RHEL:
sudo yum install -y opam

# 初始化 OPAM
opam init
eval $(opam env)
```

#### 清理不兼容的依赖

如果之前在低版本 OCaml（如 4.14.1 的 default switch）中安装了依赖，需要清理：

**清理 OCaml 4.14.1 (default switch) 中的依赖**：

**方法 1：清理 default switch 中的所有包（推荐）**

```bash
# 1. 切换到 default switch
opam switch default
eval $(opam env)

# 2. 查看已安装的包
opam list --installed

# 3. 卸载所有已安装的包（保留编译器和基础包）
# 方法 A: 使用 --update-invariant 选项（推荐）
opam remove --update-invariant $(opam list --installed --short | grep -v "^ocaml$" | grep -v "^ocaml-base-compiler$" | grep -v "^base-")

# 方法 B: 逐个卸载主要包（更安全）
opam remove mirage ocaml-solo5 solo5 mirage-solo5 mirage-runtime
# 然后卸载其他依赖
opam remove $(opam list --installed --short | grep -v "^ocaml$" | grep -v "^ocaml-base-compiler$" | grep -v "^base-" | grep -v "^dune$" | grep -v "^ocamlfind$")

# 或者更彻底：删除整个 default switch 并重新创建
opam switch remove default
opam switch create default ocaml-base-compiler.4.14.1
eval $(opam env)
```

**方法 2：删除整个 default switch（如果不需要保留 4.14.1）**

```bash
# 1. 切换到其他 switch（如 5.2.1 或 5.3.0）
opam switch 5.2.1  # 或 5.3.0
eval $(opam env)

# 2. 删除 default switch（会移除所有已安装的包）
opam switch remove default

# 3. 如果需要，可以重新创建一个干净的 default switch
opam switch create default ocaml-base-compiler.4.14.1
```

**清理 OCaml 5.2.0 switch 中的依赖**（如果存在）：

**方法 2：保留旧 switch，创建新 switch**

```bash
# 1. 直接创建新的兼容 switch（不删除旧的）
opam switch create 5.2.1
# 或者
opam switch create 5.3.0

# 2. 切换到新 switch
opam switch 5.2.1  # 或 5.3.0
eval $(opam env)

# 3. 在新 switch 中安装依赖
opam install mirage ocaml-solo5
```

**方法 3：在当前 switch 中卸载所有包（不推荐，可能有问题）**

```bash
# 获取所有已安装的包列表
opam list --installed --short > installed_packages.txt

# 卸载所有包（保留编译器）
opam remove $(opam list --installed --short | grep -v "^ocaml$")

# 然后重新安装需要的包
opam install mirage ocaml-solo5
```

#### 升级 ocaml-solo5 的完整步骤

如果您当前在 OCaml 5.2.0 环境中（不兼容），需要先升级 OCaml：

```bash
# 1. 创建新的 OCaml switch（推荐 5.2.1 或 5.3.0）
opam switch create 5.2.1
# 或者
opam switch create 5.3.0

# 2. 激活新的 switch
eval $(opam env)

# 3. 验证 OCaml 版本
ocaml -version  # 应该显示 5.2.1 或 5.3.0

# 4. 安装 MirageOS（这会自动安装一些基础依赖）
opam install mirage

# 5. 安装 ocaml-solo5（OPAM 会自动选择兼容的版本）
opam install ocaml-solo5

# 6. 验证安装
opam list | grep ocaml-solo5
```

**如果您已经在兼容的 OCaml 版本（5.2.1 或 5.3.0）中**，只需要：

```bash
# 直接安装或升级 ocaml-solo5
opam install ocaml-solo5

# 或者升级到最新版本
opam upgrade ocaml-solo5
```

**版本兼容性参考**：
- OCaml 5.2.1 → ocaml-solo5.1.0.1
- OCaml 5.3.0 → ocaml-solo5.1.1.0 或 1.2.0
- OCaml 5.4.0 → ocaml-solo5.1.2.0

### 3. 初始化网络接口

在首次使用前，需要创建网络接口（需要 root 权限）：

```bash
cd providers/unikernel
sudo bash mirage-websocket/create_network.sh
```

或者手动创建：

```bash
sudo ip tuntap add tap100 mode tap
sudo ip addr add 10.0.0.1/24 dev tap100
sudo ip link set dev tap100 up
```

## 配置

编辑 `config.yaml` 文件：

```yaml
server:
  port: 50051  # gRPC 服务器端口

resource:
  cpu: 8000      # CPU 容量（millicores）
  memory: "16Gi" # 内存容量
  gpu: 0         # GPU 数量

resource_tags:
  - cpu
  - memory

supported_languages:
  - unikernel

websocket:
  port: 8080  # WebSocket 服务器端口

unikernel:
  base_dir: ""       # unikernel 代码基础目录，为空则自动检测
  solo5_hvt_path: "" # solo5-hvt 可执行文件路径，为空则从 PATH 查找

dns:
  hosts:
    "host.internal": "localhost"
```

### 配置说明

- `solo5_hvt_path`: 如果 `solo5-hvt` 不在系统 PATH 中，请指定完整路径，例如：`/usr/local/bin/solo5-hvt`
- `base_dir`: 通常不需要手动配置，系统会自动检测 `mirage-websocket` 目录位置
- `websocket.port`: Unikernel 进程会连接到这个端口的 WebSocket 服务器

## 运行

```bash
# 从项目根目录运行
cd providers/unikernel
go run cmd/main.go -config config.yaml

# 或编译后运行
go build -o unikernel-provider cmd/main.go
./unikernel-provider -config config.yaml
```

## 工作原理

1. **函数部署**：当收到 FUNCTION 消息时，provider 会：
   - 从消息中提取 OCaml 代码
   - 将代码写入 `handlers.ml` 文件
   - 使用 MirageOS 构建工具编译为 `.hvt` 文件
   - 使用 `solo5-hvt` 运行编译后的 unikernel

2. **函数调用**：当收到 INVOKE_REQUEST 消息时：
   - 从 iarnet store 获取参数对象（支持 Python pickle 自动转换为 JSON）
   - 通过 WebSocket 发送调用请求到 unikernel
   - 等待 unikernel 响应
   - 将响应保存到 store 并返回

3. **消息格式**：
   - iarnet → unikernel: 通过 WebSocket 发送 JSON 格式消息
   - unikernel → iarnet: 通过 WebSocket 接收 JSON 格式响应

## 故障排查

### solo5-hvt 未找到

如果遇到 `exec: "solo5-hvt": executable file not found in $PATH` 错误：

1. 确认 solo5-hvt 已安装：
   ```bash
   which solo5-hvt
   solo5-hvt --version
   ```

2. 如果已安装但不在 PATH 中，在 `config.yaml` 中指定完整路径：
   ```yaml
   unikernel:
     solo5_hvt_path: "/usr/local/bin/solo5-hvt"
   ```

### 网络接口创建失败

如果 `create_network.sh` 失败：

1. 确认有 root 权限
2. 检查 tap100 设备是否已存在：
   ```bash
   ip link show tap100
   ```
3. 如果已存在，先删除再创建：
   ```bash
   sudo ip link del dev tap100
   sudo bash mirage-websocket/create_network.sh
   ```

### 构建失败

如果 unikernel 构建失败：

1. 确认 OCaml 和 MirageOS 工具链已正确安装
2. 检查 `mirage-websocket` 目录是否存在且完整
3. 查看构建日志中的具体错误信息

### configure.sh 失败

如果运行 `./configure.sh` 时提示缺少依赖：

1. **缺少 libseccomp**：
   ```bash
   # Ubuntu/Debian:
   sudo apt-get install -y libseccomp-dev
   
   # CentOS/RHEL:
   sudo yum install -y libseccomp-devel
   ```

2. **其他缺失的依赖**：根据错误信息安装相应的开发库，常见的有：
   - `libseccomp-dev` / `libseccomp-devel` - seccomp 库
   - `libssl-dev` / `openssl-devel` - OpenSSL 库（如果需要）
   - `zlib1g-dev` / `zlib-devel` - zlib 库（如果需要）

## 开发

### 目录结构

```
providers/unikernel/
├── cmd/
│   └── main.go          # 主程序入口
├── config/
│   └── config.go        # 配置加载
├── provider/
│   ├── service.go       # Provider 服务实现
│   └── manager.go       # 健康检查管理器
├── mirage-websocket/    # MirageOS unikernel 代码
│   ├── unikernel.ml
│   ├── handlers.ml
│   ├── build.sh
│   └── create_network.sh
└── config.yaml          # 配置文件
```

### 测试

确保所有依赖已安装后，可以运行 provider 并部署测试函数。
