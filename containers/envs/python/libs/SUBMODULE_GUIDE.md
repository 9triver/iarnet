# Git Submodule 使用指南

## 概述

本项目使用 git submodule 来管理 `lucas` 库，该库来自独立的 GitHub 仓库 `git@github.com:Xdydy/Lucas.git`。

## 当前配置

- **Submodule 路径**: `containers/envs/python/libs/lucas`
- **远程仓库**: `git@github.com:Xdydy/Lucas.git`
- **配置文件**: `.gitmodules`

## 常用操作

### 1. 克隆包含 submodule 的项目

```bash
# 方法一：克隆时同时初始化 submodule
git clone --recursive git@github.com:9triver/iarnet.git

# 方法二：先克隆主项目，再初始化 submodule
git clone git@github.com:9triver/iarnet.git
cd iarnet
git submodule init
git submodule update
```

### 2. 更新 submodule 到最新版本

```bash
# 进入 submodule 目录
cd containers/envs/python/libs/lucas

# 拉取最新更改
git pull origin main

# 回到主项目目录
cd ../../../../..

# 提交 submodule 版本更新
git add containers/envs/python/libs/lucas
git commit -m "Update lucas submodule to latest version"
```

### 3. 切换 submodule 到特定版本

```bash
# 进入 submodule 目录
cd containers/envs/python/libs/lucas

# 切换到特定的 commit 或 tag
git checkout <commit-hash-or-tag>

# 回到主项目目录
cd ../../../../..

# 提交版本变更
git add containers/envs/python/libs/lucas
git commit -m "Pin lucas submodule to version <version>"
```

### 4. 在 submodule 中进行开发

```bash
# 进入 submodule 目录
cd containers/envs/python/libs/lucas

# 创建新分支进行开发
git checkout -b feature/new-feature

# 进行开发和提交
git add .
git commit -m "Add new feature"

# 推送到远程仓库
git push origin feature/new-feature

# 在 GitHub 上创建 Pull Request
```

### 5. 检查 submodule 状态

```bash
# 查看所有 submodule 状态
git submodule status

# 查看 submodule 的详细信息
git submodule foreach git status
```

## 注意事项

1. **版本控制独立性**: lucas 库有自己的版本控制历史，与主项目独立
2. **依赖管理**: 确保在构建脚本中正确处理 submodule 的初始化
3. **团队协作**: 团队成员需要了解 submodule 的工作方式
4. **CI/CD**: 构建流水线需要包含 submodule 初始化步骤

## 构建脚本更新

在使用 Docker 构建时，需要确保 submodule 已正确初始化：

```dockerfile
# 在 Dockerfile 中添加
RUN git submodule init && git submodule update
```

## 故障排除

### 问题：submodule 目录为空
```bash
git submodule init
git submodule update
```

### 问题：submodule 指向错误的 commit
```bash
cd containers/envs/python/libs/lucas
git checkout main
git pull origin main
cd ../../../../..
git add containers/envs/python/libs/lucas
git commit -m "Update submodule to latest"
```

### 问题：删除 submodule
```bash
# 1. 删除 .gitmodules 中的相关条目
# 2. 删除 .git/config 中的相关条目
# 3. 运行以下命令
git rm --cached containers/envs/python/libs/lucas
rm -rf containers/envs/python/libs/lucas
git commit -m "Remove lucas submodule"
```

## 备份信息

原始的 lucas 目录已备份为 `lucas_backup_20251023_163325`，可在需要时参考。