package py

import (
	"context"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/9triver/iarnet/component/runtime"
	proto "github.com/9triver/ignis/proto"
	icluster "github.com/9triver/ignis/proto/cluster"
	"github.com/9triver/ignis/utils/errors"
	"github.com/sirupsen/logrus"
)

// RuntimeManager 同时负责 Python 环境初始化（安装依赖）与运行时接口实现
type RuntimeManager struct {
	runtime.UnimplementedManager
	Timeout      time.Duration
	executorPath string
	venvPath     string
	pythonExec   string
}

const connId = "connId"

// NewRuntimeManager 创建一个 RuntimeManager。默认使用环境变量 PYTHON_PATH 或 "python3"
func NewRuntimeManager(venvPath string, executorPath string, ipcAddr string) (*RuntimeManager, error) {
	// 创建虚拟环境
	if err := os.MkdirAll(venvPath, 0755); err != nil {
		return nil, errors.WrapWith(err, "venv %s: path creation failed", venvPath)
	}

	if err := exec.Command("python3", "-m", "venv", venvPath).Run(); err != nil {
		return nil, errors.WrapWith(err, "venv %s: venv creation failed", venvPath)
	}

	pythonExec := path.Join(venvPath, "bin", "python3")

	return &RuntimeManager{
		UnimplementedManager: *runtime.NewUnimplementedManager(context.TODO(), ipcAddr, connId),
		Timeout:              10 * time.Minute,
		executorPath:         executorPath,
		venvPath:             venvPath,
		pythonExec:           pythonExec,
	}, nil
}

// ensurePip 检查 pip 是否可用，不可用则尝试通过 ensurepip 安装
func (m *RuntimeManager) ensurePip(ctx context.Context) error {
	// 尝试 python -m pip --version
	cmd := exec.CommandContext(ctx, m.pythonExec, "-m", "pip", "--version")
	if err := cmd.Run(); err == nil {
		return nil
	}
	logrus.Warn("pip not found; attempting to bootstrap via ensurepip")
	// 尝试 python -m ensurepip --upgrade
	boot := exec.CommandContext(ctx, m.pythonExec, "-m", "ensurepip", "--upgrade")
	boot.Stdout = os.Stdout
	boot.Stderr = os.Stderr
	if err := boot.Run(); err != nil {
		return err
	}
	// 再次校验
	return exec.CommandContext(ctx, m.pythonExec, "-m", "pip", "--version").Run()
}

// InstallDependencies 根据给定的 requirements 安装依赖
func (m *RuntimeManager) InstallDependencies(requirements []string) error {
	// 过滤空字符串，避免无效参数
	pkgs := make([]string, 0, len(requirements))
	for _, r := range requirements {
		r = strings.TrimSpace(r)
		if r != "" {
			pkgs = append(pkgs, r)
		}
	}
	if len(pkgs) == 0 {
		logrus.Info("no python requirements provided; skipping installation")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.Timeout)
	defer cancel()

	if err := m.ensurePip(ctx); err != nil {
		return err
	}

	args := []string{"-m", "pip", "install", "--no-cache-dir", "--upgrade"}
	// 允许通过环境变量追加自定义 index 源
	if idx := strings.TrimSpace(os.Getenv("PIP_INDEX_URL")); idx != "" {
		args = append(args, "--index-url", idx)
	}
	if extra := strings.TrimSpace(os.Getenv("PIP_EXTRA_INDEX_URL")); extra != "" {
		args = append(args, "--extra-index-url", extra)
	}
	args = append(args, pkgs...)

	logrus.Infof("installing python packages: %s", strings.Join(pkgs, ", "))
	cmd := exec.CommandContext(ctx, m.pythonExec, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (m *RuntimeManager) Language() proto.Language { return proto.Language_LANG_PYTHON }

// Setup 在容器内安装依赖，并按需启动 Python 执行器
func (m *RuntimeManager) Setup(fn *icluster.Function) error {
	if err := m.InstallDependencies(fn.Requirements); err != nil {
		return err
	}
	logrus.Infof("python requirements installed for function %s", fn.Name)
	go func() {
		cmd := exec.CommandContext(context.TODO(), m.pythonExec, m.executorPath, "--remote", m.Addr())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logrus.Errorf("python executor failed for function %s: %v", fn.Name, err)
			return
		}
	}()
	return nil
}
