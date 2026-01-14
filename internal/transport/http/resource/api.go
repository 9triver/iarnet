package resource

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/domain/audit"
	audittypes "github.com/9triver/iarnet/internal/domain/audit/types"
	"github.com/9triver/iarnet/internal/domain/resource"
	"github.com/9triver/iarnet/internal/domain/resource/discovery"
	"github.com/9triver/iarnet/internal/domain/resource/logger"
	"github.com/9triver/iarnet/internal/domain/resource/provider"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	httpauth "github.com/9triver/iarnet/internal/transport/http/util/auth"
	"github.com/9triver/iarnet/internal/transport/http/util/response"
	"github.com/9triver/iarnet/internal/util"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func RegisterRoutes(router *mux.Router, resMgr *resource.Manager, cfg *config.Config, discoveryService discovery.Service, auditMgr *audit.Manager) {
	api := NewAPI(resMgr, cfg, discoveryService, auditMgr)
	router.HandleFunc("/resource/capacity", api.handleGetResourceCapacity).Methods("GET")
	router.HandleFunc("/resource/node/info", api.handleGetNodeInfo).Methods("GET")
	router.HandleFunc("/resource/provider", api.handleGetResourceProviders).Methods("GET")
	// 使用 {id:.*} 模式以支持包含点号的 ID（如 fake.provider.xxxxx）
	router.HandleFunc("/resource/provider/{id}/info", api.handleGetResourceProviderInfo).Methods("GET")
	router.HandleFunc("/resource/provider/{id}/capacity", api.handleGetResourceProviderCapacity).Methods("GET")
	router.HandleFunc("/resource/provider/{id}/usage", api.handleGetResourceProviderUsage).Methods("GET")
	router.HandleFunc("/resource/provider/test", api.handleTestResourceProvider).Methods("POST")
	router.HandleFunc("/resource/provider", api.handleRegisterResourceProvider).Methods("POST")
	router.HandleFunc("/resource/provider/batch", api.handleBatchRegisterResourceProvider).Methods("POST")
	// 注意：PUT 和 DELETE 路由需要在 /resource/provider/test 和 /resource/provider/batch 之后注册
	// 以避免路由冲突（因为 {id} 会匹配任何字符串）
	router.HandleFunc("/resource/provider/{id}", api.handleUpdateResourceProvider).Methods("PUT")
	router.HandleFunc("/resource/provider/{id}", api.handleUnregisterResourceProvider).Methods("DELETE")

	// Discovery 相关路由
	router.HandleFunc("/resource/discovery/nodes", api.handleGetDiscoveredNodes).Methods("GET")

	router.HandleFunc("/resource/components/{id}/logs", api.handleGetComponentLogs).Methods("GET")
}

// handleGetDiscoveredNodes 获取通过 gossip 发现的节点列表
func (api *API) handleGetDiscoveredNodes(w http.ResponseWriter, r *http.Request) {
	if api.discoveryService == nil {
		// Discovery 服务未启用，返回空列表
		resp := GetDiscoveredNodesResponse{
			Nodes: []DiscoveredNodeItem{},
			Total: 0,
		}
		response.Success(resp).WriteJSON(w)
		return
	}

	// 获取已知节点
	knownNodes := api.discoveryService.GetKnownNodes()
	items := make([]DiscoveredNodeItem, 0, len(knownNodes))

	for _, node := range knownNodes {
		item := DiscoveredNodeItem{
			NodeID:   node.NodeID,
			NodeName: node.NodeName,
			Address:  node.Address,
			DomainID: node.DomainID,
			Status:   string(node.Status),
			LastSeen: node.LastSeen.Format(time.RFC3339),
		}

		// 转换资源容量
		if node.ResourceCapacity != nil {
			if node.ResourceCapacity.Total != nil {
				item.CPU = &ResourceUsage{
					Total:     node.ResourceCapacity.Total.CPU,
					Used:      0, // 如果 Used 存在则使用，否则计算
					Available: 0,
				}
				if node.ResourceCapacity.Used != nil {
					item.CPU.Used = node.ResourceCapacity.Used.CPU
				}
				if node.ResourceCapacity.Available != nil {
					item.CPU.Available = node.ResourceCapacity.Available.CPU
					// 如果 Used 不存在，通过 Total - Available 计算
					if node.ResourceCapacity.Used == nil {
						item.CPU.Used = item.CPU.Total - item.CPU.Available
					}
				}
			}

			if node.ResourceCapacity.Total != nil {
				item.Memory = &ResourceUsage{
					Total:     node.ResourceCapacity.Total.Memory,
					Used:      0,
					Available: 0,
				}
				if node.ResourceCapacity.Used != nil {
					item.Memory.Used = node.ResourceCapacity.Used.Memory
				}
				if node.ResourceCapacity.Available != nil {
					item.Memory.Available = node.ResourceCapacity.Available.Memory
					if node.ResourceCapacity.Used == nil {
						item.Memory.Used = item.Memory.Total - item.Memory.Available
					}
				}
			}

			if node.ResourceCapacity.Total != nil {
				item.GPU = &ResourceUsage{
					Total:     node.ResourceCapacity.Total.GPU,
					Used:      0,
					Available: 0,
				}
				if node.ResourceCapacity.Used != nil {
					item.GPU.Used = node.ResourceCapacity.Used.GPU
				}
				if node.ResourceCapacity.Available != nil {
					item.GPU.Available = node.ResourceCapacity.Available.GPU
					if node.ResourceCapacity.Used == nil {
						item.GPU.Used = item.GPU.Total - item.GPU.Available
					}
				}
			}
		}

		// 转换资源标签
		if node.ResourceTags != nil {
			item.ResourceTags = &ResourceTagsInfo{
				CPU:    node.ResourceTags.CPU,
				GPU:    node.ResourceTags.GPU,
				Memory: node.ResourceTags.Memory,
				Camera: node.ResourceTags.Camera,
			}
		}

		items = append(items, item)
	}

	resp := GetDiscoveredNodesResponse{
		Nodes: items,
		Total: len(items),
	}
	response.Success(resp).WriteJSON(w)
}

// GetDiscoveredNodesResponse 获取发现的节点列表响应
type GetDiscoveredNodesResponse struct {
	Nodes []DiscoveredNodeItem `json:"nodes"`
	Total int                  `json:"total"`
}

// DiscoveredNodeItem 发现的节点项
type DiscoveredNodeItem struct {
	NodeID       string            `json:"node_id"`
	NodeName     string            `json:"node_name"`
	Address      string            `json:"address"`
	DomainID     string            `json:"domain_id"`
	Status       string            `json:"status"` // online/offline/error
	CPU          *ResourceUsage    `json:"cpu,omitempty"`
	Memory       *ResourceUsage    `json:"memory,omitempty"`
	GPU          *ResourceUsage    `json:"gpu,omitempty"`
	ResourceTags *ResourceTagsInfo `json:"resource_tags,omitempty"`
	LastSeen     string            `json:"last_seen"` // RFC3339 格式
}

// GetNodeInfoResponse 返回当前节点与域信息
type GetNodeInfoResponse struct {
	NodeID     string `json:"node_id"`
	NodeName   string `json:"node_name"`
	DomainID   string `json:"domain_id"`
	DomainName string `json:"domain_name"`
}

// ResourceUsage 资源使用情况
type ResourceUsage struct {
	Total     int64 `json:"total"`
	Used      int64 `json:"used"`
	Available int64 `json:"available"`
}

// ResourceTagsInfo 资源标签信息
type ResourceTagsInfo struct {
	CPU    bool `json:"cpu"`
	GPU    bool `json:"gpu"`
	Memory bool `json:"memory"`
	Camera bool `json:"camera"`
}

type API struct {
	resMgr           *resource.Manager
	cfg              *config.Config
	discoveryService discovery.Service
	auditMgr         *audit.Manager
}

func NewAPI(resMgr *resource.Manager, cfg *config.Config, discoveryService discovery.Service, auditMgr *audit.Manager) *API {
	return &API{
		resMgr:           resMgr,
		cfg:              cfg,
		discoveryService: discoveryService,
		auditMgr:         auditMgr,
	}
}

// recordOperation 记录操作日志的辅助函数
func (api *API) recordOperation(r *http.Request, operation audittypes.OperationType, resourceType, resourceID, action string, before, after map[string]interface{}) {
	if api.auditMgr == nil {
		return
	}

	// 从 context 中获取用户信息（由认证中间件注入）
	user := httpauth.GetUsernameFromContext(r.Context())
	if user == "" {
		user = "unknown"
	}

	// 获取IP地址
	ip := getClientIP(r)

	// 生成操作日志ID
	logID := util.GenIDWith("op.")

	opLog := &audittypes.OperationLog{
		ID:           logID,
		User:         user,
		Operation:    operation,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       action,
		Before:       before,
		After:        after,
		Timestamp:    time.Now(),
		IP:           ip,
	}

	// 异步记录操作日志，避免影响主流程
	// 使用独立的 context，避免 HTTP 请求 context 被取消导致记录失败
	go func() {
		ctx := context.Background()
		if err := api.auditMgr.RecordOperation(ctx, opLog); err != nil {
			logrus.Errorf("Failed to record operation log: %v", err)
		}
	}()
}

// getClientIP 获取客户端IP地址
func getClientIP(r *http.Request) string {
	// 检查 X-Forwarded-For 头（代理/负载均衡器）
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// 检查 X-Real-IP 头
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// 直接从 RemoteAddr 获取
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func (api *API) handleGetResourceCapacity(w http.ResponseWriter, r *http.Request) {
	capacity, err := api.getAggregatedCapacity(r.Context())
	if err != nil {
		response.InternalError(err.Error()).WriteJSON(w)
		return
	}
	response.Success((&GetResourceCapacityResponse{}).FromCapacity(capacity)).WriteJSON(w)
}

// handleGetNodeInfo 返回当前节点及域信息
func (api *API) handleGetNodeInfo(w http.ResponseWriter, r *http.Request) {
	if api.resMgr == nil {
		response.InternalError("resource manager not initialized").WriteJSON(w)
		return
	}

	resp := GetNodeInfoResponse{
		NodeID:     api.resMgr.GetNodeID(),
		NodeName:   api.resMgr.GetNodeName(),
		DomainID:   api.resMgr.GetDomainID(),
		DomainName: api.resMgr.GetDomainName(),
	}

	response.Success(resp).WriteJSON(w)
}

func (api *API) handleGetResourceProviders(w http.ResponseWriter, r *http.Request) {
	// 使用 GetAllProvidersIncludingFake 以包含 fake provider（前端需要显示它们）
	providers := api.resMgr.GetAllProvidersIncludingFake()
	items := make([]ProviderItem, 0, len(providers))
	for _, p := range providers {
		item := (&ProviderItem{}).FromProvider(p)
		items = append(items, *item)
		// 记录 fake provider 的 ID 用于调试
		if p.IsFake() {
			logrus.Debugf("Including fake provider in list: ID=%s, Name=%s", item.ID, item.Name)
		}
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

	// 注册 provider
	p, err := api.resMgr.RegisterProvider(req.Name, req.Host, req.Port)
	if err != nil {
		logrus.Errorf("Failed to register provider: %v", err)
		response.InternalError("failed to register provider: " + err.Error()).WriteJSON(w)
		return
	}

	// 记录操作日志
	after := map[string]interface{}{
		"id":      p.GetID(),
		"name":    p.GetName(),
		"host":    req.Host,
		"port":    req.Port,
		"address": fmt.Sprintf("%s:%d", req.Host, req.Port),
	}
	api.recordOperation(r, audittypes.OperationTypeRegisterResource, "resource_provider", p.GetID(),
		fmt.Sprintf("接入资源提供者: %s (%s:%d)", req.Name, req.Host, req.Port), nil, after)

	resp := RegisterResourceProviderResponse{
		ID:   p.GetID(),
		Name: p.GetName(),
	}
	response.Created(resp).WriteJSON(w)
}

// handleBatchRegisterResourceProvider 批量注册资源提供者
func (api *API) handleBatchRegisterResourceProvider(w http.ResponseWriter, r *http.Request) {
	// 解析 multipart/form-data
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB max
		response.BadRequest("failed to parse multipart form: " + err.Error()).WriteJSON(w)
		return
	}

	// 获取上传的文件
	file, header, err := r.FormFile("file")
	if err != nil {
		response.BadRequest("file not found in request: " + err.Error()).WriteJSON(w)
		return
	}
	defer file.Close()

	// 验证文件类型
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".csv") {
		response.BadRequest("file must be a CSV file").WriteJSON(w)
		return
	}

	// 解析 CSV 文件
	providers, parseErrors := parseCSVFile(file)
	if len(parseErrors) > 0 {
		// 如果有解析错误，返回错误信息
		resp := BatchRegisterResourceProviderResponse{
			Total:   0,
			Success: 0,
			Failed:  0,
			Results: nil,
			Errors:  parseErrors,
		}
		badResp := response.BadRequest("CSV parsing errors")
		badResp.Data = resp
		badResp.WriteJSON(w)
		return
	}

	if len(providers) == 0 {
		response.BadRequest("CSV file is empty or contains no valid data").WriteJSON(w)
		return
	}

	// 批量注册
	results := make([]BatchRegisterResult, 0, len(providers))
	successCount := 0
	failedCount := 0

	for _, req := range providers {
		result := BatchRegisterResult{
			Name:    req.Name,
			Address: fmt.Sprintf("%s:%d", req.Host, req.Port),
		}

		// 注册 provider（注意：Manager.RegisterProvider 没有 context 参数）
		p, err := api.resMgr.RegisterProvider(req.Name, req.Host, req.Port)
		if err != nil {
			logrus.Errorf("Failed to register provider %s (%s:%d): %v", req.Name, req.Host, req.Port, err)
			result.Success = false
			result.Message = err.Error()
			failedCount++
		} else {
			result.Success = true
			result.Message = "注册成功"
			result.ProviderID = p.GetID()
			successCount++
		}

		results = append(results, result)
	}

	resp := BatchRegisterResourceProviderResponse{
		Total:   len(providers),
		Success: successCount,
		Failed:  failedCount,
		Results: results,
		Errors:  nil,
	}

	response.Success(resp).WriteJSON(w)
}

// parseCSVFile 解析 CSV 文件，首行是数据而不是表头
func parseCSVFile(file io.Reader) ([]RegisterResourceProviderRequest, []string) {
	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	reader.LazyQuotes = true

	var providers []RegisterResourceProviderRequest
	var errors []string

	lineNum := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errors = append(errors, fmt.Sprintf("第 %d 行 CSV 解析错误: %v", lineNum+1, err))
			continue
		}

		lineNum++

		// 跳过空行
		if len(record) == 0 || (len(record) == 1 && strings.TrimSpace(record[0]) == "") {
			continue
		}

		// CSV 格式：节点名称,地址:端口
		// 注意：首行就是数据，不是表头
		if len(record) < 2 {
			errors = append(errors, fmt.Sprintf("第 %d 行列数不足，需要至少 2 列（节点名称、地址:端口）", lineNum))
			continue
		}

		name := strings.TrimSpace(record[0])
		addressStr := strings.TrimSpace(record[1])

		if name == "" {
			errors = append(errors, fmt.Sprintf("第 %d 行节点名称为空", lineNum))
			continue
		}

		if addressStr == "" {
			errors = append(errors, fmt.Sprintf("第 %d 行地址为空", lineNum))
			continue
		}

		// 解析地址:端口格式
		addressParts := strings.Split(addressStr, ":")
		if len(addressParts) != 2 {
			errors = append(errors, fmt.Sprintf("第 %d 行地址格式错误: %s（格式应为 主机地址:端口，例如：192.168.1.100:50051）", lineNum, addressStr))
			continue
		}

		host := strings.TrimSpace(addressParts[0])
		portStr := strings.TrimSpace(addressParts[1])

		if host == "" {
			errors = append(errors, fmt.Sprintf("第 %d 行主机地址为空", lineNum))
			continue
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			errors = append(errors, fmt.Sprintf("第 %d 行端口无效: %s（端口必须是数字）", lineNum, portStr))
			continue
		}

		if port <= 0 || port > 65535 {
			errors = append(errors, fmt.Sprintf("第 %d 行端口无效: %d（端口必须在 1-65535 之间）", lineNum, port))
			continue
		}

		providers = append(providers, RegisterResourceProviderRequest{
			Name: name,
			Host: host,
			Port: port,
		})
	}

	return providers, errors
}

func (api *API) handleUpdateResourceProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["id"]
	if providerID == "" {
		response.BadRequest("provider id is required").WriteJSON(w)
		return
	}

	// URL 解码（处理 URL 编码的 ID，如包含点号的 fake.provider.xxxxx）
	if decodedID, err := url.QueryUnescape(providerID); err == nil {
		providerID = decodedID
	}

	// 解析请求体
	req := UpdateResourceProviderRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest("invalid request body: " + err.Error()).WriteJSON(w)
		return
	}

	// 使用 GetProviderIncludingFake 以支持 fake provider
	provider := api.resMgr.GetProviderIncludingFake(providerID)
	if provider == nil {
		logrus.Warnf("Provider not found for update: %s", providerID)
		// 列出所有 provider ID 用于调试
		allProviders := api.resMgr.GetAllProvidersIncludingFake()
		providerIDs := make([]string, 0, len(allProviders))
		for _, p := range allProviders {
			providerIDs = append(providerIDs, fmt.Sprintf("%s (name: %s, fake: %v)", p.GetID(), p.GetName(), p.IsFake()))
		}
		logrus.Debugf("Available providers: %v", providerIDs)
		response.NotFound("provider not found").WriteJSON(w)
		return
	}

	// 记录操作前的状态
	before := map[string]interface{}{
		"id":   providerID,
		"name": provider.GetName(),
	}

	// 检查是否有需要更新的字段
	hasUpdates := false
	updatedName := provider.GetName()

	// 更新名称（目前唯一支持的字段）
	if req.Name != nil {
		if *req.Name == "" {
			response.BadRequest("provider name cannot be empty").WriteJSON(w)
			return
		}
		provider.SetName(*req.Name)
		updatedName = *req.Name
		hasUpdates = true
	}

	// 对于 fake provider，只在内存中更新，不持久化
	if provider.IsFake() {
		logrus.Debugf("Updating fake provider %s in memory only (no persistence)", providerID)
		// 构建响应（fake provider 不需要持久化）
		resp := UpdateResourceProviderResponse{
			ID:      providerID,
			Name:    updatedName,
			Message: "Fake provider updated successfully (in memory only)",
		}
		response.Success(resp).WriteJSON(w)
		return
	}

	// 未来可以在这里添加其他字段的更新逻辑
	// if req.Host != nil {
	//     provider.UpdateHost(*req.Host)
	//     hasUpdates = true
	// }
	// if req.Port != nil {
	//     provider.UpdatePort(*req.Port)
	//     hasUpdates = true
	// }

	// 如果没有提供任何更新字段，返回错误
	if !hasUpdates {
		response.BadRequest("at least one field must be provided for update").WriteJSON(w)
		return
	}

	// 记录操作后的状态
	after := map[string]interface{}{
		"id":   providerID,
		"name": updatedName,
	}

	// 记录操作日志
	api.recordOperation(r, audittypes.OperationTypeUpdateResource, "resource_provider", providerID,
		fmt.Sprintf("更新资源提供者: %s -> %s", before["name"], updatedName), before, after)

	// 构建响应
	resp := UpdateResourceProviderResponse{
		ID:      providerID,
		Name:    updatedName,
		Message: "Provider updated successfully",
	}

	response.Success(resp).WriteJSON(w)
}

func (api *API) handleUnregisterResourceProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["id"]
	if providerID == "" {
		response.BadRequest("provider id is required").WriteJSON(w)
		return
	}

	// URL 解码（处理 URL 编码的 ID，如包含点号的 fake.provider.xxxxx）
	if decodedID, err := url.QueryUnescape(providerID); err == nil {
		providerID = decodedID
	}

	// 使用 GetProviderIncludingFake 以支持 fake provider
	provider := api.resMgr.GetProviderIncludingFake(providerID)
	if provider == nil {
		response.NotFound("provider not found").WriteJSON(w)
		return
	}

	before := map[string]interface{}{
		"id":   providerID,
		"name": provider.GetName(),
	}

	// 对于 fake provider，只在内存中删除，不持久化
	if provider.IsFake() {
		logrus.Debugf("Deleting fake provider %s from memory only (no persistence)", providerID)
		// 直接从 provider manager 中移除（不通过 service，避免持久化操作）
		providerManager := api.resMgr.GetProviderManager()
		if providerManager != nil {
			providerManager.Remove(providerID)
			provider.Disconnect()
		}
		// 记录操作日志
		api.recordOperation(r, audittypes.OperationTypeDeleteResource, "resource_provider", providerID,
			fmt.Sprintf("删除资源提供者（fake）: %s", before["name"]), before, nil)
		resp := UnregisterResourceProviderResponse{
			ID:      providerID,
			Message: "Fake provider deleted successfully (from memory only)",
		}
		response.Success(resp).WriteJSON(w)
		return
	}

	// 注销普通 provider（会持久化）
	if err := api.resMgr.UnregisterProvider(providerID); err != nil {
		logrus.Errorf("Failed to unregister provider %s: %v", providerID, err)
		response.NotFound("provider not found: " + err.Error()).WriteJSON(w)
		return
	}

	// 记录操作日志
	api.recordOperation(r, audittypes.OperationTypeDeleteResource, "resource_provider", providerID,
		fmt.Sprintf("删除资源提供者: %s", before["name"]), before, nil)

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

	// URL 解码（处理 URL 编码的 ID，如包含点号的 fake.provider.xxxxx）
	if decodedID, err := url.QueryUnescape(providerID); err == nil {
		providerID = decodedID
	}

	// 使用 GetProviderIncludingFake 以支持 fake provider
	provider := api.resMgr.GetProviderIncludingFake(providerID)
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

	// URL 解码（处理 URL 编码的 ID，如包含点号的 fake.provider.xxxxx）
	if decodedID, err := url.QueryUnescape(providerID); err == nil {
		providerID = decodedID
	}

	// 使用 GetProviderIncludingFake 以支持 fake provider
	provider := api.resMgr.GetProviderIncludingFake(providerID)
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

func (api *API) handleGetResourceProviderUsage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["id"]
	if providerID == "" {
		response.BadRequest("provider id is required").WriteJSON(w)
		return
	}

	// URL 解码（处理 URL 编码的 ID，如包含点号的 fake.provider.xxxxx）
	if decodedID, err := url.QueryUnescape(providerID); err == nil {
		providerID = decodedID
	}

	// 使用 GetProviderIncludingFake 以支持 fake provider
	provider := api.resMgr.GetProviderIncludingFake(providerID)
	if provider == nil {
		response.NotFound("provider not found").WriteJSON(w)
		return
	}

	usage, err := provider.GetRealTimeUsage(r.Context())
	if err != nil {
		response.InternalError("failed to get provider usage: " + err.Error()).WriteJSON(w)
		return
	}

	resp := (&GetResourceProviderUsageResponse{}).FromUsage(usage)
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
	testProvider := provider.NewProvider(req.Name, req.Host, req.Port, &provider.EnvVariables{
		IarnetHost: api.cfg.Host,
		ZMQPort:    api.cfg.Transport.ZMQ.Port,
		StorePort:  api.cfg.Transport.RPC.Store.Port,
	})

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

// getAggregatedCapacity 聚合所有 provider 的资源容量（包括 fake provider）
// 使用实时使用量（GetRealTimeUsage）而不是已分配资源（GetCapacity.Used）
func (api *API) getAggregatedCapacity(ctx context.Context) (*types.Capacity, error) {
	// 使用 GetAllProvidersIncludingFake 以包含 fake provider（前端需要显示总容量）
	providers := api.resMgr.GetAllProvidersIncludingFake()
	if len(providers) == 0 {
		return &types.Capacity{
			Total:     &types.Info{CPU: 0, Memory: 0, GPU: 0},
			Used:      &types.Info{CPU: 0, Memory: 0, GPU: 0},
			Available: &types.Info{CPU: 0, Memory: 0, GPU: 0},
		}, nil
	}

	var totalCPU, totalMemory, totalGPU int64
	var usedCPU, usedMemory, usedGPU int64

	for _, p := range providers {
		if p.GetStatus() != types.ProviderStatusConnected {
			continue
		}

		// 获取容量（总量）
		capacity, err := p.GetCapacity(ctx)
		if err != nil {
			// 跳过获取容量失败的 provider
			logrus.Debugf("Failed to get capacity from provider %s: %v", p.GetID(), err)
			continue
		}

		// 获取实时使用量
		usage, err := p.GetRealTimeUsage(ctx)
		if err != nil {
			// 如果获取实时使用量失败，回退到使用已分配资源
			logrus.Debugf("Failed to get real-time usage from provider %s: %v, falling back to allocated resources", p.GetID(), err)
			if capacity.Used != nil {
				usage = capacity.Used
			} else {
				usage = &types.Info{CPU: 0, Memory: 0, GPU: 0}
			}
		}

		// 聚合总量
		if capacity.Total != nil {
			totalCPU += capacity.Total.CPU
			totalMemory += capacity.Total.Memory
			totalGPU += capacity.Total.GPU
		}

		// 聚合实时使用量
		if usage != nil {
			usedCPU += usage.CPU
			usedMemory += usage.Memory
			usedGPU += usage.GPU
		}
	}

	// 计算可用资源
	availableCPU := totalCPU - usedCPU
	if availableCPU < 0 {
		availableCPU = 0
	}
	availableMemory := totalMemory - usedMemory
	if availableMemory < 0 {
		availableMemory = 0
	}
	availableGPU := totalGPU - usedGPU
	if availableGPU < 0 {
		availableGPU = 0
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

func (api *API) handleGetComponentLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentID := vars["id"]
	if componentID == "" {
		response.BadRequest("component id is required").WriteJSON(w)
		return
	}

	query := r.URL.Query()

	limit, err := parsePositiveInt(query.Get("limit"), 100)
	if err != nil {
		response.BadRequest("invalid limit: " + err.Error()).WriteJSON(w)
		return
	}

	offset, err := parseNonNegativeInt(query.Get("offset"), 0)
	if err != nil {
		response.BadRequest("invalid offset: " + err.Error()).WriteJSON(w)
		return
	}

	opts := &logger.QueryOptions{
		Limit:  limit,
		Offset: offset,
	}

	if levelParam := strings.TrimSpace(query.Get("level")); levelParam != "" && strings.ToLower(levelParam) != "all" {
		level, err := parseLogLevel(levelParam)
		if err != nil {
			response.BadRequest(err.Error()).WriteJSON(w)
			return
		}
		opts.Level = level
	}

	if startParam := strings.TrimSpace(query.Get("start_time")); startParam != "" {
		startTime, err := time.Parse(time.RFC3339, startParam)
		if err != nil {
			response.BadRequest("invalid start_time, must be RFC3339").WriteJSON(w)
			return
		}
		opts.StartTime = &startTime
	}

	if endParam := strings.TrimSpace(query.Get("end_time")); endParam != "" {
		endTime, err := time.Parse(time.RFC3339, endParam)
		if err != nil {
			response.BadRequest("invalid end_time, must be RFC3339").WriteJSON(w)
			return
		}
		opts.EndTime = &endTime
	}

	result, err := api.resMgr.GetLogs(r.Context(), componentID, opts)
	if err != nil {
		logrus.Errorf("Failed to get application logs: %v", err)
		response.InternalError("failed to get application logs: " + err.Error()).WriteJSON(w)
		return
	}

	resp := BuildGetComponentLogsResponse(componentID, result)
	response.Success(resp).WriteJSON(w)
}

func parsePositiveInt(raw string, defaultVal int) (int, error) {
	if raw == "" {
		return defaultVal, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("must be positive integer")
	}
	return value, nil
}

func parseNonNegativeInt(raw string, defaultVal int) (int, error) {
	if raw == "" {
		return defaultVal, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("must be non-negative integer")
	}
	return value, nil
}

func parseLogLevel(level string) (logger.LogLevel, error) {
	switch strings.ToLower(level) {
	case "trace":
		return logger.LogLevelTrace, nil
	case "debug":
		return logger.LogLevelDebug, nil
	case "info":
		return logger.LogLevelInfo, nil
	case "warn", "warning":
		return logger.LogLevelWarn, nil
	case "error":
		return logger.LogLevelError, nil
	case "fatal":
		return logger.LogLevelFatal, nil
	case "panic":
		return logger.LogLevelPanic, nil
	default:
		return "", fmt.Errorf("invalid level: %s", level)
	}
}

type GetComponentLogsResponse struct {
	ComponentID string         `json:"component_id"`
	Logs        []ComponentLog `json:"logs"`
	Total       int            `json:"total"`
	HasMore     bool           `json:"has_more"`
}

type ComponentLog struct {
	Timestamp time.Time           `json:"timestamp"`
	Level     string              `json:"level"`
	Message   string              `json:"message"`
	Fields    []ComponentLogField `json:"fields,omitempty"`
	Caller    *ComponentLogCaller `json:"caller,omitempty"`
}

type ComponentLogField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ComponentLogCaller struct {
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Function string `json:"function,omitempty"`
}

func BuildGetComponentLogsResponse(componentID string, result *logger.QueryResult) GetComponentLogsResponse {
	logs := make([]ComponentLog, len(result.Entries))
	for i, entry := range result.Entries {
		fields := make([]ComponentLogField, len(entry.Fields))
		for idx, field := range entry.Fields {
			fields[idx] = ComponentLogField{
				Key:   field.Key,
				Value: field.Value,
			}
		}

		var caller *ComponentLogCaller
		if entry.Caller != nil {
			caller = &ComponentLogCaller{
				File:     entry.Caller.File,
				Line:     entry.Caller.Line,
				Function: entry.Caller.Function,
			}
		}

		logs[i] = ComponentLog{
			Timestamp: entry.Timestamp,
			Level:     string(entry.Level),
			Message:   entry.Message,
			Fields:    fields,
			Caller:    caller,
		}
	}
	return GetComponentLogsResponse{
		ComponentID: componentID,
		Logs:        logs,
		Total:       result.Total,
		HasMore:     result.HasMore,
	}
}
