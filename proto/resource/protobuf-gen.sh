#!/bin/bash

PROTOC="python -m grpc_tools.protoc"
export PATH="$PATH:$HOME/go/bin"

PROTOC="$PROTOC -I ."
PROTO_SRC="*.proto ./provider/*.proto ./store/*.proto"

GO_OUTPUTS="../../internal/proto/resource"
PY_OUTPUTS=("../../containers/component/python/proto/resource")

for GO_OUTPUT in $GO_OUTPUTS; do
  echo "Generating protobuf files for Go: $GO_OUTPUT"

  if [ ! -d $GO_OUTPUT ]; then
    mkdir -p $GO_OUTPUT
  else
    find $GO_OUTPUT -type f -name "*.pb.go" -delete
  fi

  $PROTOC --go_out=$GO_OUTPUT --go_opt=paths=source_relative --go-grpc_out=$GO_OUTPUT --go-grpc_opt=paths=source_relative $PROTO_SRC
done

for PY_OUTPUT in "${PY_OUTPUTS[@]}"; do
  echo "Generating protobuf files for Python: $PY_OUTPUT"

  if [ ! -d $PY_OUTPUT ]; then
    mkdir -p $PY_OUTPUT
  else
    find $PY_OUTPUT -type f -name "*_pb2.py" -delete
    find $PY_OUTPUT -type f -name "*_pb2.pyi" -delete
    find $PY_OUTPUT -type f -name "*_pb2_grpc.py" -delete
  fi

  $PROTOC --python_out=$PY_OUTPUT --pyi_out=$PY_OUTPUT --grpc_python_out=$PY_OUTPUT $PROTO_SRC $ACTOR_PROTO
done