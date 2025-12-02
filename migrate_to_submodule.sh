#!/bin/bash

# 迁移脚本：将 iarnet/proto 目录改为使用 git submodule
# 此脚本会：
# 1. 删除 proto 源文件目录（保留生成脚本）
# 2. 添加 iarnet-proto 作为 git submodule

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROTO_DIR="${PROJECT_ROOT}/proto"

echo "=========================================="
echo "迁移 iarnet/proto 到 git submodule"
echo "项目根目录: $PROJECT_ROOT"
echo "=========================================="

# 检查是否在 git 仓库中
if [ ! -d "$PROJECT_ROOT/.git" ]; then
    echo "错误: 当前目录不是 git 仓库"
    exit 1
fi

# 1. 删除 proto 源文件目录
echo ""
echo ">>> 步骤 1: 删除 proto 源文件目录..."
if [ -d "$PROTO_DIR/application" ]; then
    echo "  删除 application/"
    rm -rf "$PROTO_DIR/application"
fi
if [ -d "$PROTO_DIR/common" ]; then
    echo "  删除 common/"
    rm -rf "$PROTO_DIR/common"
fi
if [ -d "$PROTO_DIR/global" ]; then
    echo "  删除 global/"
    rm -rf "$PROTO_DIR/global"
fi
if [ -d "$PROTO_DIR/ignis" ]; then
    echo "  删除 ignis/"
    rm -rf "$PROTO_DIR/ignis"
fi
if [ -d "$PROTO_DIR/resource" ]; then
    echo "  删除 resource/"
    rm -rf "$PROTO_DIR/resource"
fi
echo "  ✓ Proto 源文件目录已删除"

# 2. 检查是否已存在 submodule
echo ""
echo ">>> 步骤 2: 检查 git submodule..."
if [ -f "$PROJECT_ROOT/.gitmodules" ] && grep -q "iarnet-proto" "$PROJECT_ROOT/.gitmodules" 2>/dev/null; then
    echo "  ⚠ iarnet-proto submodule 已存在"
    echo "  更新 submodule..."
    git submodule update --init --recursive third_party/iarnet-proto || true
else
    echo "  添加 iarnet-proto 作为 git submodule..."
    
    # 创建 third_party 目录
    mkdir -p "$PROJECT_ROOT/third_party"
    
    # 尝试添加 submodule
    IARNET_PROTO_REPO="git@github.com:9triver/iarnet-proto.git"
    
    if git submodule add "$IARNET_PROTO_REPO" third_party/iarnet-proto 2>&1; then
        echo "  ✓ Submodule 添加成功"
    else
        echo "  ⚠ Submodule 添加失败，可能已存在或需要手动添加"
        echo "  请手动执行:"
        echo "    git submodule add $IARNET_PROTO_REPO third_party/iarnet-proto"
    fi
fi

# 3. 验证 submodule
echo ""
echo ">>> 步骤 3: 验证 submodule..."
if [ -d "$PROJECT_ROOT/third_party/iarnet-proto/proto" ] && [ -d "$PROJECT_ROOT/third_party/iarnet-proto/scripts" ]; then
    echo "  ✓ iarnet-proto submodule 验证成功"
    echo "    - proto/ 目录存在"
    echo "    - scripts/ 目录存在"
else
    echo "  ⚠ iarnet-proto submodule 验证失败"
    echo "  请检查 third_party/iarnet-proto 目录"
fi

# 4. 测试生成脚本
echo ""
echo ">>> 步骤 4: 测试生成脚本..."
if [ -f "$PROTO_DIR/protobuf-gen.sh" ]; then
    echo "  生成脚本存在: $PROTO_DIR/protobuf-gen.sh"
    echo "  可以运行以下命令测试:"
    echo "    cd $PROJECT_ROOT"
    echo "    ./proto/protobuf-gen.sh"
else
    echo "  ⚠ 生成脚本不存在"
fi

echo ""
echo "=========================================="
echo "迁移完成！"
echo "=========================================="
echo ""
echo "下一步操作:"
echo "1. 检查更改: git status"
echo "2. 测试生成: ./proto/protobuf-gen.sh"
echo "3. 提交更改:"
echo "   git add .gitmodules third_party/iarnet-proto proto/"
echo "   git commit -m '重构: 将 proto 源文件迁移到独立的 iarnet-proto 仓库'"
echo ""

