package discovery

import (
	"context"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/9triver/iarnet/proto"
)

type PeerManager struct {
	peers map[string]struct{}
	mu    sync.Mutex
}

func NewPeerManager(initialPeers []string) *PeerManager {
	pm := &PeerManager{peers: make(map[string]struct{})}
	for _, p := range initialPeers {
		pm.peers[p] = struct{}{}
	}
	return pm
}

func (pm *PeerManager) GetPeers() []string {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	var list []string
	for p := range pm.peers {
		list = append(list, p)
	}
	return list
}

func (pm *PeerManager) AddPeers(newPeers []string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for _, p := range newPeers {
		pm.peers[p] = struct{}{}
	}
}

// Gossip: Periodically exchange with known peers
func (pm *PeerManager) StartGossip(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pm.gossipOnce()
		}
	}
}

func (pm *PeerManager) gossipOnce() {
	known := pm.GetPeers()
	for _, peerAddr := range known {
		conn, err := grpc.Dial(peerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Failed to dial peer %s: %v", peerAddr, err)
			continue
		}
		client := proto.NewPeerServiceClient(conn)
		resp, err := client.ExchangePeers(context.Background(), &proto.ExchangeRequest{KnownPeers: known})
		if err != nil {
			log.Printf("Exchange failed with %s: %v", peerAddr, err)
			conn.Close()
			continue
		}
		pm.AddPeers(resp.KnownPeers)
		conn.Close()
	}
	log.Printf("Known peers after gossip: %v", pm.GetPeers())
}
