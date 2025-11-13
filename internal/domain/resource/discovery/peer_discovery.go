package discovery

// import (
// 	"context"
// 	"log"
// 	"sync"
// 	"time"

// 	"google.golang.org/grpc"
// 	"google.golang.org/grpc/credentials/insecure"

// 	"github.com/9triver/iarnet/internal/resource"
// 	"github.com/9triver/iarnet/proto"
// )

// type PeerManager struct {
// 	peers               map[string]struct{}
// 	discoveredProviders map[string]*DiscoveredProvider // provider_id -> DiscoveredProvider
// 	resMgr              *resource.Manager
// 	mu                  sync.Mutex
// }

// // DiscoveredProvider represents a provider discovered through gossip
// type DiscoveredProvider struct {
// 	ID          string
// 	Name        string
// 	Type        string
// 	Host        string
// 	Port        int
// 	Status      int
// 	PeerAddress string
// 	LastSeen    time.Time
// }

// func NewPeerManager(initialPeers []string, resMgr *resource.Manager) *PeerManager {
// 	pm := &PeerManager{
// 		peers:               make(map[string]struct{}),
// 		discoveredProviders: make(map[string]*DiscoveredProvider),
// 		resMgr:              resMgr,
// 	}
// 	for _, p := range initialPeers {
// 		pm.peers[p] = struct{}{}
// 	}
// 	return pm
// }

// func (pm *PeerManager) GetPeers() []string {
// 	pm.mu.Lock()
// 	defer pm.mu.Unlock()
// 	var list []string
// 	for p := range pm.peers {
// 		list = append(list, p)
// 	}
// 	return list
// }

// func (pm *PeerManager) AddPeers(newPeers []string) {
// 	pm.mu.Lock()
// 	defer pm.mu.Unlock()
// 	for _, p := range newPeers {
// 		pm.peers[p] = struct{}{}
// 	}
// }

// // RemovePeer removes a peer from the known peers list
// func (pm *PeerManager) RemovePeer(peerAddr string) bool {
// 	pm.mu.Lock()
// 	defer pm.mu.Unlock()
// 	if _, exists := pm.peers[peerAddr]; exists {
// 		delete(pm.peers, peerAddr)
// 		return true
// 	}
// 	return false
// }

// // GetDiscoveredProviders returns all discovered providers
// func (pm *PeerManager) GetDiscoveredProviders() []*DiscoveredProvider {
// 	pm.mu.Lock()
// 	defer pm.mu.Unlock()

// 	var providers []*DiscoveredProvider
// 	for _, provider := range pm.discoveredProviders {
// 		providers = append(providers, provider)
// 	}
// 	return providers
// }

// // UpdateDiscoveredProviders updates the list of discovered providers
// func (pm *PeerManager) UpdateDiscoveredProviders(providers []*proto.ProviderInfo, peerAddr string) {
// 	pm.mu.Lock()
// 	defer pm.mu.Unlock()

// 	now := time.Now()
// 	for _, p := range providers {
// 		// Skip providers that are managed by the current peer (avoid self-discovery)
// 		if p.PeerAddress == "" || p.PeerAddress == peerAddr {
// 			continue
// 		}

// 		discovered := &DiscoveredProvider{
// 			ID:          p.Id,
// 			Name:        p.Name,
// 			Type:        p.Type,
// 			Host:        p.Host,
// 			Port:        int(p.Port),
// 			Status:      int(p.Status),
// 			PeerAddress: peerAddr,
// 			LastSeen:    now,
// 		}

// 		pm.discoveredProviders[p.Id] = discovered
// 	}

// 	// Register discovered providers with resource manager
// 	pm.registerDiscoveredProviders()
// }

// // registerDiscoveredProviders creates remote provider proxies and registers them
// func (pm *PeerManager) registerDiscoveredProviders() {
// 	for _, discovered := range pm.discoveredProviders {
// 		// Check if this provider is already registered
// 		providers := pm.resMgr.GetProviders()
// 		alreadyRegistered := false

// 		for _, existing := range providers.LocalProviders {
// 			if existing.GetID() == discovered.ID {
// 				alreadyRegistered = true
// 				break
// 			}
// 		}

// 		if !alreadyRegistered {
// 			// Create remote provider proxy
// 			remoteProvider, err := resource.NewRemoteProvider(
// 				discovered.ID,
// 				discovered.Name,
// 				discovered.Type,
// 				discovered.Host,
// 				discovered.Port,
// 				discovered.PeerAddress,
// 			)
// 			if err != nil {
// 				log.Printf("Failed to create remote provider %s: %v", discovered.ID, err)
// 				continue
// 			}

// 			// Register as discovered provider
// 			pm.resMgr.RegisterDiscoveredProvider(remoteProvider)
// 			log.Printf("Registered discovered provider: %s (%s) from peer %s", discovered.Name, discovered.Type, discovered.PeerAddress)
// 		}
// 	}
// }

// // CleanupStaleProviders removes providers that haven't been seen recently
// func (pm *PeerManager) CleanupStaleProviders(maxAge time.Duration) {
// 	pm.mu.Lock()
// 	defer pm.mu.Unlock()

// 	now := time.Now()
// 	for id, provider := range pm.discoveredProviders {
// 		if now.Sub(provider.LastSeen) > maxAge {
// 			delete(pm.discoveredProviders, id)
// 			log.Printf("Removed stale discovered provider: %s", id)
// 		}
// 	}
// }

// // Gossip: Periodically exchange with known peers
// func (pm *PeerManager) StartGossip(ctx context.Context) {
// 	gossipTicker := time.NewTicker(30 * time.Second)
// 	cleanupTicker := time.NewTicker(5 * time.Minute)

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return
// 		case <-gossipTicker.C:
// 			pm.gossipOnce()
// 		case <-cleanupTicker.C:
// 			// Clean up providers not seen for more than 10 minutes
// 			pm.CleanupStaleProviders(10 * time.Minute)
// 		}
// 	}
// }

// func (pm *PeerManager) gossipOnce() {
// 	known := pm.GetPeers()
// 	for _, peerAddr := range known {
// 		conn, err := grpc.Dial(peerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
// 		if err != nil {
// 			log.Printf("Failed to dial peer %s: %v", peerAddr, err)
// 			continue
// 		}

// 		client := proto.NewPeerServiceClient(conn)

// 		// Exchange peer information
// 		peerResp, err := client.ExchangePeers(context.Background(), &proto.ExchangeRequest{KnownPeers: known})
// 		if err != nil {
// 			log.Printf("Peer exchange failed with %s: %v", peerAddr, err)
// 			conn.Close()
// 			continue
// 		}
// 		pm.AddPeers(peerResp.KnownPeers)

// 		// Exchange provider information
// 		internalProviders := pm.getInternalProvidersForExchange()
// 		providerResp, err := client.ExchangeProviders(context.Background(), &proto.ProviderExchangeRequest{
// 			Providers: internalProviders,
// 		})
// 		if err != nil {
// 			log.Printf("Provider exchange failed with %s: %v", peerAddr, err)
// 		} else {
// 			// Update discovered providers
// 			pm.UpdateDiscoveredProviders(providerResp.Providers, peerAddr)
// 		}

// 		conn.Close()
// 	}
// 	log.Printf("Known peers after gossip: %v", pm.GetPeers())
// 	log.Printf("Discovered providers: %d", len(pm.discoveredProviders))
// }

// // getInternalProvidersForExchange converts internal providers to protobuf format for exchange
// func (pm *PeerManager) getInternalProvidersForExchange() []*proto.ProviderInfo {
// 	providers := pm.resMgr.GetProviders()
// 	var internalProviders []*proto.ProviderInfo

// 	// Add local providers (includes internal and external managed)
// 	for _, provider := range providers.LocalProviders {
// 		internalProviders = append(internalProviders, &proto.ProviderInfo{
// 			Id:          provider.GetID(),
// 			Name:        provider.GetName(),
// 			Type:        provider.GetType(),
// 			Host:        provider.GetHost(),
// 			Port:        int32(provider.GetPort()),
// 			Status:      int32(provider.GetStatus()),
// 			PeerAddress: "", // Local provider
// 		})
// 	}

// 	return internalProviders
// }
