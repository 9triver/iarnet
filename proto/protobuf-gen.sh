#!/bin/bash

# Unified Protobuf generation script
# Generates Go and Python code from all proto files in the project
# 
# Usage: ./protobuf-gen.sh
# 
# This script generates protobuf files for:
# - common: Common types and messages
# - execution-ignis: Ignis execution engine
# - resource: Resource management

set -e  # Exit on error

PROTOC="python -m grpc_tools.protoc"
export PATH="$PATH:$HOME/go/bin"

ACTOR_SRC=$(go list -f {{.Dir}} github.com/asynkron/protoactor-go/actor 2>/dev/null || echo "")
ACTOR_PROTO=""
if [ -n "$ACTOR_SRC" ]; then
  ACTOR_PROTO="$ACTOR_SRC/actor.proto"
fi

# Base directory (proto root)
BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$BASE_DIR"

echo "=========================================="
echo "Protobuf Generation Script"
echo "Base directory: $BASE_DIR"
echo "=========================================="

# ============================================================================
# 1. Generate common types
# ============================================================================
echo ""
echo ">>> Generating common types..."
cd "$BASE_DIR/common"

PROTOC_CMD="$PROTOC -I ."
PROTO_SRC="*.proto"

GO_OUTPUT="../../internal/proto/common"
PY_OUTPUTS=("../../containers/envs/python/libs/lucas/lucas/actorc/protos" "../../containers/component/python/proto/common")

# Go generation
echo "  Generating Go files: $GO_OUTPUT"
if [ ! -d "$GO_OUTPUT" ]; then
  mkdir -p "$GO_OUTPUT"
else
  find "$GO_OUTPUT" -type f -name "*.pb.go" -delete
fi
$PROTOC_CMD --go_out="$GO_OUTPUT" --go_opt=paths=source_relative --go-grpc_out="$GO_OUTPUT" --go-grpc_opt=paths=source_relative $PROTO_SRC

# Python generation
for PY_OUTPUT in "${PY_OUTPUTS[@]}"; do
  echo "  Generating Python files: $PY_OUTPUT"
  if [ ! -d "$PY_OUTPUT" ]; then
    mkdir -p "$PY_OUTPUT"
  else
    find "$PY_OUTPUT" -type f -name "*_pb2.py" -delete
    find "$PY_OUTPUT" -type f -name "*_pb2.pyi" -delete
    find "$PY_OUTPUT" -type f -name "*_pb2_grpc.py" -delete
  fi
  $PROTOC_CMD --python_out="$PY_OUTPUT" --pyi_out="$PY_OUTPUT" --grpc_python_out="$PY_OUTPUT" $PROTO_SRC
done

# ============================================================================
# 2. Generate execution-ignis
# ============================================================================
echo ""
echo ">>> Generating execution-ignis..."
cd "$BASE_DIR/execution-ignis"

# Include common proto directory and actor proto
if [ -n "$ACTOR_SRC" ]; then
  PROTOC_CMD="$PROTOC -I $ACTOR_SRC -I ../common -I ."
else
  PROTOC_CMD="$PROTOC -I ../common -I ."
fi
PROTO_SRC="types.proto ./controller/*.proto ./actor/*.proto"

GO_OUTPUT="../../internal/proto/execution_ignis"
PY_OUTPUTS=("../../containers/envs/python/libs/lucas/lucas/actorc/protos" "../../containers/component/python/proto/execution_ignis")

# Go generation
echo "  Generating Go files: $GO_OUTPUT"
if [ ! -d "$GO_OUTPUT" ]; then
  mkdir -p "$GO_OUTPUT"
else
  find "$GO_OUTPUT" -type f -name "*.pb.go" -delete
fi
$PROTOC_CMD --go_out="$GO_OUTPUT" --go_opt=paths=source_relative --go-grpc_out="$GO_OUTPUT" --go-grpc_opt=paths=source_relative $PROTO_SRC

# Python generation
for PY_OUTPUT in "${PY_OUTPUTS[@]}"; do
  echo "  Generating Python files: $PY_OUTPUT"
  if [ ! -d "$PY_OUTPUT" ]; then
    mkdir -p "$PY_OUTPUT"
  else
    find "$PY_OUTPUT" -type f -name "*_pb2.py" -delete
    find "$PY_OUTPUT" -type f -name "*_pb2.pyi" -delete
    find "$PY_OUTPUT" -type f -name "*_pb2_grpc.py" -delete
  fi
  if [ -n "$ACTOR_PROTO" ]; then
    $PROTOC_CMD --python_out="$PY_OUTPUT" --pyi_out="$PY_OUTPUT" --grpc_python_out="$PY_OUTPUT" $PROTO_SRC "$ACTOR_PROTO"
  else
    $PROTOC_CMD --python_out="$PY_OUTPUT" --pyi_out="$PY_OUTPUT" --grpc_python_out="$PY_OUTPUT" $PROTO_SRC
  fi
done

# ============================================================================
# 3. Generate resource
# ============================================================================
echo ""
echo ">>> Generating resource..."
cd "$BASE_DIR/resource"

PROTOC_CMD="$PROTOC -I ../common -I ."
PROTO_SRC="*.proto ./provider/*.proto ./store/*.proto"

GO_OUTPUT="../../internal/proto/resource"
PY_OUTPUTS=("../../containers/component/python/proto/resource")

# Go generation
echo "  Generating Go files: $GO_OUTPUT"
if [ ! -d "$GO_OUTPUT" ]; then
  mkdir -p "$GO_OUTPUT"
else
  find "$GO_OUTPUT" -type f -name "*.pb.go" -delete
fi
$PROTOC_CMD --go_out="$GO_OUTPUT" --go_opt=paths=source_relative --go-grpc_out="$GO_OUTPUT" --go-grpc_opt=paths=source_relative $PROTO_SRC

# Python generation
for PY_OUTPUT in "${PY_OUTPUTS[@]}"; do
  echo "  Generating Python files: $PY_OUTPUT"
  if [ ! -d "$PY_OUTPUT" ]; then
    mkdir -p "$PY_OUTPUT"
  else
    find "$PY_OUTPUT" -type f -name "*_pb2.py" -delete
    find "$PY_OUTPUT" -type f -name "*_pb2.pyi" -delete
    find "$PY_OUTPUT" -type f -name "*_pb2_grpc.py" -delete
  fi
  $PROTOC_CMD --python_out="$PY_OUTPUT" --pyi_out="$PY_OUTPUT" --grpc_python_out="$PY_OUTPUT" $PROTO_SRC
done

# ============================================================================
# 4. Generate logger (if exists)
# ============================================================================
if [ -d "$BASE_DIR/logger" ] && [ -n "$(find "$BASE_DIR/logger" -name "*.proto" 2>/dev/null)" ]; then
  echo ""
  echo ">>> Generating logger..."
  cd "$BASE_DIR/logger"
  
  PROTOC_CMD="$PROTOC -I ."
  PROTO_SRC="*.proto"
  
  GO_OUTPUT="../../internal/proto/logger"
  PY_OUTPUTS=("../../containers/component/python/proto/logger")
  
  # Go generation
  echo "  Generating Go files: $GO_OUTPUT"
  if [ ! -d "$GO_OUTPUT" ]; then
    mkdir -p "$GO_OUTPUT"
  else
    find "$GO_OUTPUT" -type f -name "*.pb.go" -delete
  fi
  $PROTOC_CMD --go_out="$GO_OUTPUT" --go_opt=paths=source_relative --go-grpc_out="$GO_OUTPUT" --go-grpc_opt=paths=source_relative $PROTO_SRC
  
  # Python generation
  for PY_OUTPUT in "${PY_OUTPUTS[@]}"; do
    echo "  Generating Python files: $PY_OUTPUT"
    if [ ! -d "$PY_OUTPUT" ]; then
      mkdir -p "$PY_OUTPUT"
    else
      find "$PY_OUTPUT" -type f -name "*_pb2.py" -delete
      find "$PY_OUTPUT" -type f -name "*_pb2.pyi" -delete
      find "$PY_OUTPUT" -type f -name "*_pb2_grpc.py" -delete
    fi
    $PROTOC_CMD --python_out="$PY_OUTPUT" --pyi_out="$PY_OUTPUT" --grpc_python_out="$PY_OUTPUT" $PROTO_SRC
  done
fi

# ============================================================================
# 5. Generate peer.proto (if exists)
# ============================================================================
if [ -f "$BASE_DIR/peer.proto" ]; then
  echo ""
  echo ">>> Generating peer..."
  cd "$BASE_DIR"
  
  PROTOC_CMD="$PROTOC -I ."
  PROTO_SRC="peer.proto"
  
  GO_OUTPUT="../internal/proto"
  PY_OUTPUTS=("../containers/component/python/proto")
  
  # Go generation
  echo "  Generating Go files: $GO_OUTPUT"
  if [ ! -d "$GO_OUTPUT" ]; then
    mkdir -p "$GO_OUTPUT"
  else
    find "$GO_OUTPUT" -type f -name "peer*.pb.go" -delete
  fi
  $PROTOC_CMD --go_out="$GO_OUTPUT" --go_opt=paths=source_relative --go-grpc_out="$GO_OUTPUT" --go-grpc_opt=paths=source_relative $PROTO_SRC
  
  # Python generation
  for PY_OUTPUT in "${PY_OUTPUTS[@]}"; do
    echo "  Generating Python files: $PY_OUTPUT"
    if [ ! -d "$PY_OUTPUT" ]; then
      mkdir -p "$PY_OUTPUT"
    else
      find "$PY_OUTPUT" -type f -name "peer*_pb2.py" -delete
      find "$PY_OUTPUT" -type f -name "peer*_pb2.pyi" -delete
      find "$PY_OUTPUT" -type f -name "peer*_pb2_grpc.py" -delete
    fi
    $PROTOC_CMD --python_out="$PY_OUTPUT" --pyi_out="$PY_OUTPUT" --grpc_python_out="$PY_OUTPUT" $PROTO_SRC
  done
fi

echo ""
echo "=========================================="
echo "Protobuf generation completed!"
echo "=========================================="

