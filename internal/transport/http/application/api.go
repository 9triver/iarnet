package application

import (
	"context"
	"encoding/json"
	"net/http"

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
}

type API struct {
	am *application.Manager
}

func NewAPI(am *application.Manager) *API {
	return &API{
		am: am,
	}
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

	// 创建应用元数据，初始状态为 cloning
	metadata := req.ToAppMetadata()
	metadata.Status = types.AppStatusCloning
	appID, err := api.am.CreateAppMetadata(r.Context(), metadata)
	if err != nil {
		logrus.Errorf("Failed to create app metadata: %v", err)
		response.InternalError("failed to create application: " + err.Error()).WriteJSON(w)
		return
	}

	// 异步克隆 Git 仓库
	go func() {
		ctx := context.Background()
		logrus.Infof("Starting async clone for application %s", appID)

		if err := api.am.CloneRepository(ctx, string(appID), req.GitURL, req.Branch); err != nil {
			logrus.Errorf("Failed to clone repository for application %s: %v", appID, err)
			// 克隆失败，更新状态为 error
			if updateErr := api.am.UpdateAppStatus(ctx, string(appID), types.AppStatusFailed); updateErr != nil {
				logrus.Errorf("Failed to update app status to error: %v", updateErr)
			}
			return
		}

		// 克隆成功，更新状态为 idle（未部署）
		logrus.Infof("Successfully cloned repository for application %s", appID)
		if err := api.am.UpdateAppStatus(ctx, string(appID), types.AppStatusUndeployed); err != nil {
			logrus.Errorf("Failed to update app status to idle: %v", err)
		}
	}()

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

	// 更新应用状态为部署中
	if err := api.am.UpdateAppStatus(ctx, appID, types.AppStatusDeploying); err != nil {
		logrus.Errorf("Failed to update application status to deploying: %v", err)
		response.NotFound("application not found").WriteJSON(w)
		return
	}

	// 获取应用元数据
	metadata, err := api.am.GetAppMetadata(ctx, appID)
	if err != nil {
		logrus.Errorf("Failed to get app metadata: %v", err)
		api.am.UpdateAppStatus(ctx, appID, types.AppStatusFailed)
		response.NotFound("application not found").WriteJSON(w)
		return
	}

	// 创建 runner
	if err := api.am.CreateRunner(ctx, appID, "", types.RunnerEnv(metadata.RunnerEnv)); err != nil {
		logrus.Errorf("Failed to create runner: %v", err)
		api.am.UpdateAppStatus(ctx, appID, types.AppStatusFailed)
		response.InternalError("failed to create runner: " + err.Error()).WriteJSON(w)
		return
	}

	// 启动 runner
	if err := api.am.StartRunner(ctx, appID); err != nil {
		logrus.Errorf("Failed to start runner: %v", err)
		api.am.UpdateAppStatus(ctx, appID, types.AppStatusFailed)
		response.InternalError("failed to start application: " + err.Error()).WriteJSON(w)
		return
	}

	// 更新应用状态为运行中
	if err := api.am.UpdateAppStatus(ctx, appID, types.AppStatusRunning); err != nil {
		logrus.Errorf("Failed to update application status to running: %v", err)
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
