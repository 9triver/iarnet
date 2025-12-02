#!/bin/bash

# Clean script for protobuf generated files
# Removes all .pb.go, _pb2.py, _pb2.pyi, and _pb2_grpc.py files
# 
# Usage: ./clean-protobuf.sh

set -e  # Exit on error

# Base directory (proto root)
BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$BASE_DIR/.." && pwd)"

echo "=========================================="
echo "Protobuf Clean Script"
echo "Base directory: $BASE_DIR"
echo "=========================================="
echo ""

# Go generated files
GO_PROTO_DIR="$PROJECT_ROOT/internal/proto"
if [ -d "$GO_PROTO_DIR" ]; then
  echo ">>> Cleaning Go generated files in $GO_PROTO_DIR..."
  find "$GO_PROTO_DIR" -type f -name "*.pb.go" -delete
  find "$GO_PROTO_DIR" -type f -name "*_grpc.pb.go" -delete
  echo "  ✓ Go files cleaned"
else
  echo "  ⚠ Go proto directory not found: $GO_PROTO_DIR"
fi

# Python generated files - lucas（优先 third_party，其次兼容旧路径）
LUCAS_BASE_DIR="$PROJECT_ROOT/third_party/lucas"
if [ ! -d "$LUCAS_BASE_DIR" ]; then
  LUCAS_BASE_DIR="$PROJECT_ROOT/containers/envs/python/libs/lucas"
fi
PY_LUCAS_PROTO_DIR="$LUCAS_BASE_DIR/lucas/actorc/protos"
if [ -d "$PY_LUCAS_PROTO_DIR" ]; then
  echo ">>> Cleaning Python generated files in $PY_LUCAS_PROTO_DIR..."
  find "$PY_LUCAS_PROTO_DIR" -type f -name "*_pb2.py" -delete
  find "$PY_LUCAS_PROTO_DIR" -type f -name "*_pb2.pyi" -delete
  find "$PY_LUCAS_PROTO_DIR" -type f -name "*_pb2_grpc.py" -delete
  echo "  ✓ Python files (lucas) cleaned"
else
  echo "  ⚠ Python lucas proto directory not found: $PY_LUCAS_PROTO_DIR"
fi

# Python generated files - component
PY_COMPONENT_PROTO_DIR="$PROJECT_ROOT/containers/component/python/proto"
if [ -d "$PY_COMPONENT_PROTO_DIR" ]; then
  echo ">>> Cleaning Python generated files in $PY_COMPONENT_PROTO_DIR..."
  find "$PY_COMPONENT_PROTO_DIR" -type f -name "*_pb2.py" -delete
  find "$PY_COMPONENT_PROTO_DIR" -type f -name "*_pb2.pyi" -delete
  find "$PY_COMPONENT_PROTO_DIR" -type f -name "*_pb2_grpc.py" -delete
  echo "  ✓ Python files (component) cleaned"
else
  echo "  ⚠ Python component proto directory not found: $PY_COMPONENT_PROTO_DIR"
fi

echo ""
echo "=========================================="
echo "Protobuf files cleaned!"
echo "=========================================="

