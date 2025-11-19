package discovery

import (
	"context"
	"sync"
	"time"

	peerproto "github.com/9triver/iarnet/internal/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// DialPeerFunc abstracts how a peer service client is created, so tests can inject fakes.
type DialPeerFunc func(addr string) (peerproto.PeerServiceClient, func(), error)

// PeerManager maintains peer addresses and discovered providers via gossip.
type PeerManager struct {
	mu          sync.Mutex
	peers       map[string]struct{}
	providers   map[string]*discoveredProvider
	dialPeer    DialPeerFunc
	now         func() time.Time
	gossipPeers []string // cached slice to avoid reallocations
}

type discoveredProvider struct {
	info     *peerproto.ProviderInfo
	lastSeen time.Time
}

// NewPeerManager creates a peer manager with initial peer seeds.
func NewPeerManager(initial []string, dialer DialPeerFunc) *PeerManager {
	peers := make(map[string]struct{})
	for _, addr := range initial {
		if addr == "" {
			continue
		}
		peers[addr] = struct{}{}
	}
	if dialer == nil {
		dialer = defaultDialPeer
	}
	return &PeerManager{
		peers:     peers,
		providers: make(map[string]*discoveredProvider),
		dialPeer:  dialer,
		now:       time.Now,
	}
}

// Gossip performs a single gossip round with every known peer.
func (pm *PeerManager) Gossip(ctx context.Context) {
	peers := pm.snapshotPeers()
	for _, addr := range peers {
		client, closeFn, err := pm.dialPeer(addr)
		if err != nil {
			continue
		}

		pm.exchangePeers(ctx, client)
		pm.exchangeProviders(ctx, client, addr)

		closeFn()
	}
}

// GetPeers returns a copy of the peer list.
func (pm *PeerManager) GetPeers() []string {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	peers := make([]string, 0, len(pm.peers))
	for addr := range pm.peers {
		peers = append(peers, addr)
	}
	return peers
}

// GetDiscoveredProviders returns provider info snapshots.
func (pm *PeerManager) GetDiscoveredProviders() []*peerproto.ProviderInfo {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	result := make([]*peerproto.ProviderInfo, 0, len(pm.providers))
	for _, p := range pm.providers {
		result = append(result, p.info)
	}
	return result
}

// CleanupStaleProviders removes providers not updated within maxAge.
func (pm *PeerManager) CleanupStaleProviders(maxAge time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	cutoff := pm.now().Add(-maxAge)
	for id, p := range pm.providers {
		if p.lastSeen.Before(cutoff) {
			delete(pm.providers, id)
		}
	}
}

// exchangePeers merges remote peers into local list.
func (pm *PeerManager) exchangePeers(ctx context.Context, client peerproto.PeerServiceClient) {
	resp, err := client.ExchangePeers(ctx, &peerproto.ExchangeRequest{
		KnownPeers: pm.snapshotPeers(),
	})
	if err != nil || resp == nil {
		return
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for _, peer := range resp.KnownPeers {
		if peer == "" {
			continue
		}
		pm.peers[peer] = struct{}{}
	}
}

// exchangeProviders retrieves provider info and stores it locally.
func (pm *PeerManager) exchangeProviders(ctx context.Context, client peerproto.PeerServiceClient, peerAddr string) {
	resp, err := client.ExchangeProviders(ctx, &peerproto.ProviderExchangeRequest{
		Providers: nil,
	})
	if err != nil || resp == nil {
		return
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	now := pm.now()
	for _, info := range resp.Providers {
		if info == nil || info.Id == "" {
			continue
		}
		// Annotate provider with peer address of the source node
		info.PeerAddress = peerAddr
		pm.providers[info.Id] = &discoveredProvider{
			info:     info,
			lastSeen: now,
		}
	}
}

func (pm *PeerManager) snapshotPeers() []string {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if cap(pm.gossipPeers) < len(pm.peers) {
		pm.gossipPeers = make([]string, 0, len(pm.peers))
	} else {
		pm.gossipPeers = pm.gossipPeers[:0]
	}
	for addr := range pm.peers {
		pm.gossipPeers = append(pm.gossipPeers, addr)
	}
	return append([]string(nil), pm.gossipPeers...)
}

func defaultDialPeer(addr string) (peerproto.PeerServiceClient, func(), error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return peerproto.NewPeerServiceClient(conn), func() { conn.Close() }, nil
}

// InjectDialer allows tests to override the dialer after construction.
func (pm *PeerManager) InjectDialer(dialer DialPeerFunc) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if dialer == nil {
		dialer = defaultDialPeer
	}
	pm.dialPeer = dialer
}

// InjectNow allows tests to control current time.
func (pm *PeerManager) InjectNow(now func() time.Time) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if now == nil {
		pm.now = time.Now
		return
	}
	pm.now = now
}

// AddPeer adds a new peer address.
func (pm *PeerManager) AddPeer(addr string) {
	if addr == "" {
		return
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.peers[addr] = struct{}{}
}

// RemovePeer removes the peer address if present.
func (pm *PeerManager) RemovePeer(addr string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.peers, addr)
}

// GetProviderByID returns provider info if discovered.
func (pm *PeerManager) GetProviderByID(id string) (*peerproto.ProviderInfo, bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if dp, ok := pm.providers[id]; ok {
		return dp.info, true
	}
	return nil, false
}
