#!/bin/bash

# iarnet 仓库的 Protobuf 生成脚本
# 此脚本调用 iarnet-proto 仓库的生成脚本，并输出到 iarnet 仓库的指定目录
# 
# 使用方法:
#   1. 确保 iarnet-proto 已通过 git submodule 或直接克隆到 third_party/iarnet-proto
#   2. 运行: ./proto/protobuf-gen.sh

set -e  # Exit on error

PROTOC="python -m grpc_tools.protoc"
export PATH="$PATH:$HOME/go/bin"

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="${PROJECT_ROOT}/proto"

# iarnet-proto 目录（优先使用 submodule）
IARNET_PROTO_DIR="${IARNET_PROTO_DIR:-${PROJECT_ROOT}/third_party/iarnet-proto}"
if [ ! -d "$IARNET_PROTO_DIR" ]; then
    # 回退到同级目录（用于开发环境）
    IARNET_PROTO_DIR="${PROJECT_ROOT}/../iarnet-proto"
fi

if [ ! -d "$IARNET_PROTO_DIR" ]; then
    cat >&2 <<EOF
错误: 未找到 iarnet-proto 目录
请确保:
  1. 已通过 git submodule 添加 iarnet-proto 到 third_party/iarnet-proto，或
  2. 已直接克隆 iarnet-proto 到 ../iarnet-proto，或
  3. 通过环境变量 IARNET_PROTO_DIR 指定路径

当前查找路径:
  - ${PROJECT_ROOT}/../iarnet-proto
  - ${PROJECT_ROOT}/third_party/iarnet-proto
EOF
    exit 1
fi

IARNET_PROTO_PROTO_DIR="${IARNET_PROTO_DIR}/proto"
IARNET_PROTO_SCRIPTS_DIR="${IARNET_PROTO_DIR}/scripts"

if [ ! -d "$IARNET_PROTO_PROTO_DIR" ] || [ ! -f "$IARNET_PROTO_SCRIPTS_DIR/gen_go.sh" ]; then
    cat >&2 <<EOF
错误: iarnet-proto 目录结构不正确
期望的目录结构:
  $IARNET_PROTO_DIR/
    ├── proto/
    └── scripts/
        ├── gen_go.sh
        └── gen_python.sh
EOF
    exit 1
fi

echo "=========================================="
echo "iarnet 仓库 Protobuf 生成"
echo "项目根目录: $PROJECT_ROOT"
echo "iarnet-proto 目录: $IARNET_PROTO_DIR"
echo "=========================================="

# ============================================================================
# 1. 生成 Go 代码
# ============================================================================
echo ""
echo ">>> 生成 Go 代码..."

GO_OUTPUT="${PROJECT_ROOT}/internal/proto"
bash "$IARNET_PROTO_SCRIPTS_DIR/gen_go.sh" "$IARNET_PROTO_PROTO_DIR" "$GO_OUTPUT"

# Runner common Go generation (特殊输出目录)
RUNNER_COMMON_OUTPUT="${PROJECT_ROOT}/containers/images/runner/proto"
if [ -d "$IARNET_PROTO_PROTO_DIR/common" ]; then
    echo ""
    echo ">>> 生成 Runner Common Go 代码: $RUNNER_COMMON_OUTPUT"
    mkdir -p "$RUNNER_COMMON_OUTPUT"
    find "$RUNNER_COMMON_OUTPUT" -type f -name "*.pb.go" -delete 2>/dev/null || true
    
    pushd "$IARNET_PROTO_PROTO_DIR/common" >/dev/null
    $PROTOC -I "$IARNET_PROTO_PROTO_DIR" \
        --go_out="$RUNNER_COMMON_OUTPUT" --go_opt=paths=source_relative \
        --go-grpc_out="$RUNNER_COMMON_OUTPUT" --go-grpc_opt=paths=source_relative \
        *.proto
    popd >/dev/null
fi

# Runner application logger Go generation
RUNNER_LOGGER_OUTPUT="${PROJECT_ROOT}/containers/images/runner/proto/logger"
if [ -d "$IARNET_PROTO_PROTO_DIR/application/logger" ]; then
    echo ""
    echo ">>> 生成 Runner Logger Go 代码: $RUNNER_LOGGER_OUTPUT"
    mkdir -p "$RUNNER_LOGGER_OUTPUT"
    find "$RUNNER_LOGGER_OUTPUT" -type f -name "*.pb.go" -delete 2>/dev/null || true
    
    pushd "$IARNET_PROTO_PROTO_DIR/application/logger" >/dev/null
    $PROTOC -I "$IARNET_PROTO_PROTO_DIR" -I . \
        --go_out="$RUNNER_LOGGER_OUTPUT" --go_opt=paths=source_relative \
        --go-grpc_out="$RUNNER_LOGGER_OUTPUT" --go-grpc_opt=paths=source_relative \
        *.proto
    popd >/dev/null
    
    # 修复 import 路径（如果需要）
    find "$RUNNER_LOGGER_OUTPUT" -type f -name "*.pb.go" -exec sed -i 's|github.com/9triver/iarnet/internal/proto/common|github.com/9triver/iarnet/runner/proto/common|g' {} + 2>/dev/null || true
fi

# ============================================================================
# 2. 生成 Python 代码
# ============================================================================
echo ""
echo ">>> 生成 Python 代码..."

# Component Python 输出
PY_OUTPUT_COMPONENT="${PROJECT_ROOT}/containers/component/python/proto"
bash "$IARNET_PROTO_SCRIPTS_DIR/gen_python.sh" "$IARNET_PROTO_PROTO_DIR" "$PY_OUTPUT_COMPONENT"

# Lucas Python 输出（特殊处理，优先使用 third_party，其次兼容旧路径）
LUCAS_BASE_DIR="${PROJECT_ROOT}/third_party/lucas"
if [ ! -d "$LUCAS_BASE_DIR" ]; then
    LUCAS_BASE_DIR="${PROJECT_ROOT}/containers/envs/python/libs/lucas"
fi
PY_OUTPUT_LUCAS="${LUCAS_BASE_DIR}/lucas/actorc/protos"

# Common
if [ -d "$IARNET_PROTO_PROTO_DIR/common" ]; then
    echo ""
    echo ">>> 生成 Lucas Common Python 代码: $PY_OUTPUT_LUCAS/common"
    mkdir -p "$PY_OUTPUT_LUCAS/common"
    find "$PY_OUTPUT_LUCAS/common" -type f -name "*_pb2.py" -delete 2>/dev/null || true
    find "$PY_OUTPUT_LUCAS/common" -type f -name "*_pb2.pyi" -delete 2>/dev/null || true
    find "$PY_OUTPUT_LUCAS/common" -type f -name "*_pb2_grpc.py" -delete 2>/dev/null || true
    
    pushd "$IARNET_PROTO_PROTO_DIR/common" >/dev/null
    $PROTOC -I "$IARNET_PROTO_PROTO_DIR" \
        --python_out="$PY_OUTPUT_LUCAS/common" \
        --pyi_out="$PY_OUTPUT_LUCAS/common" \
        --grpc_python_out="$PY_OUTPUT_LUCAS/common" \
        *.proto
    popd >/dev/null
    touch "$PY_OUTPUT_LUCAS/common/__init__.py"
fi

# Controller
if [ -d "$IARNET_PROTO_PROTO_DIR/ignis/controller" ]; then
    echo ""
    echo ">>> 生成 Lucas Controller Python 代码: $PY_OUTPUT_LUCAS/controller"
    mkdir -p "$PY_OUTPUT_LUCAS/controller"
    find "$PY_OUTPUT_LUCAS/controller" -type f -name "*_pb2.py" -delete 2>/dev/null || true
    find "$PY_OUTPUT_LUCAS/controller" -type f -name "*_pb2.pyi" -delete 2>/dev/null || true
    find "$PY_OUTPUT_LUCAS/controller" -type f -name "*_pb2_grpc.py" -delete 2>/dev/null || true
    
    pushd "$IARNET_PROTO_PROTO_DIR/ignis/controller" >/dev/null
    $PROTOC -I "$IARNET_PROTO_PROTO_DIR" -I . \
        --python_out="$PY_OUTPUT_LUCAS/controller" \
        --pyi_out="$PY_OUTPUT_LUCAS/controller" \
        --grpc_python_out="$PY_OUTPUT_LUCAS/controller" \
        *.proto
    popd >/dev/null
    touch "$PY_OUTPUT_LUCAS/controller/__init__.py"
fi

# Logger
if [ -d "$IARNET_PROTO_PROTO_DIR/application/logger" ]; then
    echo ""
    echo ">>> 生成 Lucas Logger Python 代码: $PY_OUTPUT_LUCAS/logger"
    mkdir -p "$PY_OUTPUT_LUCAS/logger"
    find "$PY_OUTPUT_LUCAS/logger" -type f -name "*_pb2.py" -delete 2>/dev/null || true
    find "$PY_OUTPUT_LUCAS/logger" -type f -name "*_pb2.pyi" -delete 2>/dev/null || true
    find "$PY_OUTPUT_LUCAS/logger" -type f -name "*_pb2_grpc.py" -delete 2>/dev/null || true
    
    pushd "$IARNET_PROTO_PROTO_DIR/application/logger" >/dev/null
    $PROTOC -I "$IARNET_PROTO_PROTO_DIR" -I . \
        --python_out="$PY_OUTPUT_LUCAS/logger" \
        --pyi_out="$PY_OUTPUT_LUCAS/logger" \
        --grpc_python_out="$PY_OUTPUT_LUCAS/logger" \
        *.proto
    popd >/dev/null
    touch "$PY_OUTPUT_LUCAS/logger/__init__.py"
fi

echo ""
echo "=========================================="
echo "Protobuf 生成完成！"
echo "=========================================="
