package main

import (
	"os"
	"os/exec"
	"strings"

	logrus "github.com/sirupsen/logrus"
)

const APP_CODE_PATH = "/iarnet/app/"

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

	os.Setenv("IGNIS_ADDR", "host.internal:"+ignisPort)

	logrus.Infof("Registering app %s to Ignis platform at port %s", appID, ignisPort)

	exec.Command("cd", APP_CODE_PATH)

	if envInstallCmd != "" {
		cmd := strings.Split(envInstallCmd, " ")
		envCmd := exec.Command(cmd[0], cmd[1:]...)
		envCmd.Stdout = os.Stdout
		envCmd.Stderr = os.Stderr
		if err := envCmd.Run(); err != nil {
			logrus.Fatalf("failed to install env %s: %v", envInstallCmd, err)
		}
	}

	cmd := strings.Split(executeCmd, " ")
	execCmd := exec.Command(cmd[0], cmd[1:]...)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	if err := execCmd.Run(); err != nil {
		logrus.Fatalf("failed to execute command %s: %v", executeCmd, err)
	}

	logrus.Infof("Successfully executed app %s", appID)
}
