package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/9triver/iarnet/internal/config"
	httpauth "github.com/9triver/iarnet/internal/transport/http/util/auth"
	"github.com/9triver/iarnet/internal/transport/http/util/response"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func RegisterRoutes(router *mux.Router, cfg *config.Config) {
	api := NewAPI(cfg)
	router.HandleFunc("/auth/login", api.handleLogin).Methods("POST")
	router.HandleFunc("/auth/me", api.handleGetCurrentUser).Methods("GET")
}

type API struct {
	config *config.Config
}

func NewAPI(cfg *config.Config) *API {
	return &API{
		config: cfg,
	}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Username string `json:"username"`
	Token    string `json:"token"`
}

type GetCurrentUserResponse struct {
	Username string `json:"username"`
}

func (api *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest("invalid request body: " + err.Error()).WriteJSON(w)
		return
	}

	// 验证用户名和密码
	username := strings.TrimSpace(req.Username)
	if username == "" {
		response.BadRequest("username is required").WriteJSON(w)
		return
	}

	// 查找用户
	var user *config.UserConfig
	for i := range api.config.Users {
		if strings.TrimSpace(api.config.Users[i].Name) == username {
			user = &api.config.Users[i]
			break
		}
	}

	if user == nil {
		logrus.Warnf("Login failed: user not found: %s", username)
		response.Unauthorized("invalid username or password").WriteJSON(w)
		return
	}

	// 验证密码（简单字符串比较，因为密码在配置文件中是明文）
	if user.Password != req.Password {
		logrus.Warnf("Login failed: incorrect password for user: %s", username)
		response.Unauthorized("invalid username or password").WriteJSON(w)
		return
	}

	logrus.Infof("User logged in: %s", username)

	// 生成 JWT token
	token, err := httpauth.GenerateToken(username)
	if err != nil {
		logrus.Errorf("Failed to generate token: %v", err)
		response.InternalError("failed to generate token").WriteJSON(w)
		return
	}

	resp := LoginResponse{
		Username: username,
		Token:    token,
	}
	response.Success(resp).WriteJSON(w)
}

func (api *API) handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	// 从 context 中获取用户名（由中间件注入）
	username := httpauth.GetUsernameFromContext(r.Context())
	if username == "" {
		response.Unauthorized("authentication required").WriteJSON(w)
		return
	}

	// 查找用户
	var user *config.UserConfig
	for i := range api.config.Users {
		if strings.TrimSpace(api.config.Users[i].Name) == username {
			user = &api.config.Users[i]
			break
		}
	}

	if user == nil {
		response.Unauthorized("user not found").WriteJSON(w)
		return
	}

	resp := GetCurrentUserResponse{
		Username: username,
	}
	response.Success(resp).WriteJSON(w)
}
