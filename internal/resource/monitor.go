package resource

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ProviderMonitor monitors provider health and handles failover
type ProviderMonitor struct {
	mu           sync.RWMutex
	providers    map[string]Provider
	healthChecks map[string]*HealthCheck
	manager      *Manager
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// HealthCheck represents the health status of a provider
type HealthCheck struct {
	Provider     Provider
	LastCheck    time.Time
	IsHealthy    bool
	FailureCount int
	MaxFailures  int
}

// NewProviderMonitor creates a new provider monitor
func NewProviderMonitor(manager *Manager) *ProviderMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &ProviderMonitor{
		providers:    make(map[string]Provider),
		healthChecks: make(map[string]*HealthCheck),
		manager:      manager,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start begins monitoring providers
func (pm *ProviderMonitor) Start() {
	pm.wg.Add(1)
	go pm.monitorLoop()
	logrus.Info("Provider monitor started")
}

// Stop stops the provider monitor
func (pm *ProviderMonitor) Stop() {
	pm.cancel()
	pm.wg.Wait()
	logrus.Info("Provider monitor stopped")
}

// AddProvider adds a provider to monitor
func (pm *ProviderMonitor) AddProvider(provider Provider) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	id := provider.GetID()
	pm.providers[id] = provider
	pm.healthChecks[id] = &HealthCheck{
		Provider:     provider,
		LastCheck:    time.Now(),
		IsHealthy:    true,
		FailureCount: 0,
		MaxFailures:  3, // Allow 3 consecutive failures before marking as unhealthy
	}

	logrus.Infof("Added provider %s to monitoring", id)
}

// RemoveProvider removes a provider from monitoring
func (pm *ProviderMonitor) RemoveProvider(providerID string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	delete(pm.providers, providerID)
	delete(pm.healthChecks, providerID)

	logrus.Infof("Removed provider %s from monitoring", providerID)
}

// GetHealthStatus returns the health status of a provider
func (pm *ProviderMonitor) GetHealthStatus(providerID string) (bool, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	healthCheck, exists := pm.healthChecks[providerID]
	if !exists {
		return false, false
	}

	return healthCheck.IsHealthy, true
}

// GetAllHealthStatus returns health status of all monitored providers
func (pm *ProviderMonitor) GetAllHealthStatus() map[string]bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	status := make(map[string]bool)
	for id, healthCheck := range pm.healthChecks {
		status[id] = healthCheck.IsHealthy
	}

	return status
}

// monitorLoop runs the main monitoring loop
func (pm *ProviderMonitor) monitorLoop() {
	defer pm.wg.Done()

	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.performHealthChecks()
		}
	}
}

// performHealthChecks checks the health of all monitored providers
func (pm *ProviderMonitor) performHealthChecks() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for id, healthCheck := range pm.healthChecks {
		isHealthy := pm.checkProviderHealth(healthCheck.Provider)
		healthCheck.LastCheck = time.Now()

		if isHealthy {
			if !healthCheck.IsHealthy {
				// Provider recovered
				logrus.Infof("Provider %s recovered", id)
				pm.handleProviderRecovery(healthCheck.Provider)
			}
			healthCheck.IsHealthy = true
			healthCheck.FailureCount = 0
		} else {
			healthCheck.FailureCount++
			logrus.Warnf("Provider %s health check failed (attempt %d/%d)", id, healthCheck.FailureCount, healthCheck.MaxFailures)

			if healthCheck.FailureCount >= healthCheck.MaxFailures && healthCheck.IsHealthy {
				// Provider failed
				logrus.Errorf("Provider %s marked as unhealthy after %d failures", id, healthCheck.FailureCount)
				healthCheck.IsHealthy = false
				pm.handleProviderFailure(healthCheck.Provider)
			}
		}
	}
}

// checkProviderHealth performs a health check on a provider
func (pm *ProviderMonitor) checkProviderHealth(provider Provider) bool {
	// For remote providers, try a simple ping/status call
	if remoteProvider, ok := provider.(*PeerProvider); ok {
		return pm.checkRemoteProviderHealth(remoteProvider)
	}

	// For local providers, check if they're still responsive
	return pm.checkLocalProviderHealth(provider)
}

// checkRemoteProviderHealth checks health of a remote provider
func (pm *ProviderMonitor) checkRemoteProviderHealth(provider *PeerProvider) bool {
	// Try to call a simple method to check connectivity
	_, err := provider.CallRemoteMethod("GetStatus", nil)
	if err != nil {
		logrus.Debugf("Remote provider %s health check failed: %v", provider.GetID(), err)
		return false
	}

	return true
}

// checkLocalProviderHealth checks health of a local provider
func (pm *ProviderMonitor) checkLocalProviderHealth(provider Provider) bool {
	// For local providers, we assume they're healthy if they exist
	// In a real implementation, you might want to check if the underlying service is responsive
	return provider.GetStatus() == StatusConnected
}

// handleProviderFailure handles when a provider fails
func (pm *ProviderMonitor) handleProviderFailure(provider Provider) {
	// Mark provider as inactive
	if localProvider, ok := provider.(interface{ SetStatus(Status) }); ok {
		localProvider.SetStatus(StatusDisconnected)
	}

	// Notify the resource manager about the failure
	pm.manager.HandleProviderFailure(provider.GetID())

	// Log the failure
	logrus.Errorf("Provider %s failed and marked as inactive", provider.GetID())
}

// handleProviderRecovery handles when a provider recovers
func (pm *ProviderMonitor) handleProviderRecovery(provider Provider) {
	// Mark provider as active
	if localProvider, ok := provider.(interface{ SetStatus(Status) }); ok {
		localProvider.SetStatus(StatusConnected)
	}

	// Notify the resource manager about the recovery
	pm.manager.HandleProviderRecovery(provider.GetID())

	// Log the recovery
	logrus.Infof("Provider %s recovered and marked as active", provider.GetID())
}
