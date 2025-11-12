#!/bin/bash

PROTOC="python -m grpc_tools.protoc"
export PATH="$PATH:$HOME/go/bin"

PROTOC="$PROTOC -I ."
PROTO_SRC="*.proto ./provider/*.proto ./store/*.proto"

GO_OUTPUTS="../../internal/proto/resource"

for GO_OUTPUT in $GO_OUTPUTS; do
  echo "Generating protobuf files for Go: $GO_OUTPUT"

  if [ ! -d $GO_OUTPUT ]; then
    mkdir -p $GO_OUTPUT
  else
    find $GO_OUTPUT -type f -name "*.pb.go" -delete
  fi

  $PROTOC --go_out=$GO_OUTPUT --go_opt=paths=source_relative --go-grpc_out=$GO_OUTPUT --go-grpc_opt=paths=source_relative $PROTO_SRC
done
