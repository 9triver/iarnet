package rpc

import (
	"github.com/9triver/iarnet/internal/ignis/controller"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	storepb "github.com/9triver/iarnet/internal/proto/resource/store"
	"github.com/9triver/iarnet/internal/resource/store"
	"google.golang.org/grpc"
)

func RegisterIgnisServer(server *grpc.Server, controllerManager controller.Manager) {
	ctrlpb.RegisterServiceServer(server, controllerManager)
}

func RegisterStoreServiceServer(server *grpc.Server, storeService store.Service) {
	storepb.RegisterServiceServer(server, storeService)
}
