#!/bin/bash

# iarnet 仓库的 Protobuf 生成脚本（按模块划分）
# 此脚本调用 iarnet-proto 仓库的生成脚本，并输出到 iarnet 仓库的指定目录
# 
# 使用方法:
#   1. 确保 iarnet-proto 已通过 git submodule 或直接克隆到 third_party/iarnet-proto
#   2. 运行: ./proto/protobuf-gen.sh [模块名]
#   3. 如果不指定模块名，则生成所有模块
#
# 模块列表:
#   - internal: 生成 Go 代码到 internal/proto/
#   - runner: 生成 Go 代码到 containers/runner/core/proto/
#   - component: 生成 Python 代码到 containers/component/python/proto/
#   - lucas: 生成 Python 代码到 third_party/lucas/lucas/actorc/protos/
#   - ignis: 生成 Go 代码到 third_party/ignis/ignis-go/proto/

set -e  # Exit on error

PROTOC="python3 -m grpc_tools.protoc"
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

if [ ! -d "$IARNET_PROTO_PROTO_DIR" ]; then
    cat >&2 <<EOF
错误: iarnet-proto 目录结构不正确
期望的目录结构:
  $IARNET_PROTO_DIR/
    └── proto/
EOF
    exit 1
fi

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=========================================="
echo "iarnet 仓库 Protobuf 生成"
echo "项目根目录: $PROJECT_ROOT"
echo "iarnet-proto 目录: $IARNET_PROTO_DIR"
echo "=========================================="

# ============================================================================
# 模块 1: internal - 生成 Go 代码到 internal/proto/
# ============================================================================
generate_internal() {
    echo ""
    echo -e "${YELLOW}>>> 模块: internal${NC}"
    echo -e "${YELLOW}输出目录: ${PROJECT_ROOT}/internal/proto${NC}"
    
    # 检查 protoc-gen-go 工具
    if ! command -v protoc-gen-go >/dev/null 2>&1 || ! command -v protoc-gen-go-grpc >/dev/null 2>&1; then
        cat >&2 <<EOF
错误: protoc-gen-go 和/或 protoc-gen-go-grpc 未在 PATH 中找到
安装方法:
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
EOF
        exit 1
    fi
    
    INTERNAL_OUTPUT="${PROJECT_ROOT}/internal/proto"
    
    # 清理已有的 proto 代码
    if [ -d "$INTERNAL_OUTPUT" ]; then
        echo -e "${YELLOW}  清理已有的 proto 代码...${NC}"
        find "$INTERNAL_OUTPUT" -type f -name "*.pb.go" -delete 2>/dev/null || true
    fi
    mkdir -p "$INTERNAL_OUTPUT"
    
    # 生成函数：为指定目录生成 Go 代码
    generate_go_for_dir() {
        local rel_dir="$1"
        
        local proto_dir="$IARNET_PROTO_PROTO_DIR/$rel_dir"
        if [ ! -d "$proto_dir" ]; then
            return
        fi
        
        local out_dir="$INTERNAL_OUTPUT/$rel_dir"
        mkdir -p "$out_dir"
        
        # 清理旧文件
        find "$out_dir" -type f -name "*.pb.go" -delete 2>/dev/null || true
        
        # 检查是否有 proto 文件
        local proto_files=("$proto_dir"/*.proto)
        if [ ! -f "${proto_files[0]}" ]; then
            return
        fi
        
        echo -e "${YELLOW}  生成 $rel_dir -> $out_dir${NC}"
        
        # 从 proto 根目录调用，使用完整路径，这样生成的 source 注释和变量名会包含完整路径
        pushd "$IARNET_PROTO_PROTO_DIR" >/dev/null
        for proto_file in "$rel_dir"/*.proto; do
            if [ -f "$proto_file" ]; then
                $PROTOC -I "$IARNET_PROTO_PROTO_DIR" \
                    --go_out="$INTERNAL_OUTPUT" --go_opt=paths=source_relative \
                    --go-grpc_out="$INTERNAL_OUTPUT" --go-grpc_opt=paths=source_relative \
                    "$proto_file"
            fi
        done
        popd >/dev/null
    }
    
    # 1. 生成 common
    generate_go_for_dir "common"
    
    # 2. 生成 ignis/controller
    generate_go_for_dir "ignis/controller"
    
    # 3. 生成 ignis/actor
    generate_go_for_dir "ignis/actor"
    
    # 4. 生成 resource（包括所有子目录）
    # resource 根目录
    if [ -f "$IARNET_PROTO_PROTO_DIR/resource/resource.proto" ]; then
        mkdir -p "$INTERNAL_OUTPUT/resource"
        find "$INTERNAL_OUTPUT/resource" -maxdepth 1 -type f -name "*.pb.go" -delete 2>/dev/null || true
        echo -e "${YELLOW}  生成 resource/resource.proto -> $INTERNAL_OUTPUT/resource${NC}"
        pushd "$IARNET_PROTO_PROTO_DIR" >/dev/null
        $PROTOC -I "$IARNET_PROTO_PROTO_DIR" \
            --go_out="$INTERNAL_OUTPUT" --go_opt=paths=source_relative \
            --go-grpc_out="$INTERNAL_OUTPUT" --go-grpc_opt=paths=source_relative \
            resource/resource.proto
        popd >/dev/null
    fi
    
    # resource 子目录
    for subdir in provider store component logger discovery scheduler; do
        generate_go_for_dir "resource/$subdir"
    done
    
    # 5. 生成 application/logger
    generate_go_for_dir "application/logger"
    
    # 6. 生成 global/registry
    generate_go_for_dir "global/registry"
    
    echo -e "${GREEN}✔ internal 模块生成完成${NC}"
}

# ============================================================================
# 模块 2: runner - 生成 Go 代码到 containers/runner/core/proto/
# ============================================================================
generate_runner() {
    echo ""
    echo -e "${YELLOW}>>> 模块: runner${NC}"
    
    RUNNER_PROTO_DIR="${PROJECT_ROOT}/containers/runner/core/proto"
    
    # 清理已有的 proto 代码
    if [ -d "$RUNNER_PROTO_DIR" ]; then
        echo -e "${YELLOW}  清理已有的 proto 代码...${NC}"
        find "$RUNNER_PROTO_DIR" -type f -name "*.pb.go" -delete 2>/dev/null || true
    fi
    
    # Runner common Go generation
    if [ -d "$IARNET_PROTO_PROTO_DIR/common" ]; then
        RUNNER_COMMON_OUTPUT="${RUNNER_PROTO_DIR}/common"
        echo -e "${YELLOW}  生成 Runner Common Go 代码: $RUNNER_COMMON_OUTPUT${NC}"
        mkdir -p "$RUNNER_COMMON_OUTPUT"
        find "$RUNNER_COMMON_OUTPUT" -type f -name "*.pb.go" -delete 2>/dev/null || true
        
        # 从 proto 根目录调用，使用完整路径
        # 输出到 RUNNER_PROTO_DIR，protoc 会自动创建 common/ 子目录
        pushd "$IARNET_PROTO_PROTO_DIR" >/dev/null
        for proto_file in common/*.proto; do
            if [ -f "$proto_file" ]; then
                $PROTOC -I "$IARNET_PROTO_PROTO_DIR" \
                    --go_out="$RUNNER_PROTO_DIR" --go_opt=paths=source_relative \
                    --go-grpc_out="$RUNNER_PROTO_DIR" --go-grpc_opt=paths=source_relative \
                    "$proto_file"
            fi
        done
        popd >/dev/null
    fi
    
    # Runner application logger Go generation
    if [ -d "$IARNET_PROTO_PROTO_DIR/application/logger" ]; then
        RUNNER_LOGGER_OUTPUT="${RUNNER_PROTO_DIR}/logger"
        echo -e "${YELLOW}  生成 Runner Logger Go 代码: $RUNNER_LOGGER_OUTPUT${NC}"
        mkdir -p "$RUNNER_LOGGER_OUTPUT"
        find "$RUNNER_LOGGER_OUTPUT" -type f -name "*.pb.go" -delete 2>/dev/null || true
        
        # 从 proto 根目录调用，使用完整路径
        # 先输出到临时目录，然后移动到目标目录
        pushd "$IARNET_PROTO_PROTO_DIR" >/dev/null
        for proto_file in application/logger/*.proto; do
            if [ -f "$proto_file" ]; then
                $PROTOC -I "$IARNET_PROTO_PROTO_DIR" \
                    --go_out="$RUNNER_PROTO_DIR" --go_opt=paths=source_relative \
                    --go-grpc_out="$RUNNER_PROTO_DIR" --go-grpc_opt=paths=source_relative \
                    "$proto_file"
            fi
        done
        popd >/dev/null
        
        # 移动生成的文件从 application/logger/ 到 logger/
        if [ -d "$RUNNER_PROTO_DIR/application/logger" ]; then
            mv "$RUNNER_PROTO_DIR/application/logger"/* "$RUNNER_LOGGER_OUTPUT/" 2>/dev/null || true
            rmdir "$RUNNER_PROTO_DIR/application/logger" 2>/dev/null || true
            rmdir "$RUNNER_PROTO_DIR/application" 2>/dev/null || true
        fi
        
        # 修复 import 路径（如果需要）
        find "$RUNNER_LOGGER_OUTPUT" -type f -name "*.pb.go" -exec sed -i 's|github.com/9triver/iarnet/internal/proto/common|github.com/9triver/iarnet/runner/proto/common|g' {} + 2>/dev/null || true
    fi
    
    echo -e "${GREEN}✔ runner 模块生成完成${NC}"
}

# ============================================================================
# 模块 3: component - 生成 Python 代码到 containers/component/python/proto/
# ============================================================================
generate_component() {
    echo ""
    echo -e "${YELLOW}>>> 模块: component${NC}"
    echo -e "${YELLOW}输出目录: ${PROJECT_ROOT}/containers/component/python/proto${NC}"
    
    PY_OUTPUT_COMPONENT="${PROJECT_ROOT}/containers/component/python/proto"
    
    # 清理已有的 proto 代码
    if [ -d "$PY_OUTPUT_COMPONENT" ]; then
        echo -e "${YELLOW}  清理已有的 proto 代码...${NC}"
        find "$PY_OUTPUT_COMPONENT" -type f -name "*_pb2.py" -delete 2>/dev/null || true
        find "$PY_OUTPUT_COMPONENT" -type f -name "*_pb2.pyi" -delete 2>/dev/null || true
        find "$PY_OUTPUT_COMPONENT" -type f -name "*_pb2_grpc.py" -delete 2>/dev/null || true
    fi
    
    # 生成函数：为指定目录生成 Python 代码
    generate_python_for_dir() {
        local rel_dir="$1"
        
        local proto_dir="$IARNET_PROTO_PROTO_DIR/$rel_dir"
        if [ ! -d "$proto_dir" ]; then
            return
        fi
        
        # 检查是否有 proto 文件
        local proto_files=("$proto_dir"/*.proto)
        if [ ! -f "${proto_files[0]}" ]; then
            return
        fi
        
        echo -e "${YELLOW}  生成 $rel_dir Python 代码${NC}"
        
        # 从 proto 根目录调用，使用完整路径
        pushd "$IARNET_PROTO_PROTO_DIR" >/dev/null
        for proto_file in "$rel_dir"/*.proto; do
            if [ -f "$proto_file" ]; then
                $PROTOC -I "$IARNET_PROTO_PROTO_DIR" \
                    --python_out="$PY_OUTPUT_COMPONENT" \
                    --pyi_out="$PY_OUTPUT_COMPONENT" \
                    --grpc_python_out="$PY_OUTPUT_COMPONENT" \
                    "$proto_file"
            fi
        done
        popd >/dev/null
    }
    
    # 1. 生成 common
    generate_python_for_dir "common"
    
    # 2. 生成 ignis/controller
    generate_python_for_dir "ignis/controller"
    
    # 3. 生成 ignis/actor
    generate_python_for_dir "ignis/actor"
    
    # 4. 生成 resource（包括所有子目录）
    # resource 根目录
    if [ -f "$IARNET_PROTO_PROTO_DIR/resource/resource.proto" ]; then
        echo -e "${YELLOW}  生成 resource/resource.proto Python 代码${NC}"
        pushd "$IARNET_PROTO_PROTO_DIR" >/dev/null
        $PROTOC -I "$IARNET_PROTO_PROTO_DIR" \
            --python_out="$PY_OUTPUT_COMPONENT" \
            --pyi_out="$PY_OUTPUT_COMPONENT" \
            --grpc_python_out="$PY_OUTPUT_COMPONENT" \
            resource/resource.proto
        popd >/dev/null
    fi
    
    # resource 子目录
    for subdir in provider store component logger discovery scheduler; do
        generate_python_for_dir "resource/$subdir"
    done
    
    # 5. 生成 application/logger
    generate_python_for_dir "application/logger"
    
    # 6. 生成 global/registry
    generate_python_for_dir "global/registry"
    
    echo -e "${GREEN}✔ component 模块生成完成${NC}"
}

# ============================================================================
# 模块 4: lucas - 生成 Python 代码到 third_party/lucas/lucas/actorc/protos/
# ============================================================================
generate_lucas() {
    echo ""
    echo -e "${YELLOW}>>> 模块: lucas${NC}"
    
    # Lucas Python 输出（特殊处理，优先使用 third_party，其次兼容旧路径）
    LUCAS_BASE_DIR="${PROJECT_ROOT}/third_party/lucas"
    if [ ! -d "$LUCAS_BASE_DIR" ]; then
        LUCAS_BASE_DIR="${PROJECT_ROOT}/containers/envs/python/libs/lucas"
    fi
    PY_OUTPUT_LUCAS="${LUCAS_BASE_DIR}/lucas/actorc/protos"
    
    # 清理已有的 proto 代码
    if [ -d "$PY_OUTPUT_LUCAS" ]; then
        echo -e "${YELLOW}  清理已有的 proto 代码...${NC}"
        find "$PY_OUTPUT_LUCAS" -type f -name "*_pb2.py" -delete 2>/dev/null || true
        find "$PY_OUTPUT_LUCAS" -type f -name "*_pb2.pyi" -delete 2>/dev/null || true
        find "$PY_OUTPUT_LUCAS" -type f -name "*_pb2_grpc.py" -delete 2>/dev/null || true
    fi
    
    # 修复导入路径的函数：将当前模块的绝对导入改为相对导入
    # 注意：跨模块导入（如 logger 导入 common）需要保留
    fix_imports() {
        local target_dir="$1"
        local module_path="$2"
        
        # 将模块路径转换为点分隔格式（如 ignis/controller -> ignis.controller）
        local module_dot="${module_path//\//\.}"
        
        # 修复 *_pb2.py、*_pb2.pyi 和 *_pb2_grpc.py 中的导入
        find "$target_dir" -type f \( -name "*_pb2.py" -o -name "*_pb2.pyi" -o -name "*_pb2_grpc.py" \) | while read -r file; do
            # 修复当前模块的导入（如 ignis.controller -> controller_pb2）
            # 匹配: from ignis.controller import controller_pb2 as xxx
            sed -i "s/from ${module_dot} import \([^ ]*\)_pb2 as \([^ ]*\)/import \1_pb2 as \2/g" "$file"
            # 匹配: from ignis.controller import controller_pb2
            sed -i "s/from ${module_dot} import \([^ ]*\)_pb2$/import \1_pb2/g" "$file"
            
            # 修复嵌套导入（如 from ignis.controller.xxx import）
            sed -i "s/from ${module_dot}\.\([^ ]*\) import \([^ ]*\) as \([^ ]*\)/import \2 as \3/g" "$file"
            sed -i "s/from ${module_dot}\.\([^ ]*\) import \([^ ]*\)$/import \2/g" "$file"
        done
    }
    
    # 1. Common
    if [ -d "$IARNET_PROTO_PROTO_DIR/common" ]; then
        echo -e "${YELLOW}  生成 Lucas Common Python 代码: $PY_OUTPUT_LUCAS/common${NC}"
        mkdir -p "$PY_OUTPUT_LUCAS/common"
        
        # 从 proto 根目录调用，使用完整路径，这样 source 注释会包含完整路径
        pushd "$IARNET_PROTO_PROTO_DIR" >/dev/null
        for proto_file in common/*.proto; do
            if [ -f "$proto_file" ]; then
                $PROTOC -I "$IARNET_PROTO_PROTO_DIR" \
                    --python_out="$PY_OUTPUT_LUCAS" \
                    --pyi_out="$PY_OUTPUT_LUCAS" \
                    --grpc_python_out="$PY_OUTPUT_LUCAS" \
                    "$proto_file"
            fi
        done
        popd >/dev/null
        
        # protoc 会生成到 common/ 目录，文件已经在正确位置
        # 修复导入路径（common 目录下的文件应该使用相对导入）
        fix_imports "$PY_OUTPUT_LUCAS/common" "common"
        touch "$PY_OUTPUT_LUCAS/common/__init__.py"
    fi
    
    # 2. Controller (从 ignis/controller 生成到 controller/)
    if [ -d "$IARNET_PROTO_PROTO_DIR/ignis/controller" ]; then
        echo -e "${YELLOW}  生成 Lucas Ignis Controller Python 代码: $PY_OUTPUT_LUCAS/controller${NC}"
        mkdir -p "$PY_OUTPUT_LUCAS/controller"
        
        # 从 proto 根目录调用，使用完整路径
        pushd "$IARNET_PROTO_PROTO_DIR" >/dev/null
        for proto_file in ignis/controller/*.proto; do
            if [ -f "$proto_file" ]; then
                $PROTOC -I "$IARNET_PROTO_PROTO_DIR" \
                    --python_out="$PY_OUTPUT_LUCAS" \
                    --pyi_out="$PY_OUTPUT_LUCAS" \
                    --grpc_python_out="$PY_OUTPUT_LUCAS" \
                    "$proto_file"
            fi
        done
        popd >/dev/null
        
        # protoc 会生成到 ignis/controller/，需要移动到 controller/
        if [ -d "$PY_OUTPUT_LUCAS/ignis/controller" ]; then
            mv "$PY_OUTPUT_LUCAS/ignis/controller"/* "$PY_OUTPUT_LUCAS/controller/" 2>/dev/null || true
            rmdir "$PY_OUTPUT_LUCAS/ignis/controller" 2>/dev/null || true
            rmdir "$PY_OUTPUT_LUCAS/ignis" 2>/dev/null || true
        fi
        
        # 修复导入路径：将 from ignis.controller import 改为 import controller_pb2
        fix_imports "$PY_OUTPUT_LUCAS/controller" "ignis/controller"
        touch "$PY_OUTPUT_LUCAS/controller/__init__.py"
    fi
    
    # 3. Application Logger (从 application/logger 生成到 logger/)
    if [ -d "$IARNET_PROTO_PROTO_DIR/application/logger" ]; then
        echo -e "${YELLOW}  生成 Lucas Application Logger Python 代码: $PY_OUTPUT_LUCAS/logger${NC}"
        mkdir -p "$PY_OUTPUT_LUCAS/logger"
        
        # 从 proto 根目录调用，使用完整路径
        pushd "$IARNET_PROTO_PROTO_DIR" >/dev/null
        for proto_file in application/logger/*.proto; do
            if [ -f "$proto_file" ]; then
                $PROTOC -I "$IARNET_PROTO_PROTO_DIR" \
                    --python_out="$PY_OUTPUT_LUCAS" \
                    --pyi_out="$PY_OUTPUT_LUCAS" \
                    --grpc_python_out="$PY_OUTPUT_LUCAS" \
                    "$proto_file"
            fi
        done
        popd >/dev/null
        
        # protoc 会生成到 application/logger/，需要移动到 logger/
        if [ -d "$PY_OUTPUT_LUCAS/application/logger" ]; then
            mv "$PY_OUTPUT_LUCAS/application/logger"/* "$PY_OUTPUT_LUCAS/logger/" 2>/dev/null || true
            rmdir "$PY_OUTPUT_LUCAS/application/logger" 2>/dev/null || true
            rmdir "$PY_OUTPUT_LUCAS/application" 2>/dev/null || true
        fi
        
        # 修复导入路径：将 from application.logger import 改为 import logger_pb2
        fix_imports "$PY_OUTPUT_LUCAS/logger" "application/logger"
        touch "$PY_OUTPUT_LUCAS/logger/__init__.py"
    fi
    
    echo -e "${GREEN}✔ lucas 模块生成完成${NC}"
}

# ============================================================================
# 模块 5: ignis - 生成 Go 代码到 third_party/ignis/ignis-go/proto/
# ============================================================================
generate_ignis() {
    echo ""
    echo -e "${YELLOW}>>> 模块: ignis${NC}"
    echo -e "${YELLOW}输出目录: ${PROJECT_ROOT}/third_party/ignis/ignis-go/proto${NC}"
    
    # 检查 protoc-gen-go 工具
    if ! command -v protoc-gen-go >/dev/null 2>&1 || ! command -v protoc-gen-go-grpc >/dev/null 2>&1; then
        cat >&2 <<EOF
错误: protoc-gen-go 和/或 protoc-gen-go-grpc 未在 PATH 中找到
安装方法:
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
EOF
        exit 1
    fi
    
    IGNIS_OUTPUT="${PROJECT_ROOT}/third_party/ignis/ignis-go/proto"
    
    # 清理已有的 proto 代码
    if [ -d "$IGNIS_OUTPUT" ]; then
        echo -e "${YELLOW}  清理已有的 proto 代码...${NC}"
        find "$IGNIS_OUTPUT/common" -type f -name "*.pb.go" -delete 2>/dev/null || true
        find "$IGNIS_OUTPUT/resource" -type f -name "*.pb.go" -delete 2>/dev/null || true
    fi
    
    # 生成函数：为指定目录生成 Go 代码到 ignis
    generate_go_for_ignis() {
        local rel_dir="$1"
        
        local proto_dir="$IARNET_PROTO_PROTO_DIR/$rel_dir"
        if [ ! -d "$proto_dir" ]; then
            return
        fi
        
        local out_dir="$IGNIS_OUTPUT/$rel_dir"
        mkdir -p "$out_dir"
        
        # 清理旧文件
        find "$out_dir" -type f -name "*.pb.go" -delete 2>/dev/null || true
        
        # 检查是否有 proto 文件
        local proto_files=("$proto_dir"/*.proto)
        if [ ! -f "${proto_files[0]}" ]; then
            return
        fi
        
        echo -e "${YELLOW}  生成 $rel_dir -> $out_dir${NC}"
        
        # 从 proto 根目录调用，使用完整路径
        pushd "$IARNET_PROTO_PROTO_DIR" >/dev/null
        for proto_file in "$rel_dir"/*.proto; do
            if [ -f "$proto_file" ]; then
                $PROTOC -I "$IARNET_PROTO_PROTO_DIR" \
                    --go_out="$IGNIS_OUTPUT" --go_opt=paths=source_relative \
                    --go-grpc_out="$IGNIS_OUTPUT" --go-grpc_opt=paths=source_relative \
                    "$proto_file"
            fi
        done
        popd >/dev/null
        
        # 修复 import 路径：将 iarnet 的路径改为 ignis 的路径
        echo -e "${YELLOW}  修复 import 路径...${NC}"
        find "$out_dir" -type f -name "*.pb.go" -exec sed -i \
            -e 's|github.com/9triver/iarnet/internal/proto/|github.com/9triver/ignis/proto/|g' \
            -e 's|"github.com/9triver/iarnet/internal/proto/|"github.com/9triver/ignis/proto/|g' \
            {} + 2>/dev/null || true
    }
    
    # 1. 生成 common
    generate_go_for_ignis "common"
    
    # 2. 生成 resource/store
    generate_go_for_ignis "resource/store"
    
    echo -e "${GREEN}✔ ignis 模块生成完成${NC}"
}

# ============================================================================
# 主逻辑：根据参数决定生成哪些模块
# ============================================================================
MODULE="${1:-all}"

case "$MODULE" in
    internal)
        generate_internal
        ;;
    runner)
        generate_runner
        ;;
    component)
        generate_component
        ;;
    lucas)
        generate_lucas
        ;;
    ignis)
        generate_ignis
        ;;
    all)
        generate_internal
        generate_runner
        generate_component
        generate_lucas
        generate_ignis
        ;;
    *)
        cat >&2 <<EOF
错误: 未知的模块名: $MODULE

可用模块:
  - internal: 生成 Go 代码到 internal/proto/
  - runner: 生成 Go 代码到 containers/runner/core/proto/
  - component: 生成 Python 代码到 containers/component/python/proto/
  - lucas: 生成 Python 代码到 third_party/lucas/lucas/actorc/protos/
  - ignis: 生成 Go 代码到 third_party/ignis/ignis-go/proto/
  - all: 生成所有模块（默认）

示例:
  ./proto/protobuf-gen.sh internal
  ./proto/protobuf-gen.sh ignis
  ./proto/protobuf-gen.sh all
EOF
        exit 1
        ;;
esac

echo ""
echo "=========================================="
echo -e "${GREEN}Protobuf 生成完成！${NC}"
echo "=========================================="
