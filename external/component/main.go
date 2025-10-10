package main

import (
	"os"
	"path"

	"github.com/sirupsen/logrus"
)

const (
	venvPath  = "/tmp/venvs"
	venvStart = "start.py"
)

// Component 代表一个在容器中运行的 Actor 组件
// 它在容器内启动 Python 进程,Python 进程通过 gRPC 连接到主 Ignis 平台
func main() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.Info("Starting Actor Component in container...")

	// 从环境变量获取配置
	appID := os.Getenv("APP_ID")
	funcName := os.Getenv("FUNC_NAME")
	ignisAddr := os.Getenv("IGNIS_ADDR")
	venvName := os.Getenv("VENV_NAME")
	pythonExec := os.Getenv("PYTHON_EXEC")

	if appID == "" {
		logrus.Fatal("APP_ID environment variable is required")
	}
	if funcName == "" {
		logrus.Fatal("FUNC_NAME environment variable is required")
	}
	if ignisAddr == "" {
		// 默认使用 host.internal 作为主机地址
		ignisAddr = "host.internal:50051"
		logrus.Warnf("IGNIS_ADDR not set, using default: %s", ignisAddr)
	}
	if venvName == "" {
		venvName = "/app/venv"
	}
	if pythonExec == "" {
		pythonExec = "python3"
	}

	logrus.Infof("Component config: appID=%s, funcName=%s, ignisAddr=%s, venv=%s",
		appID, funcName, ignisAddr, venvName)

	// 创建虚拟环境目录
	venvDir := path.Join(venvPath, venvName)
	if err := os.MkdirAll(venvDir, 0755); err != nil {
		logrus.Fatalf("Failed to create venv directory: %v", err)
	}

	// TODO

	logrus.Info("Shutting down actor component...")
}
