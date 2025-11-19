#!/bin/bash

# Unified Protobuf generation script
# Generates Go and Python code from all proto files in the project
# 
# Usage: ./protobuf-gen.sh

# TODO: refactor

# This script generates protobuf files for:
# - common: Common types and messages
# - ignis: Ignis execution engine
# - resource: Resource management

set -e  # Exit on error

PROTOC="python -m grpc_tools.protoc"
export PATH="$PATH:$HOME/go/bin"

# Base directory (proto root)
BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$BASE_DIR/.." && pwd)"
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
# Run from BASE_DIR so that proto files are registered with full path (common/types.proto)
# This ensures that when controller.proto imports "common/types.proto", the descriptor pool
# can find the correct file

PROTOC_CMD="$PROTOC -I $BASE_DIR"
PROTO_SRC="common/*.proto"

GO_OUTPUT="$PROJECT_ROOT/internal/proto/"
PY_OUTPUTS=("$PROJECT_ROOT/containers/envs/python/libs/lucas/lucas/actorc/protos/" "$PROJECT_ROOT/containers/component/python/proto/")

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
# 2. Generate ignis
# ============================================================================
echo ""
echo ">>> Generating ignis..."
cd "$BASE_DIR/ignis"

# Include common proto directory
# Run from ignis directory, so paths are relative to it
# Use BASE_DIR for -I so that "common/types.proto" imports work correctly
PROTOC_CMD="$PROTOC -I $BASE_DIR -I ."
PROTO_SRC="controller/*.proto actor/*.proto"

GO_OUTPUT="$PROJECT_ROOT/internal/proto/ignis"
PY_OUTPUT_COMPONENT="$PROJECT_ROOT/containers/component/python/proto/ignis"
PY_OUTPUT_LUCAS_COMMON="$PROJECT_ROOT/containers/envs/python/libs/lucas/lucas/actorc/protos/common"
PY_OUTPUT_LUCAS_CONTROLLER="$PROJECT_ROOT/containers/envs/python/libs/lucas/lucas/actorc/protos/controller"

# Go generation
echo "  Generating Go files: $GO_OUTPUT"
if [ ! -d "$GO_OUTPUT" ]; then
  mkdir -p "$GO_OUTPUT"
else
  find "$GO_OUTPUT" -type f -name "*.pb.go" -delete
fi
$PROTOC_CMD --go_out="$GO_OUTPUT" --go_opt=paths=source_relative --go-grpc_out="$GO_OUTPUT" --go-grpc_opt=paths=source_relative $PROTO_SRC

# Python generation for component
echo "  Generating Python files: $PY_OUTPUT_COMPONENT"
if [ ! -d "$PY_OUTPUT_COMPONENT" ]; then
  mkdir -p "$PY_OUTPUT_COMPONENT"
else
  find "$PY_OUTPUT_COMPONENT" -type f -name "*_pb2.py" -delete
  find "$PY_OUTPUT_COMPONENT" -type f -name "*_pb2.pyi" -delete
  find "$PY_OUTPUT_COMPONENT" -type f -name "*_pb2_grpc.py" -delete
fi
# Create subdirectories for controller and actor
mkdir -p "$PY_OUTPUT_COMPONENT/controller"
mkdir -p "$PY_OUTPUT_COMPONENT/actor"
# Generate controller and actor separately to avoid conflicts
# Controller proto files - change to controller directory to avoid nested structure
cd controller
PROTOC_CMD_CONTROLLER="$PROTOC -I . -I $BASE_DIR"
$PROTOC_CMD_CONTROLLER --python_out="$PY_OUTPUT_COMPONENT/controller" --pyi_out="$PY_OUTPUT_COMPONENT/controller" --grpc_python_out="$PY_OUTPUT_COMPONENT/controller" *.proto
cd ..
# Actor proto files - change to actor directory
cd actor
PROTOC_CMD_ACTOR="$PROTOC -I . -I $BASE_DIR"
$PROTOC_CMD_ACTOR --python_out="$PY_OUTPUT_COMPONENT/actor" --pyi_out="$PY_OUTPUT_COMPONENT/actor" --grpc_python_out="$PY_OUTPUT_COMPONENT/actor" *.proto
cd ..

# Python generation for lucas (only controller, common is already generated)
echo "  Generating Python files for lucas: $PY_OUTPUT_LUCAS_CONTROLLER"
if [ ! -d "$PY_OUTPUT_LUCAS_CONTROLLER" ]; then
  mkdir -p "$PY_OUTPUT_LUCAS_CONTROLLER"
else
  find "$PY_OUTPUT_LUCAS_CONTROLLER" -type f -name "*_pb2.py" -delete
  find "$PY_OUTPUT_LUCAS_CONTROLLER" -type f -name "*_pb2.pyi" -delete
  find "$PY_OUTPUT_LUCAS_CONTROLLER" -type f -name "*_pb2_grpc.py" -delete
fi
# Only generate controller for lucas, use module option to avoid nested directory
# Change to controller directory to generate files directly in target directory
cd controller
PROTOC_CMD_LUCAS="$PROTOC -I . -I $BASE_DIR"
$PROTOC_CMD_LUCAS --python_out="$PY_OUTPUT_LUCAS_CONTROLLER" --pyi_out="$PY_OUTPUT_LUCAS_CONTROLLER" --grpc_python_out="$PY_OUTPUT_LUCAS_CONTROLLER" *.proto
cd ..

# ============================================================================
# 3. Generate resource
# ============================================================================
echo ""
echo ">>> Generating resource..."
cd "$BASE_DIR/resource"

# Run from resource directory, so paths are relative to it
# Use BASE_DIR for -I so that "common/types.proto" imports work correctly
PROTOC_CMD="$PROTOC -I $BASE_DIR -I ."
PROTO_SRC="*.proto provider/*.proto store/*.proto component/*.proto logger/*.proto"

IARNET_OUTPUT="$PROJECT_ROOT/internal/proto/resource"
PY_OUTPUTS=("$PROJECT_ROOT/containers/component/python/proto/resource")

# Go generation
echo "  Generating Go files: $IARNET_OUTPUT"
if [ ! -d "$IARNET_OUTPUT" ]; then
  mkdir -p "$IARNET_OUTPUT"
else
  find "$IARNET_OUTPUT" -type f -name "*.pb.go" -delete
fi
$PROTOC_CMD --go_out="$IARNET_OUTPUT" --go_opt=paths=source_relative --go-grpc_out="$IARNET_OUTPUT" --go-grpc_opt=paths=source_relative $PROTO_SRC

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
# 4. Generate application
# ============================================================================
echo ""
echo ">>> Generating application..."
cd "$BASE_DIR/application"

PROTO_SRC="logger/*.proto"

IARNET_OUTPUT="$PROJECT_ROOT/internal/proto/application"

# Go generation
echo "  Generating Go files: $IARNET_OUTPUT"
if [ ! -d "$IARNET_OUTPUT" ]; then
  mkdir -p "$IARNET_OUTPUT"
else
  find "$IARNET_OUTPUT" -type f -name "*.pb.go" -delete
fi
$PROTOC_CMD --go_out="$IARNET_OUTPUT" --go_opt=paths=source_relative --go-grpc_out="$IARNET_OUTPUT" --go-grpc_opt=paths=source_relative $PROTO_SRC

cd "$BASE_DIR/application/logger"
PROTOC_CMD="$PROTOC -I . -I $BASE_DIR"
LUCAS_PROTO_SRC="*.proto"
LUCAS_PY_OUTPUT="$PROJECT_ROOT/containers/envs/python/libs/lucas/lucas/actorc/protos/logger"
# Python generation for lucas
mkdir -p "$LUCAS_PY_OUTPUT"
echo "  Generating Python files for lucas: $LUCAS_PY_OUTPUT"
if [ ! -d "$LUCAS_PY_OUTPUT" ]; then
  mkdir -p "$LUCAS_PY_OUTPUT"
else
  find "$LUCAS_PY_OUTPUT" -type f -name "*_pb2.py" -delete
  find "$LUCAS_PY_OUTPUT" -type f -name "*_pb2.pyi" -delete
  find "$LUCAS_PY_OUTPUT" -type f -name "*_pb2_grpc.py" -delete
fi
$PROTOC_CMD --python_out="$LUCAS_PY_OUTPUT" --pyi_out="$LUCAS_PY_OUTPUT" --grpc_python_out="$LUCAS_PY_OUTPUT" $LUCAS_PROTO_SRC

# ============================================================================
# 5. Generate peer.proto (if exists)
# ============================================================================
if [ -f "$BASE_DIR/peer.proto" ]; then
  echo ""
  echo ">>> Generating peer..."
  cd "$BASE_DIR"
  
  PROTOC_CMD="$PROTOC -I ."
  PROTO_SRC="peer.proto"
  
  GO_OUTPUT="$PROJECT_ROOT/internal/proto"
  PY_OUTPUTS=("$PROJECT_ROOT/containers/component/python/proto")
  
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

