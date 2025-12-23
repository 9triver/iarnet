package application

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/9triver/iarnet/internal/domain/application"
	applogger "github.com/9triver/iarnet/internal/domain/application/logger"
	apptypes "github.com/9triver/iarnet/internal/domain/application/types"
	"github.com/9triver/iarnet/internal/transport/http/util/response"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func RegisterRoutes(router *mux.Router, am *application.Manager) {
	api := NewAPI(am)
	router.HandleFunc("/application/stats", api.handleGetApplicationStats).Methods("GET")
	router.HandleFunc("/application/runner-environments", api.handleGetRunnerEnvironments).Methods("GET")
	router.HandleFunc("/application/apps", api.handleGetApplicationList).Methods("GET")
	router.HandleFunc("/application/apps", api.handleCreateApplication).Methods("POST")
	router.HandleFunc("/application/apps/{id}", api.handleGetApplicationById).Methods("GET")
	router.HandleFunc("/application/apps/{id}", api.handleUpdateApplication).Methods("PUT")
	router.HandleFunc("/application/apps/{id}", api.handleDeleteApplication).Methods("DELETE")
	router.HandleFunc("/application/apps/{id}/run", api.handleRunApplication).Methods("POST")
	router.HandleFunc("/application/apps/{id}/stop", api.handleStopApplication).Methods("POST")
	// 文件管理相关路由
	router.HandleFunc("/application/apps/{id}/files", api.handleGetFileTree).Methods("GET")
	router.HandleFunc("/application/apps/{id}/files/content", api.handleGetFileContent).Methods("GET")
	router.HandleFunc("/application/apps/{id}/files/content", api.handleSaveFileContent).Methods("PUT")
	router.HandleFunc("/application/apps/{id}/files", api.handleCreateFile).Methods("POST")
	router.HandleFunc("/application/apps/{id}/files", api.handleDeleteFile).Methods("DELETE")
	router.HandleFunc("/application/apps/{id}/directories", api.handleCreateDirectory).Methods("POST")
	router.HandleFunc("/application/apps/{id}/directories", api.handleDeleteDirectory).Methods("DELETE")
	// DAG管理相关路由
	router.HandleFunc("/application/apps/{id}/dag", api.handleGetApplicationDAG).Methods("GET")
	// Actor管理相关路由
	router.HandleFunc("/application/apps/{id}/actors", api.handleGetApplicationActors).Methods("GET")
	// 执行结果相关路由
	router.HandleFunc("/application/apps/{id}/execution-result", api.handleGetExecutionResult).Methods("GET")
	router.HandleFunc("/application/apps/{id}/logs", api.handleGetApplicationLogs).Methods("GET")
}

type API struct {
	am *application.Manager
}

func NewAPI(am *application.Manager) *API {
	return &API{
		am: am,
	}
}

func (api *API) handleGetApplicationDAG(w http.ResponseWriter, r *http.Request) {
	req := GetApplicationDAGRequest{
		AppID:     mux.Vars(r)["id"],
		SessionID: r.URL.Query().Get("session_id"),
	}
	if req.AppID == "" {
		response.BadRequest("application id is required").WriteJSON(w)
		return
	}

	dag, err := api.am.GetApplicationDAGs(r.Context(), req.AppID)
	if err != nil {
		logrus.Errorf("Failed to get application DAGs: %v", err)
		response.InternalError("failed to get application DAGs: " + err.Error()).WriteJSON(w)
		return
	}

	resp := BuildGetApplicationDAGResponse(dag, req.SessionID)
	response.Success(resp).WriteJSON(w)
}

func (api *API) handleGetApplicationList(w http.ResponseWriter, r *http.Request) {
	apps, err := api.am.GetAllAppMetadata(r.Context())
	if err != nil {
		logrus.Errorf("Failed to get all app metadata: %v", err)
		response.InternalError("failed to get applications: " + err.Error()).WriteJSON(w)
		return
	}
	response.Success((&GetApplicationListResponse{}).FromAppMetadataArray(apps)).WriteJSON(w)
}

func (api *API) handleCreateApplication(w http.ResponseWriter, r *http.Request) {
	req := CreateApplicationRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logrus.Errorf("Failed to decode create application request: %v", err)
		response.BadRequest("invalid request body: " + err.Error()).WriteJSON(w)
		return
	}

	// 验证必填字段
	if req.Name == "" {
		response.BadRequest("application name is required").WriteJSON(w)
		return
	}
	if req.GitURL == "" {
		response.BadRequest("git_url is required for git import").WriteJSON(w)
		return
	}
	if req.RunnerEnv == "" {
		response.BadRequest("runner_env is required").WriteJSON(w)
		return
	}

	logrus.Infof("Creating application: name=%s, git_url=%s", req.Name, req.GitURL)

	// 转换为领域层的 AppMetadata
	metadata := req.ToAppMetadata()

	// 调用 manager 的 CreateApplication 方法，封装了创建元数据和异步克隆的逻辑
	appID, err := api.am.CreateApplication(r.Context(), metadata)
	if err != nil {
		logrus.Errorf("Failed to create application: %v", err)
		response.InternalError("failed to create application: " + err.Error()).WriteJSON(w)
		return
	}

	logrus.Infof("Application created successfully: id=%s (cloning in background)", appID)
	resp := CreateApplicationResponse{
		ID: string(appID),
	}
	response.Created(resp).WriteJSON(w)
}

func (api *API) handleGetApplicationById(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["id"]
	if appID == "" {
		response.BadRequest("application id is required").WriteJSON(w)
		return
	}

	metadata, err := api.am.GetAppMetadata(r.Context(), appID)
	if err != nil {
		logrus.Warnf("Application not found: %s", appID)
		response.NotFound("application not found").WriteJSON(w)
		return
	}

	resp := FromAppMetadataToGetResponse(metadata)
	response.Success(resp).WriteJSON(w)
}

func (api *API) handleGetApplicationActors(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["id"]
	if appID == "" {
		response.BadRequest("application id is required").WriteJSON(w)
		return
	}

	actors, err := api.am.GetApplicationActors(r.Context(), appID)
	if err != nil {
		logrus.Errorf("Failed to get actors: %v", err)
		response.InternalError("failed to get actors: " + err.Error()).WriteJSON(w)
		return
	}

	// 转换 Actor 数据为 HTTP 响应格式
	resp := BuildGetApplicationActorsResponse(actors)
	response.Success(resp).WriteJSON(w)
}

func (api *API) handleUpdateApplication(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["id"]
	if appID == "" {
		response.BadRequest("application id is required").WriteJSON(w)
		return
	}

	req := UpdateApplicationRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logrus.Errorf("Failed to decode update application request: %v", err)
		response.BadRequest("invalid request body: " + err.Error()).WriteJSON(w)
		return
	}

	// 获取现有元数据
	metadata, err := api.am.GetAppMetadata(r.Context(), appID)
	if err != nil {
		logrus.Warnf("Application not found: %s", appID)
		response.NotFound("application not found").WriteJSON(w)
		return
	}

	// 更新字段
	if req.Name != nil {
		metadata.Name = *req.Name
	}
	if req.GitURL != nil {
		metadata.GitUrl = *req.GitURL
	}
	if req.Branch != nil {
		metadata.Branch = *req.Branch
	}
	if req.Description != nil {
		metadata.Description = *req.Description
	}
	if req.ExecuteCmd != nil {
		metadata.ExecuteCmd = *req.ExecuteCmd
	}
	if req.EnvInstallCmd != nil {
		metadata.EnvInstallCmd = *req.EnvInstallCmd
	}
	if req.RunnerEnv != nil {
		metadata.RunnerEnv = *req.RunnerEnv
	}

	if err := api.am.UpdateAppMetadata(r.Context(), appID, metadata); err != nil {
		logrus.Errorf("Failed to update app metadata: %v", err)
		response.InternalError("failed to update application: " + err.Error()).WriteJSON(w)
		return
	}

	logrus.Infof("Application updated successfully: id=%s", appID)
	response.Success(map[string]string{"message": "Application updated successfully"}).WriteJSON(w)
}

func (api *API) handleDeleteApplication(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["id"]
	if appID == "" {
		response.BadRequest("application id is required").WriteJSON(w)
		return
	}

	// 验证应用是否存在
	_, err := api.am.GetAppMetadata(r.Context(), appID)
	if err != nil {
		logrus.Warnf("Application not found: %s", appID)
		response.NotFound("application not found").WriteJSON(w)
		return
	}

	// 停止并移除 runner（如果存在）
	ctx := r.Context()
	if err := api.am.StopRunner(ctx, appID); err != nil {
		logrus.Warnf("Failed to stop runner for app %s: %v", appID, err)
		// 继续删除，不因停止失败而中断
	}
	if err := api.am.RemoveRunner(ctx, appID); err != nil {
		logrus.Warnf("Failed to remove runner for app %s: %v", appID, err)
		// 继续删除，不因移除失败而中断
	}

	// 清理工作目录
	if err := api.am.CleanWorkDir(ctx, appID); err != nil {
		logrus.Warnf("Failed to clean work dir for app %s: %v", appID, err)
		// 继续删除，不因清理失败而中断
	}

	// 删除应用元数据
	if err := api.am.RemoveAppMetadata(ctx, appID); err != nil {
		logrus.Errorf("Failed to remove app metadata: %v", err)
		response.InternalError("failed to delete application: " + err.Error()).WriteJSON(w)
		return
	}

	logrus.Infof("Application deleted successfully: id=%s", appID)
	response.Success(map[string]string{
		"message": "Application deleted successfully",
		"app_id":  appID,
	}).WriteJSON(w)
}

func (api *API) handleRunApplication(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["id"]
	if appID == "" {
		response.BadRequest("application id is required").WriteJSON(w)
		return
	}

	ctx := r.Context()

	// 调用 manager 的 RunApplication 方法，封装了启动 runner 的完整逻辑
	if err := api.am.RunApplication(ctx, appID); err != nil {
		logrus.Errorf("Failed to run application %s: %v", appID, err)
		// 检查是否是应用不存在的错误
		if strings.Contains(err.Error(), "application not found") {
			response.NotFound("application not found").WriteJSON(w)
		} else {
			response.InternalError("failed to run application: " + err.Error()).WriteJSON(w)
		}
		return
	}

	// 获取更新后的应用信息
	updatedMetadata, _ := api.am.GetAppMetadata(ctx, appID)
	resp := FromAppMetadataToGetResponse(updatedMetadata)
	response.Success(resp).WriteJSON(w)
}

func (api *API) handleStopApplication(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["id"]
	if appID == "" {
		response.BadRequest("application id is required").WriteJSON(w)
		return
	}

	ctx := r.Context()

	// 停止 runner
	if err := api.am.StopRunner(ctx, appID); err != nil {
		logrus.Errorf("Failed to stop runner: %v", err)
		response.InternalError("failed to stop application: " + err.Error()).WriteJSON(w)
		return
	}

	// 清理应用的所有资源（components、controller 状态等）
	// 注意：这里会清理 controller 的状态，但不会移除 controller 本身
	if err := api.am.CleanupApplicationResources(ctx, appID); err != nil {
		logrus.Errorf("Failed to cleanup application resources: %v", err)
		// 不返回错误，因为 runner 已经停止，资源清理失败不应该影响停止操作
	}

	// 更新应用状态为已停止
	if err := api.am.UpdateAppStatus(ctx, appID, apptypes.AppStatusStopped); err != nil {
		logrus.Errorf("Failed to update application status to stopped: %v", err)
	}

	// 获取更新后的应用信息
	updatedMetadata, _ := api.am.GetAppMetadata(ctx, appID)
	resp := FromAppMetadataToGetResponse(updatedMetadata)
	response.Success(resp).WriteJSON(w)
}

func (api *API) handleGetApplicationStats(w http.ResponseWriter, r *http.Request) {
	apps, err := api.am.GetAllAppMetadata(r.Context())
	if err != nil {
		logrus.Errorf("Failed to get all app metadata for stats: %v", err)
		response.InternalError("failed to get application stats: " + err.Error()).WriteJSON(w)
		return
	}

	stats := ApplicationStats{
		Total:      len(apps),
		Running:    0,
		Stopped:    0,
		Undeployed: 0,
		Failed:     0,
	}

	for _, app := range apps {
		switch app.Status {
		case apptypes.AppStatusRunning:
			stats.Running++
		case apptypes.AppStatusStopped:
			stats.Stopped++
		case apptypes.AppStatusUndeployed:
			stats.Undeployed++
		case apptypes.AppStatusFailed:
			stats.Failed++
		}
	}

	response.Success(stats).WriteJSON(w)
}

func (api *API) handleGetRunnerEnvironments(w http.ResponseWriter, r *http.Request) {
	runnerEnvs := make([]string, 0)
	for name := range api.am.GetRunnerImages() {
		runnerEnvs = append(runnerEnvs, string(name))
	}

	resp := &GetRunnerEnvironmentsResponse{
		Environments: runnerEnvs,
	}

	response.Success(resp).WriteJSON(w)
}

func (api *API) handleGetApplicationLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["id"]
	if appID == "" {
		response.BadRequest("application id is required").WriteJSON(w)
		return
	}

	query := r.URL.Query()

	opts := &applogger.QueryOptions{}

	// 解析时间范围参数
	var hasTimeRange bool
	if startParam := strings.TrimSpace(query.Get("start_time")); startParam != "" {
		startTime, err := time.Parse(time.RFC3339, startParam)
		if err != nil {
			logrus.Debugf("Failed to parse start_time: %v, param: %s", err, startParam)
			response.BadRequest("invalid start_time, must be RFC3339").WriteJSON(w)
			return
		}
		// 转换为本地时间，因为数据库中的时间戳是本地时间
		startTimeLocal := startTime.Local()
		opts.StartTime = &startTimeLocal
		hasTimeRange = true
		logrus.Debugf("Parsed start_time: %v (UTC), %v (Local)", startTime.UTC(), startTimeLocal.Local())
	}

	if endParam := strings.TrimSpace(query.Get("end_time")); endParam != "" {
		endTime, err := time.Parse(time.RFC3339, endParam)
		if err != nil {
			logrus.Debugf("Failed to parse end_time: %v, param: %s", err, endParam)
			response.BadRequest("invalid end_time, must be RFC3339").WriteJSON(w)
			return
		}
		// 转换为本地时间，因为数据库中的时间戳是本地时间
		endTimeLocal := endTime.Local()
		opts.EndTime = &endTimeLocal
		hasTimeRange = true
		logrus.Debugf("Parsed end_time: %v (UTC), %v (Local)", endTime.UTC(), endTimeLocal.Local())
	}

	// 如果有时间范围，不解析 limit 和 offset，返回全部日志
	// 如果没有时间范围，使用分页查询
	if !hasTimeRange {
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

		opts.Limit = limit
		opts.Offset = offset
	} else {
		// 有时间范围时，设置 Limit 为 0 表示返回全部日志
		opts.Limit = 0
		opts.Offset = 0
	}

	if levelParam := strings.TrimSpace(query.Get("level")); levelParam != "" && strings.ToLower(levelParam) != "all" {
		level, err := parseLogLevel(levelParam)
		if err != nil {
			response.BadRequest(err.Error()).WriteJSON(w)
			return
		}
		opts.Level = level
	}

	logrus.Debugf("Querying logs for applicationID=%s, opts: StartTime=%v, EndTime=%v, Limit=%d, Offset=%d, Level=%s",
		appID, opts.StartTime, opts.EndTime, opts.Limit, opts.Offset, opts.Level)
	result, err := api.am.GetLogs(r.Context(), appID, opts)
	if err != nil {
		logrus.Errorf("Failed to get application logs: %v", err)
		response.InternalError("failed to get application logs: " + err.Error()).WriteJSON(w)
		return
	}

	logrus.Debugf("Got %d logs for applicationID=%s, Total=%d, HasMore=%v",
		len(result.Entries), appID, result.Total, result.HasMore)
	resp := BuildGetApplicationLogsResponse(appID, result)
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

func parseLogLevel(level string) (applogger.LogLevel, error) {
	switch strings.ToLower(level) {
	case "trace":
		return applogger.LogLevelTrace, nil
	case "debug":
		return applogger.LogLevelDebug, nil
	case "info":
		return applogger.LogLevelInfo, nil
	case "warn", "warning":
		return applogger.LogLevelWarn, nil
	case "error":
		return applogger.LogLevelError, nil
	case "fatal":
		return applogger.LogLevelFatal, nil
	case "panic":
		return applogger.LogLevelPanic, nil
	default:
		return "", fmt.Errorf("invalid level: %s", level)
	}
}

type GetApplicationLogsResponse struct {
	ApplicationID string           `json:"application_id"`
	Logs          []ApplicationLog `json:"logs"`
	Total         int              `json:"total"`
	HasMore       bool             `json:"has_more"`
}

type ApplicationLog struct {
	Timestamp time.Time             `json:"timestamp"`
	Level     string                `json:"level"`
	Message   string                `json:"message"`
	Fields    []ApplicationLogField `json:"fields,omitempty"`
	Caller    *ApplicationLogCaller `json:"caller,omitempty"`
}

type ApplicationLogField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ApplicationLogCaller struct {
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Function string `json:"function,omitempty"`
}

func BuildGetApplicationLogsResponse(appID string, result *applogger.QueryResult) GetApplicationLogsResponse {
	logs := make([]ApplicationLog, len(result.Entries))
	for i, entry := range result.Entries {
		fields := make([]ApplicationLogField, len(entry.Fields))
		for idx, field := range entry.Fields {
			fields[idx] = ApplicationLogField{
				Key:   field.Key,
				Value: field.Value,
			}
		}

		var caller *ApplicationLogCaller
		if entry.Caller != nil {
			caller = &ApplicationLogCaller{
				File:     entry.Caller.File,
				Line:     entry.Caller.Line,
				Function: entry.Caller.Function,
			}
		}

		logs[i] = ApplicationLog{
			Timestamp: entry.Timestamp,
			Level:     string(entry.Level),
			Message:   entry.Message,
			Fields:    fields,
			Caller:    caller,
		}
	}
	return GetApplicationLogsResponse{
		ApplicationID: appID,
		Logs:          logs,
		Total:         result.Total,
		HasMore:       result.HasMore,
	}
}

// 文件管理相关处理函数
func (api *API) handleGetFileTree(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["id"]
	if appID == "" {
		response.BadRequest("application id is required").WriteJSON(w)
		return
	}

	ctx := r.Context()

	// 从查询参数获取路径
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}

	// 调用 manager 获取文件树
	files, err := api.am.GetFileTree(ctx, appID, path)
	if err != nil {
		logrus.Errorf("Failed to get file tree for app %s: %v", appID, err)
		response.InternalError("failed to get file tree: " + err.Error()).WriteJSON(w)
		return
	}

	resp := GetFileTreeResponse{
		Files: files,
	}
	response.Success(resp).WriteJSON(w)
}

func (api *API) handleGetFileContent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["id"]
	if appID == "" {
		response.BadRequest("application id is required").WriteJSON(w)
		return
	}

	ctx := r.Context()

	// 从查询参数获取文件路径
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		response.BadRequest("file path is required").WriteJSON(w)
		return
	}

	// 调用 manager 获取文件内容
	content, language, err := api.am.GetFileContent(ctx, appID, filePath)
	if err != nil {
		logrus.Errorf("Failed to get file content for app %s: %v", appID, err)
		response.InternalError("failed to get file content: " + err.Error()).WriteJSON(w)
		return
	}

	resp := GetFileContentResponse{
		Content:  content,
		Language: language,
		Path:     filePath,
	}
	response.Success(resp).WriteJSON(w)
}

func (api *API) handleSaveFileContent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["id"]
	if appID == "" {
		response.BadRequest("application id is required").WriteJSON(w)
		return
	}

	ctx := r.Context()

	// 从查询参数获取文件路径
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		response.BadRequest("file path is required").WriteJSON(w)
		return
	}

	// 解析请求体
	req := SaveFileContentRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logrus.Errorf("Failed to decode save file content request: %v", err)
		response.BadRequest("invalid request body: " + err.Error()).WriteJSON(w)
		return
	}

	// 调用 manager 保存文件内容
	if err := api.am.SaveFileContent(ctx, appID, filePath, req.Content); err != nil {
		logrus.Errorf("Failed to save file content for app %s: %v", appID, err)
		response.InternalError("failed to save file content: " + err.Error()).WriteJSON(w)
		return
	}

	resp := SaveFileContentResponse{
		Message:  "File saved successfully",
		FilePath: filePath,
	}
	response.Success(resp).WriteJSON(w)
}

func (api *API) handleCreateFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["id"]
	if appID == "" {
		response.BadRequest("application id is required").WriteJSON(w)
		return
	}

	ctx := r.Context()

	// 解析请求体
	req := CreateFileRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logrus.Errorf("Failed to decode create file request: %v", err)
		response.BadRequest("invalid request body: " + err.Error()).WriteJSON(w)
		return
	}

	if req.FilePath == "" {
		response.BadRequest("file path is required").WriteJSON(w)
		return
	}

	// 调用 manager 创建文件
	if err := api.am.CreateFile(ctx, appID, req.FilePath); err != nil {
		logrus.Errorf("Failed to create file for app %s: %v", appID, err)
		response.InternalError("failed to create file: " + err.Error()).WriteJSON(w)
		return
	}

	resp := CreateFileResponse{
		Message:  "File created successfully",
		FilePath: req.FilePath,
	}
	response.Success(resp).WriteJSON(w)
}

func (api *API) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["id"]
	if appID == "" {
		response.BadRequest("application id is required").WriteJSON(w)
		return
	}

	ctx := r.Context()

	// 解析请求体
	req := DeleteFileRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logrus.Errorf("Failed to decode delete file request: %v", err)
		response.BadRequest("invalid request body: " + err.Error()).WriteJSON(w)
		return
	}

	if req.FilePath == "" {
		response.BadRequest("file path is required").WriteJSON(w)
		return
	}

	// 调用 manager 删除文件
	if err := api.am.DeleteFile(ctx, appID, req.FilePath); err != nil {
		logrus.Errorf("Failed to delete file for app %s: %v", appID, err)
		response.InternalError("failed to delete file: " + err.Error()).WriteJSON(w)
		return
	}

	resp := DeleteFileResponse{
		Message:  "File deleted successfully",
		FilePath: req.FilePath,
	}
	response.Success(resp).WriteJSON(w)
}

func (api *API) handleCreateDirectory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["id"]
	if appID == "" {
		response.BadRequest("application id is required").WriteJSON(w)
		return
	}

	ctx := r.Context()

	// 解析请求体
	req := CreateDirectoryRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logrus.Errorf("Failed to decode create directory request: %v", err)
		response.BadRequest("invalid request body: " + err.Error()).WriteJSON(w)
		return
	}

	if req.DirPath == "" {
		response.BadRequest("directory path is required").WriteJSON(w)
		return
	}

	// 调用 manager 创建目录
	if err := api.am.CreateDirectory(ctx, appID, req.DirPath); err != nil {
		logrus.Errorf("Failed to create directory for app %s: %v", appID, err)
		response.InternalError("failed to create directory: " + err.Error()).WriteJSON(w)
		return
	}

	resp := CreateDirectoryResponse{
		Message: "Directory created successfully",
		DirPath: req.DirPath,
	}
	response.Success(resp).WriteJSON(w)
}

func (api *API) handleDeleteDirectory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["id"]
	if appID == "" {
		response.BadRequest("application id is required").WriteJSON(w)
		return
	}

	ctx := r.Context()

	// 解析请求体
	req := DeleteDirectoryRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logrus.Errorf("Failed to decode delete directory request: %v", err)
		response.BadRequest("invalid request body: " + err.Error()).WriteJSON(w)
		return
	}

	if req.DirPath == "" {
		response.BadRequest("directory path is required").WriteJSON(w)
		return
	}

	// 调用 manager 删除目录
	if err := api.am.DeleteDirectory(ctx, appID, req.DirPath); err != nil {
		logrus.Errorf("Failed to delete directory for app %s: %v", appID, err)
		response.InternalError("failed to delete directory: " + err.Error()).WriteJSON(w)
		return
	}

	resp := DeleteDirectoryResponse{
		Message: "Directory deleted successfully",
		DirPath: req.DirPath,
	}
	response.Success(resp).WriteJSON(w)
}

// handleGetExecutionResult 获取执行结果
// TODO: 需要访问 store service 来获取实际数据
// 建议方案：
// 1. 修改 RegisterRoutes 接受 resource.Manager 参数
// 2. 在 API 结构中存储 resource.Manager
// 3. 通过 resource.Manager.GetObject 获取数据
func (api *API) handleGetExecutionResult(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appID := vars["id"]
	objectID := r.URL.Query().Get("object_id")
	_ = r.URL.Query().Get("source") // TODO: 使用 source 参数

	if appID == "" || objectID == "" {
		response.BadRequest("application id and object_id are required").WriteJSON(w)
		return
	}

	// TODO: 实现获取执行结果的逻辑
	// 1. 修改 RegisterRoutes 接受 resource.Manager 参数
	// 2. 在 API 结构中存储 resource.Manager
	// 3. 通过 resource.Manager.GetObject 获取 ObjectRef 对应的数据
	// 4. 根据数据类型（pickle/json/bytes）进行解码
	// 5. 返回格式化的数据

	response.BadRequest("execution result retrieval not yet implemented").WriteJSON(w)
}
