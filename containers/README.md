# IARNet 容器目录结构

本目录包含 IARNet 项目的所有容器化组件，采用了重新设计的目录结构以提高可维护性和扩展性。

## 目录结构

```
containers/
├── shared/                    # 共享的语言运行时和依赖
│   ├── python/               # Python 语言环境
│   │   ├── requirements.txt  # 通用 Python 依赖
│   │   ├── libs/            # Python 库
│   │   │   ├── actorc/      # Actor 计算库
│   │   │   └── lucas/       # Lucas 分布式计算库
│   │   └── runtime/         # Python 运行时工具
│   │       ├── executor.py  # Python 执行器
│   │       └── initializer.go # Go 初始化器
│   ├── java/                # Java 语言环境（预留）
│   │   └── libs/
│   └── nodejs/              # Node.js 语言环境（预留）
│       └── libs/
├── images/                   # 容器镜像定义
│   ├── base/                # 基础镜像
│   │   ├── python.Dockerfile   # Python 基础镜像
│   │   ├── java.Dockerfile     # Java 基础镜像（预留）
│   │   └── nodejs.Dockerfile   # Node.js 基础镜像（预留）
│   ├── component/           # 组件镜像
│   │   ├── Dockerfile       # Component 镜像定义
│   │   ├── build.sh         # 构建脚本
│   │   ├── go.mod           # Go 模块定义
│   │   ├── go.sum           # Go 依赖校验
│   │   ├── main.go          # 主程序
│   │   ├── runtime/         # Go 运行时代码
│   │   └── stub/            # 存根代码
│   └── runner/              # 运行器镜像
│       ├── Dockerfile       # Runner 镜像定义
│       ├── build.sh         # 构建脚本
│       ├── go.mod           # Go 模块定义
│       ├── go.sum           # Go 依赖校验
│       └── main.go          # 主程序
└── scripts/                 # 构建和部署脚本
    ├── build-all.sh         # 构建所有镜像
    ├── build-component.sh   # 构建 Component 镜像
    └── build-runner.sh      # 构建 Runner 镜像
```

## 重组的优势

1. **依赖共享**：所有语言的依赖库统一管理在 `shared/` 目录下，避免重复
2. **语言隔离**：每种语言有独立的目录，便于管理不同语言的依赖
3. **镜像分离**：容器镜像定义与语言运行时分离，结构更清晰
4. **扩展性好**：添加新语言支持时只需在 `shared/` 下添加对应目录
5. **构建优化**：可以创建语言特定的基础镜像，减少重复构建

## 使用方法

### 构建所有镜像
```bash
./scripts/build-all.sh
```

### 单独构建镜像
```bash
# 构建 Component 镜像
./scripts/build-component.sh

# 构建 Runner 镜像
./scripts/build-runner.sh
```

## 添加新语言支持

要添加新的语言支持（如 Java），请按以下步骤操作：

1. 在 `shared/` 下创建语言目录：
   ```bash
   mkdir -p shared/java/{libs,runtime}
   ```

2. 添加语言特定的依赖库到 `shared/java/libs/`

3. 创建或更新基础镜像 `images/base/java.Dockerfile`

4. 在 Component 和 Runner 的 Dockerfile 中添加对新语言的支持

5. 更新构建脚本以包含新的基础镜像构建

## 注意事项

- 所有 Python 库（actorc, lucas）现在位于 `shared/python/libs/` 目录
- Python 运行时文件位于 `shared/python/runtime/` 目录
- 构建脚本具有执行权限，可以直接运行
- 基础镜像包含了常用的语言依赖，可以被多个应用镜像复用