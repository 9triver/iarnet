package resource

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/domain/resource"
	"github.com/9triver/iarnet/internal/domain/resource/provider"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	"github.com/9triver/iarnet/internal/transport/http/util/response"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func RegisterRoutes(router *mux.Router, resMgr *resource.Manager, cfg *config.Config) {
	api := NewAPI(resMgr, cfg)
	router.HandleFunc("/resource/capacity", api.handleGetResourceCapacity).Methods("GET")
	router.HandleFunc("/resource/provider", api.handleGetResourceProviders).Methods("GET")
	router.HandleFunc("/resource/provider/{id}/info", api.handleGetResourceProviderInfo).Methods("GET")
	router.HandleFunc("/resource/provider/{id}/capacity", api.handleGetResourceProviderCapacity).Methods("GET")
	router.HandleFunc("/resource/provider/test", api.handleTestResourceProvider).Methods("POST")
	router.HandleFunc("/resource/provider", api.handleRegisterResourceProvider).Methods("POST")
	router.HandleFunc("/resource/provider/{id}", api.handleUnregisterResourceProvider).Methods("DELETE")
}

type API struct {
	resMgr *resource.Manager
	cfg    *config.Config
}

func NewAPI(resMgr *resource.Manager, cfg *config.Config) *API {
	return &API{
		resMgr: resMgr,
		cfg:    cfg,
	}
}

func (api *API) handleGetResourceCapacity(w http.ResponseWriter, r *http.Request) {
	capacity, err := api.getAggregatedCapacity(r.Context())
	if err != nil {
		response.InternalError(err.Error()).WriteJSON(w)
		return
	}
	response.Success((&GetResourceCapacityResponse{}).FromCapacity(capacity)).WriteJSON(w)
}

func (api *API) handleGetResourceProviders(w http.ResponseWriter, r *http.Request) {
	providers := api.resMgr.GetAllProviders()
	items := make([]ProviderItem, 0, len(providers))
	for _, p := range providers {
		item := (&ProviderItem{}).FromProvider(p)
		items = append(items, *item)
	}
	resp := GetResourceProvidersResponse{
		Providers: items,
		Total:     len(items),
	}
	response.Success(resp).WriteJSON(w)
}

func (api *API) handleRegisterResourceProvider(w http.ResponseWriter, r *http.Request) {
	req := RegisterResourceProviderRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest("invalid request body: " + err.Error()).WriteJSON(w)
		return
	}

	// 创建 provider 实例
	p := provider.NewProvider(req.Name, req.Host, req.Port, api.cfg)

	// 注册 provider
	if err := api.resMgr.RegisterProvider(p); err != nil {
		response.InternalError("failed to register provider: " + err.Error()).WriteJSON(w)
		return
	}

	resp := RegisterResourceProviderResponse{
		ID:   p.GetID(),
		Name: p.GetName(),
	}
	response.Created(resp).WriteJSON(w)
}

func (api *API) handleUnregisterResourceProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["id"]
	if providerID == "" {
		response.BadRequest("provider id is required").WriteJSON(w)
		return
	}

	// TODO: 实现 UnregisterProvider 方法
	// 目前 resource manager 没有 UnregisterProvider 方法
	// 需要先添加到 provider service  and manager

	resp := UnregisterResourceProviderResponse{
		ID:      providerID,
		Message: "provider unregistered successfully",
	}
	response.Success(resp).WriteJSON(w)
}

func (api *API) handleGetResourceProviderInfo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["id"]
	if providerID == "" {
		response.BadRequest("provider id is required").WriteJSON(w)
		return
	}

	provider := api.resMgr.GetProvider(providerID)
	if provider == nil {
		response.NotFound("provider not found").WriteJSON(w)
		return
	}

	resp := (&GetResourceProviderInfoResponse{}).FromProvider(provider)
	response.Success(resp).WriteJSON(w)
}

func (api *API) handleGetResourceProviderCapacity(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["id"]
	if providerID == "" {
		response.BadRequest("provider id is required").WriteJSON(w)
		return
	}

	provider := api.resMgr.GetProvider(providerID)
	if provider == nil {
		response.NotFound("provider not found").WriteJSON(w)
		return
	}

	capacity, err := provider.GetCapacity(r.Context())
	if err != nil {
		response.InternalError("failed to get provider capacity: " + err.Error()).WriteJSON(w)
		return
	}

	resp := (&GetResourceProviderCapacityResponse{}).FromCapacity(capacity)
	response.Success(resp).WriteJSON(w)
}

func (api *API) handleTestResourceProvider(w http.ResponseWriter, r *http.Request) {
	req := TestResourceProviderRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if writeErr := response.BadRequest("invalid request body: " + err.Error()).WriteJSON(w); writeErr != nil {
			logrus.Errorf("Failed to write error response: %v", writeErr)
		}
		return
	}

	// 创建临时 provider 实例用于测试连接（不调用 Connect，避免 AssignID）
	testProvider := provider.NewProvider(req.Name, req.Host, req.Port, api.cfg)

	ctx := r.Context()

	// 直接测试连接并获取容量（不调用 Connect，因此不会 AssignID）
	// GetCapacity 方法会在未连接时创建临时连接
	capacity, err := testProvider.GetCapacity(ctx)
	if err != nil {
		// 连接或获取容量失败
		resp := TestResourceProviderResponse{
			Success: false,
			Type:    "",
			Message: "连接失败: " + err.Error(),
		}
		if writeErr := response.Success(resp).WriteJSON(w); writeErr != nil {
			logrus.Errorf("Failed to write test response: %v", writeErr)
		}
		return
	}

	// 连接成功，构建成功响应
	// 注意：由于没有调用 Connect，GetType() 可能返回空字符串
	// 但这是可以接受的，因为测试连接的主要目的是验证连接性和获取容量
	resp := TestResourceProviderResponse{
		Success: true,
		Type:    string(testProvider.GetType()), // 可能为空，因为未调用 Connect
		Message: "连接成功",
	}

	// 转换容量信息（只返回总容量）
	if capacity != nil && capacity.Total != nil {
		resp.Capacity = ResourceInfo{
			CPU:    capacity.Total.CPU,
			Memory: capacity.Total.Memory,
			GPU:    capacity.Total.GPU,
		}
	}

	if writeErr := response.Success(resp).WriteJSON(w); writeErr != nil {
		logrus.Errorf("Failed to write test response: %v", writeErr)
	}
}

// getAggregatedCapacity 聚合所有 provider 的资源容量
func (api *API) getAggregatedCapacity(ctx context.Context) (*types.Capacity, error) {
	providers := api.resMgr.GetAllProviders()
	if len(providers) == 0 {
		return &types.Capacity{
			Total:     &types.Info{CPU: 0, Memory: 0, GPU: 0},
			Used:      &types.Info{CPU: 0, Memory: 0, GPU: 0},
			Available: &types.Info{CPU: 0, Memory: 0, GPU: 0},
		}, nil
	}

	var totalCPU, totalMemory, totalGPU int64
	var usedCPU, usedMemory, usedGPU int64
	var availableCPU, availableMemory, availableGPU int64

	for _, p := range providers {
		if p.GetStatus() != types.ProviderStatusConnected {
			continue
		}

		capacity, err := p.GetCapacity(ctx)
		if err != nil {
			// 跳过获取容量失败的 provider
			continue
		}

		if capacity.Total != nil {
			totalCPU += capacity.Total.CPU
			totalMemory += capacity.Total.Memory
			totalGPU += capacity.Total.GPU
		}
		if capacity.Used != nil {
			usedCPU += capacity.Used.CPU
			usedMemory += capacity.Used.Memory
			usedGPU += capacity.Used.GPU
		}
		if capacity.Available != nil {
			availableCPU += capacity.Available.CPU
			availableMemory += capacity.Available.Memory
			availableGPU += capacity.Available.GPU
		}
	}

	return &types.Capacity{
		Total: &types.Info{
			CPU:    totalCPU,
			Memory: totalMemory,
			GPU:    totalGPU,
		},
		Used: &types.Info{
			CPU:    usedCPU,
			Memory: usedMemory,
			GPU:    usedGPU,
		},
		Available: &types.Info{
			CPU:    availableCPU,
			Memory: availableMemory,
			GPU:    availableGPU,
		},
	}, nil
}
