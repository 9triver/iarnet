package application

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/9triver/iarnet/internal/domain/application"
	"github.com/9triver/iarnet/internal/domain/application/types"
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
		AppID: mux.Vars(r)["id"],
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

	resp := BuildGetApplicationDAGResponse(dag)
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

	// 更新应用状态为已停止
	if err := api.am.UpdateAppStatus(ctx, appID, types.AppStatusStopped); err != nil {
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
		case types.AppStatusRunning:
			stats.Running++
		case types.AppStatusStopped:
			stats.Stopped++
		case types.AppStatusUndeployed:
			stats.Undeployed++
		case types.AppStatusFailed:
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
