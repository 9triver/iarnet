package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"github.com/9triver/iarnet/internal/discovery"
	"github.com/9triver/iarnet/internal/resource"
	pb "github.com/9triver/iarnet/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// GRPCServer handles peer-to-peer communication
type GRPCServer struct {
	pb.UnimplementedPeerServiceServer
	resMgr   *resource.Manager
	peerMgr  *discovery.PeerManager
	server   *grpc.Server
	listener net.Listener
}

// NewGRPCServer creates a new gRPC server for peer communication
func NewGRPCServer(resMgr *resource.Manager, peerMgr *discovery.PeerManager) *GRPCServer {
	return &GRPCServer{
		resMgr:  resMgr,
		peerMgr: peerMgr,
	}
}

// Start starts the gRPC server on the specified address
func (gs *GRPCServer) Start(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	gs.listener = lis
	gs.server = grpc.NewServer()
	pb.RegisterPeerServiceServer(gs.server, gs)

	logrus.Infof("Starting gRPC server on %s", addr)
	go func() {
		if err := gs.server.Serve(lis); err != nil {
			logrus.Errorf("gRPC server failed: %v", err)
		}
	}()

	return nil
}

// Stop stops the gRPC server
func (gs *GRPCServer) Stop() {
	if gs.server != nil {
		gs.server.GracefulStop()
	}
	if gs.listener != nil {
		gs.listener.Close()
	}
}

// ExchangePeers implements the peer exchange functionality
func (gs *GRPCServer) ExchangePeers(ctx context.Context, req *pb.ExchangeRequest) (*pb.ExchangeResponse, error) {
	logrus.Debugf("Received peer exchange request with %d known peers", len(req.KnownPeers))

	// Add received peers to our peer manager
	gs.peerMgr.AddPeers(req.KnownPeers)

	// Return our known peers
	return &pb.ExchangeResponse{
		KnownPeers: gs.peerMgr.GetPeers(),
	}, nil
}

// ExchangeProviders implements provider information exchange
func (gs *GRPCServer) ExchangeProviders(ctx context.Context, req *pb.ProviderExchangeRequest) (*pb.ProviderExchangeResponse, error) {
	logrus.Debugf("Received provider exchange request with %d providers", len(req.Providers))

	// Get categorized providers
	providers := gs.resMgr.GetProviders()
	var allProviders []*pb.ProviderInfo

	// Add local providers (includes internal and external managed)
	for _, provider := range providers.LocalProviders {
		allProviders = append(allProviders, &pb.ProviderInfo{
			Id:          provider.GetID(),
			Name:        provider.GetName(),
			Type:        provider.GetType(),
			Host:        provider.GetHost(),
			Port:        int32(provider.GetPort()),
			Status:      int32(provider.GetStatus()),
			PeerAddress: "", // Local provider, no peer address
		})
	}

	return &pb.ProviderExchangeResponse{
		Providers: allProviders,
	}, nil
}

// CallProvider implements remote provider method calls
func (gs *GRPCServer) CallProvider(ctx context.Context, req *pb.ProviderCallRequest) (*pb.ProviderCallResponse, error) {
	logrus.Debugf("Received provider call request for provider %s, method %s", req.ProviderId, req.Method)

	// Find the provider by ID
	providers := gs.resMgr.GetProviders()
	var targetProvider resource.Provider

	// Check local providers (includes internal and external managed)
	for _, provider := range providers.LocalProviders {
		if provider.GetID() == req.ProviderId {
			targetProvider = provider
			break
		}
	}

	if targetProvider == nil {
		return &pb.ProviderCallResponse{
			Success: false,
			Error:   fmt.Sprintf("provider %s not found", req.ProviderId),
		}, nil
	}

	// Execute the method call
	result, err := gs.executeProviderMethod(targetProvider, req.Method, req.Payload)
	if err != nil {
		return &pb.ProviderCallResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pb.ProviderCallResponse{
		Success: true,
		Result:  result,
	}, nil
}

// executeProviderMethod executes a method on the provider and returns serialized result
func (gs *GRPCServer) executeProviderMethod(provider resource.Provider, method string, payload []byte) ([]byte, error) {
	switch method {
	case "GetCapacity":
		capacity, err := provider.GetCapacity(context.Background())
		if err != nil {
			return nil, err
		}
		return json.Marshal(capacity)

	case "GetStatus":
		status := provider.GetStatus()
		return json.Marshal(status)

	case "Deploy":
		var params map[string]interface{}
		if err := json.Unmarshal(payload, &params); err != nil {
			return nil, fmt.Errorf("failed to unmarshal deploy parameters: %w", err)
		}

		// Extract ContainerSpec from parameters
		specData, err := json.Marshal(params["spec"])
		if err != nil {
			return nil, fmt.Errorf("failed to marshal spec: %w", err)
		}

		var spec resource.ContainerSpec
		if err := json.Unmarshal(specData, &spec); err != nil {
			return nil, fmt.Errorf("failed to unmarshal spec: %w", err)
		}

		containerID, err := provider.Deploy(context.Background(), spec)
		if err != nil {
			return nil, err
		}
		return json.Marshal(containerID)

	case "GetLogs":
		var params map[string]interface{}
		if err := json.Unmarshal(payload, &params); err != nil {
			return nil, fmt.Errorf("failed to unmarshal log parameters: %w", err)
		}

		// Extract parameters
		containerID, ok := params["containerID"].(string)
		if !ok {
			return nil, fmt.Errorf("invalid containerID parameter")
		}

		lines, ok := params["lines"].(float64) // JSON numbers are float64
		if !ok {
			return nil, fmt.Errorf("invalid lines parameter")
		}

		logs, err := provider.GetLogs(containerID, int(lines))
		if err != nil {
			return nil, err
		}
		return json.Marshal(logs)

	default:
		return nil, fmt.Errorf("unknown method: %s", method)
	}
}
