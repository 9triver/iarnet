package main

import (
	"os"
	"os/exec"
	"path/filepath"

	logrus "github.com/sirupsen/logrus"
)

const APP_CODE_PATH = "/iarnet/app/"
const ENV_INSTALLED_MARKER = ".env_installed"

func main() {
	appID := os.Getenv("APP_ID")
	ignisPort := os.Getenv("IGNIS_PORT")
	envInstallCmd := os.Getenv("ENV_INSTALL_CMD")
	executeCmd := os.Getenv("EXECUTE_CMD")

	if appID == "" {
		logrus.Fatalf("APP_ID environment variable is required")
	}
	if ignisPort == "" {
		logrus.Fatalf("IGNIS_PORT environment variable is required")
	}
	if executeCmd == "" {
		logrus.Fatalf("EXECUTE_CMD environment variable is required")
	}

	os.Setenv("MASTER_ADDR", "host.internal:"+ignisPort)

	logrus.Infof("Registering app %s to Ignis platform at port %s", appID, ignisPort)

	markerPath := filepath.Join(APP_CODE_PATH, ENV_INSTALLED_MARKER)

	// 检查是否已经执行过环境安装命令
	_, err := os.Stat(markerPath)
	envInstalled := err == nil

	if envInstallCmd != "" && !envInstalled {
		logrus.Infof("Executing environment installation command for app %s", appID)
		// 支持多行环境安装命令
		envCmd := exec.Command("bash", "-c", envInstallCmd)
		envCmd.Dir = APP_CODE_PATH
		envCmd.Stdout = os.Stdout
		envCmd.Stderr = os.Stderr
		if err := envCmd.Run(); err != nil {
			logrus.Fatalf("failed to install env %s: %v", envInstallCmd, err)
		}

		// 创建标记文件，表示环境已安装
		markerFile, err := os.Create(markerPath)
		if err != nil {
			logrus.Warnf("Failed to create env installed marker file: %v", err)
		} else {
			markerFile.Close()
			logrus.Infof("Environment installation completed for app %s", appID)
		}
	} else if envInstalled {
		logrus.Infof("Environment already installed for app %s, skipping installation", appID)
	}

	// 支持多行执行命令，使用bash -c来执行
	execCmd := exec.Command("bash", "-c", executeCmd)
	execCmd.Dir = APP_CODE_PATH
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	if err := execCmd.Run(); err != nil {
		logrus.Fatalf("failed to execute command %s: %v", executeCmd, err)
	}

	logrus.Infof("Successfully executed app %s", appID)
}
