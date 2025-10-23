package py

import (
	"context"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	proto "github.com/9triver/ignis/proto"
	"github.com/9triver/ignis/proto/cluster"
	"github.com/9triver/ignis/utils/errors"
	"github.com/sirupsen/logrus"
)

type Initializer struct {
	timeout      time.Duration
	executorPath string
	venvPath     string
	pythonExec   string
}

func NewInitializer(venvPath string, executorPath string) (*Initializer, error) {
	// 创建虚拟环境
	if err := os.MkdirAll(venvPath, 0755); err != nil {
		return nil, errors.WrapWith(err, "venv %s: path creation failed", venvPath)
	}

	if err := exec.Command("python3", "-m", "venv", "--system-site-packages", venvPath).Run(); err != nil {
		return nil, errors.WrapWith(err, "venv %s: venv creation failed", venvPath)
	}
	logrus.Infof("venv %s: venv creation success", venvPath)

	pythonExec := path.Join(venvPath, "bin", "python3")

	return &Initializer{
		timeout:      10 * time.Minute,
		executorPath: executorPath,
		venvPath:     venvPath,
		pythonExec:   pythonExec,
	}, nil
}

// ensurePip 检查 pip 是否可用，不可用则尝试通过 ensurepip 安装
func (i *Initializer) ensurePip(ctx context.Context) error {
	// 尝试 python -m pip --version
	cmd := exec.CommandContext(ctx, i.pythonExec, "-m", "pip", "--version")
	cmd.Env = os.Environ()
	if err := cmd.Run(); err == nil {
		return nil
	}
	logrus.Warn("pip not found; attempting to bootstrap via ensurepip")
	// 尝试 python -m ensurepip --upgrade
	boot := exec.CommandContext(ctx, i.pythonExec, "-m", "ensurepip", "--upgrade")
	boot.Stdout = os.Stdout
	boot.Stderr = os.Stderr
	boot.Env = os.Environ()
	if err := boot.Run(); err != nil {
		return err
	}
	// 再次校验
	verify := exec.CommandContext(ctx, i.pythonExec, "-m", "pip", "--version")
	verify.Env = os.Environ()
	return verify.Run()
}

func (i *Initializer) InstallDependencies(requirements []string) error {
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

	ctx, cancel := context.WithTimeout(context.Background(), i.timeout)
	defer cancel()

	if err := i.ensurePip(ctx); err != nil {
		return err
	}

	if err := exec.CommandContext(ctx, i.pythonExec, "-m", "pip", "install", "--upgrade", "pip").Run(); err != nil {
		return errors.WrapWith(err, "venv %s: pip installation failed", i.venvPath)
	}
	logrus.Infof("venv %s: pip installation success", i.venvPath)

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
	cmd := exec.CommandContext(ctx, i.pythonExec, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// 继承环境变量，确保可以访问预安装的包
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (i *Initializer) Initialize(ctx context.Context, fn *cluster.Function, addr string, connId string) error {
	if err := i.InstallDependencies(fn.Requirements); err != nil {
		return err
	}
	logrus.Infof("python requirements installed for function %s", fn.Name)
	go func() {
		cmd := exec.CommandContext(context.TODO(), i.pythonExec, i.executorPath, "--remote", addr, "--conn-id", connId)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// 确保虚拟环境可以访问预安装的 actorc 包
		// 继承当前进程的环境变量，特别是 PYTHONPATH
		cmd.Env = os.Environ()

		if err := cmd.Run(); err != nil {
			logrus.Errorf("python executor failed for function %s: %v", fn.Name, err)
			return
		}
	}()
	return nil
}

func (i *Initializer) Cleanup(ctx context.Context) error {
	panic("unimplemented")
}

func (i *Initializer) Language() proto.Language { return proto.Language_LANG_PYTHON }
