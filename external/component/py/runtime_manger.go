package py

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/9triver/iarnet/component/executor"
	"github.com/9triver/ignis/objects"
	proto "github.com/9triver/ignis/proto"
	icluster "github.com/9triver/ignis/proto/cluster"
	"github.com/9triver/ignis/utils"
	"github.com/sirupsen/logrus"
)

// CloudPickleFunctionInfo cloudpickle函数信息
type CloudPickleFunctionInfo struct {
	FunctionID   string `json:"function_id"`
	PythonFile   string `json:"python_file"`
	FunctionName string `json:"function_name"`
}

// RuntimeManager 同时负责 Python 环境初始化（安装依赖）与运行时接口实现
type RuntimeManager struct {
	PythonExec        string
	Timeout           time.Duration
	containerExecutor *executor.ContainerExecutor
	functionBuilder   *executor.FunctionBuilder
	handlerPath       string // cloudpickle_handler.py 路径
}

// NewRuntimeManager 创建一个 RuntimeManager。默认使用环境变量 PYTHON_PATH 或 "python3"
func NewRuntimeManager() *RuntimeManager {
	execPath := os.Getenv("PYTHON_PATH")
	if execPath == "" {
		execPath = "python3"
	}

	// 创建容器执行器
	executorPath := os.Getenv("EXECUTOR_PATH")
	if executorPath == "" {
		executorPath = "/app/py/executor.py" // 默认容器内路径
	}

	ipcAddr := os.Getenv("IPC_ADDR")
	if ipcAddr == "" {
		ipcAddr = "ipc:///tmp/container-executor"
	}

	containerExec := executor.NewContainerExecutor("python3", executorPath, ipcAddr)
	functionBuilder := executor.NewFunctionBuilder(containerExec)

	// 设置cloudpickle处理器路径
	handlerPath := os.Getenv("CLOUDPICKLE_HANDLER_PATH")
	if handlerPath == "" {
		// 默认使用相对于当前文件的路径
		handlerPath = filepath.Join(filepath.Dir(executorPath), "cloudpickle_handler.py")
	}

	return &RuntimeManager{
		PythonExec:        execPath,
		Timeout:           10 * time.Minute,
		containerExecutor: containerExec,
		functionBuilder:   functionBuilder,
		handlerPath:       handlerPath,
	}
}

// ensurePip 检查 pip 是否可用，不可用则尝试通过 ensurepip 安装
func (m *RuntimeManager) ensurePip(ctx context.Context) error {
	// 尝试 python -m pip --version
	cmd := exec.CommandContext(ctx, m.PythonExec, "-m", "pip", "--version")
	if err := cmd.Run(); err == nil {
		return nil
	}
	logrus.Warn("pip not found; attempting to bootstrap via ensurepip")
	// 尝试 python -m ensurepip --upgrade
	boot := exec.CommandContext(ctx, m.PythonExec, "-m", "ensurepip", "--upgrade")
	boot.Stdout = os.Stdout
	boot.Stderr = os.Stderr
	if err := boot.Run(); err != nil {
		return err
	}
	// 再次校验
	return exec.CommandContext(ctx, m.PythonExec, "-m", "pip", "--version").Run()
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
	cmd := exec.CommandContext(ctx, m.PythonExec, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (m *RuntimeManager) Language() proto.Language { return proto.Language_LANG_PYTHON }

// Setup 在容器内安装依赖，并按需启动 Python 执行器（此处仅安装依赖）
func (m *RuntimeManager) Setup(fn *icluster.Function) error {
	if err := m.InstallDependencies(fn.Requirements); err != nil {
		return err
	}
	logrus.Infof("python requirements installed for function %s", fn.Name)
	return nil
}

// CreateContainerFunction 创建容器函数
func (m *RuntimeManager) CreateContainerFunction(funcPath, funcName string, params []string) (*executor.ContainerFunction, error) {
	return m.functionBuilder.BuildFromPath(funcPath, funcName, params)
}

// GetContainerExecutor 获取容器执行器
func (m *RuntimeManager) GetContainerExecutor() *executor.ContainerExecutor {
	return m.containerExecutor
}

// SaveCloudPickleFunction 保存cloudpickle序列化的函数
func (m *RuntimeManager) SaveCloudPickleFunction(funcData []byte, funcName string, funcID string) (*CloudPickleFunctionInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.Timeout)
	defer cancel()

	// 将函数数据编码为base64
	encodedData := base64.StdEncoding.EncodeToString(funcData)

	// 构建命令参数
	args := []string{m.handlerPath, "save", "--name", funcName, "--data", encodedData}
	if funcID != "" {
		args = append(args, "--id", funcID)
	}

	// 执行cloudpickle处理器
	cmd := exec.CommandContext(ctx, m.PythonExec, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to save cloudpickle function: %w", err)
	}

	// 解析结果
	var result struct {
		Success    bool   `json:"success"`
		FunctionID string `json:"function_id"`
		PythonFile string `json:"python_file"`
		Error      string `json:"error"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse save result: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("save failed: %s", result.Error)
	}

	return &CloudPickleFunctionInfo{
		FunctionID:   result.FunctionID,
		PythonFile:   result.PythonFile,
		FunctionName: funcName,
	}, nil
}

// LoadCloudPickleFunction 加载已保存的cloudpickle函数
func (m *RuntimeManager) LoadCloudPickleFunction(funcID string) (*CloudPickleFunctionInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.Timeout)
	defer cancel()

	// 构建命令参数
	args := []string{m.handlerPath, "load", "--id", funcID}

	// 执行cloudpickle处理器
	cmd := exec.CommandContext(ctx, m.PythonExec, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to load cloudpickle function: %w", err)
	}

	// 解析结果
	var result struct {
		Success      bool   `json:"success"`
		FunctionName string `json:"function_name"`
		PythonFile   string `json:"python_file"`
		Error        string `json:"error"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse load result: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("load failed: %s", result.Error)
	}

	return &CloudPickleFunctionInfo{
		FunctionID:   funcID,
		PythonFile:   result.PythonFile,
		FunctionName: result.FunctionName,
	}, nil
}

// ExecuteCloudPickleFunction 执行cloudpickle函数
func (m *RuntimeManager) ExecuteCloudPickleFunction(funcInfo *CloudPickleFunctionInfo, params map[string]interface{}) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.Timeout)
	defer cancel()

	// 序列化参数
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %w", err)
	}

	// 构建命令参数
	args := []string{funcInfo.PythonFile, "--params", string(paramsJSON)}

	// 执行函数
	cmd := exec.CommandContext(ctx, m.PythonExec, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute cloudpickle function: %w", err)
	}

	// 解析结果
	var result struct {
		Success  bool        `json:"success"`
		Result   interface{} `json:"result"`
		Error    string      `json:"error"`
		Function string      `json:"function"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse execution result: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("execution failed: %s", result.Error)
	}

	return result.Result, nil
}

// ListCloudPickleFunctions 列出所有已保存的cloudpickle函数
func (m *RuntimeManager) ListCloudPickleFunctions() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.Timeout)
	defer cancel()

	// 构建命令参数
	args := []string{m.handlerPath, "list"}

	// 执行cloudpickle处理器
	cmd := exec.CommandContext(ctx, m.PythonExec, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list cloudpickle functions: %w", err)
	}

	// 解析结果
	var result struct {
		Success   bool                   `json:"success"`
		Functions map[string]interface{} `json:"functions"`
		Error     string                 `json:"error"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse list result: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("list failed: %s", result.Error)
	}

	return result.Functions, nil
}

// RemoveCloudPickleFunction 删除已保存的cloudpickle函数
func (m *RuntimeManager) RemoveCloudPickleFunction(funcID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), m.Timeout)
	defer cancel()

	// 构建命令参数
	args := []string{m.handlerPath, "remove", "--id", funcID}

	// 执行cloudpickle处理器
	cmd := exec.CommandContext(ctx, m.PythonExec, args...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to remove cloudpickle function: %w", err)
	}

	// 解析结果
	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return fmt.Errorf("failed to parse remove result: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("remove failed: %s", result.Error)
	}

	return nil
}

func (m *RuntimeManager) Execute(name, method string, args map[string]objects.Interface) utils.Future[objects.Interface] {
	// fut := utils.NewFuture[objects.Interface](configs.ExecutionTimeout)
	// encoded := make(map[string]*objects.Remote)
	// for param, obj := range args {
	// 	enc, err := obj.Encode()
	// 	if err != nil {
	// 		fut.Reject(err)
	// 		return fut
	// 	}
	// 	encoded[param] = enc
	// }

	// corrId := utils.GenID()
	// v.futures[corrId] = fut

	// msg := executor.NewExecute(v.Name, corrId, name, method, encoded)
	// v.handler.SendChan() <- msg

	// for _, arg := range args {
	// 	if stream, ok := arg.(*objects.Stream); ok {
	// 		chunks := stream.ToChan()
	// 		go func() {
	// 			defer func() {
	// 				v.handler.SendChan() <- executor.NewStreamEnd(v.Name, stream.GetID())
	// 			}()
	// 			for chunk := range chunks {
	// 				encoded, err := chunk.Encode()
	// 				v.handler.SendChan() <- executor.NewStreamChunk(v.Name, stream.GetID(), encoded, err)
	// 			}
	// 		}()
	// 	}
	// }
	// return fut
	panic("not implemented")
}
