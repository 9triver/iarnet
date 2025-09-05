package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "github.com/9triver/iarnet/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// PeerProvider represents a provider that exists on a remote peer
// All method calls are forwarded via gRPC to the actual provider
type PeerProvider struct {
	id           string
	name         string
	providerType string
	host         string
	port         int
	peerAddress  string // Address of the peer that manages this provider
	grpcClient   pb.PeerServiceClient
	conn         *grpc.ClientConn
}

// NewRemoteProvider creates a new remote provider proxy
func NewRemoteProvider(id, name, providerType, host string, port int, peerAddress string) (*PeerProvider, error) {
	// Establish gRPC connection to the peer
	conn, err := grpc.NewClient(peerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to peer %s: %w", peerAddress, err)
	}

	client := pb.NewPeerServiceClient(conn)

	return &PeerProvider{
		id:           id,
		name:         name,
		providerType: providerType,
		host:         host,
		port:         port,
		peerAddress:  peerAddress,
		grpcClient:   client,
		conn:         conn,
	}, nil
}

// Close closes the gRPC connection
func (rp *PeerProvider) Close() error {
	if rp.conn != nil {
		return rp.conn.Close()
	}
	return nil
}

// CallRemoteMethod makes a gRPC call to the remote provider
func (rp *PeerProvider) CallRemoteMethod(method string, params interface{}) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Serialize parameters
	payload, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize parameters: %w", err)
	}

	// Make gRPC call
	resp, err := rp.grpcClient.CallProvider(ctx, &pb.ProviderCallRequest{
		ProviderId: rp.id,
		Method:     method,
		Payload:    payload,
	})
	if err != nil {
		return nil, fmt.Errorf("gRPC call failed: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("remote method call failed: %s", resp.Error)
	}

	return resp.Result, nil
}

// Provider interface implementation

func (rp *PeerProvider) GetID() string {
	return rp.id
}

func (rp *PeerProvider) GetName() string {
	return rp.name
}

func (rp *PeerProvider) GetType() string {
	return rp.providerType
}

func (rp *PeerProvider) GetHost() string {
	return rp.host
}

func (rp *PeerProvider) GetPort() int {
	return rp.port
}

func (p *PeerProvider) GetCapacity(ctx context.Context) (*Capacity, error) {
	result, err := p.CallRemoteMethod("GetCapacity", nil)
	if err != nil {
		return nil, err
	}

	var capacity Capacity
	err = json.Unmarshal(result, &capacity)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize capacity: %w", err)
	}

	return &capacity, nil
}

func (p *PeerProvider) GetAllocated(ctx context.Context) (*Usage, error) {
	result, err := p.CallRemoteMethod("GetAllocated", nil)
	if err != nil {
		return nil, err
	}

	var usage Usage
	err = json.Unmarshal(result, &usage)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize usage: %w", err)
	}

	return &usage, nil
}

func (p *PeerProvider) GetStatus() Status {
	// For remote providers, we can try a simple method call to check connectivity
	result, err := p.CallRemoteMethod("GetStatus", nil)
	if err != nil {
		return StatusDisconnected
	}

	var status Status
	err = json.Unmarshal(result, &status)
	if err != nil {
		return StatusDisconnected
	}

	return status
}

func (p *PeerProvider) GetLastUpdateTime() time.Time {
	result, err := p.CallRemoteMethod("GetLastUpdateTime", nil)
	if err != nil {
		return time.Time{}
	}

	var lastUpdateTime time.Time
	err = json.Unmarshal(result, &lastUpdateTime)
	if err != nil {
		return time.Time{}
	}

	return lastUpdateTime
}

func (p *PeerProvider) Deploy(ctx context.Context, spec ContainerSpec) (string, error) {
	params := map[string]interface{}{
		"spec": spec,
	}
	result, err := p.CallRemoteMethod("Deploy", params)
	if err != nil {
		return "", err
	}

	var containerID string
	err = json.Unmarshal(result, &containerID)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal container ID: %w", err)
	}

	return containerID, nil
}

func (p *PeerProvider) GetLogs(containerID string, lines int) ([]string, error) {
	params := map[string]interface{}{
		"containerID": containerID,
		"lines":       lines,
	}

	result, err := p.CallRemoteMethod("GetLogs", params)
	if err != nil {
		return nil, err
	}

	var logs []string
	err = json.Unmarshal(result, &logs)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal logs: %w", err)
	}

	return logs, nil
}
