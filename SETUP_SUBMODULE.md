# 设置 iarnet-proto Git Submodule

## 步骤

### 1. 添加 submodule

在 `iarnet` 仓库根目录执行：

```bash
cd /path/to/iarnet
git submodule add git@github.com:9triver/iarnet-proto.git third_party/iarnet-proto
```

### 2. 初始化 submodule

```bash
git submodule update --init --recursive
```

### 3. 验证

检查 submodule 是否正确添加：

```bash
ls -la third_party/iarnet-proto
# 应该能看到 proto/ 和 scripts/ 目录

# 检查 .gitmodules 文件
cat .gitmodules
```

### 4. 测试生成脚本

```bash
./proto/protobuf-gen.sh
```

## 提交更改

```bash
git add .gitmodules third_party/iarnet-proto proto/protobuf-gen.sh proto/README.md
git commit -m "重构: 将 proto 源文件迁移到独立的 iarnet-proto 仓库"
```

## 克隆包含 submodule 的仓库

如果其他人克隆了包含 submodule 的仓库，需要执行：

```bash
git clone <iarnet-repo-url>
cd iarnet
git submodule update --init --recursive
```

或者使用递归克隆：

```bash
git clone --recursive <iarnet-repo-url>
```

## 更新 submodule

```bash
# 更新到最新版本
cd third_party/iarnet-proto
git pull origin main
cd ../..
git add third_party/iarnet-proto
git commit -m "更新 iarnet-proto 到最新版本"

# 切换到指定版本
cd third_party/iarnet-proto
git checkout v1.0.0
cd ../..
git add third_party/iarnet-proto
git commit -m "锁定 iarnet-proto 版本为 v1.0.0"
```

