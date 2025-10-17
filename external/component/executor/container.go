package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/9triver/ignis/objects"
	"github.com/9triver/ignis/proto"
	"gopkg.in/zeromq/goczmq.v4"
)

// ExecuteMessage ZMQ执行消息
type ExecuteMessage struct {
	Type     string                 `json:"type"`
	Function string                 `json:"function"`
	Params   map[string]interface{} `json:"params"`
	CorrID   string                 `json:"corr_id"`
}

// ReturnMessage ZMQ返回消息
type ReturnMessage struct {
	Type    string      `json:"type"`
	CorrID  string      `json:"corr_id"`
	Result  interface{} `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
	Success bool        `json:"success"`
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	Success      bool        `json:"success"`
	Result       interface{} `json:"result,omitempty"`
	Error        string      `json:"error,omitempty"`
	ErrorType    string      `json:"error_type,omitempty"`
	Traceback    string      `json:"traceback,omitempty"`
	FunctionName string      `json:"function_name,omitempty"`
}

// FunctionMetadata 函数元数据
type FunctionMetadata struct {
	Parameters       []string `json:"parameters"`
	SourceFile       string   `json:"source_file"`
	ReturnAnnotation string   `json:"return_annotation,omitempty"`
}

// ContainerExecutor 容器内的函数执行器，支持ZMQ通信
type ContainerExecutor struct {
	pythonPath   string
	executorPath string
	timeout      time.Duration
	ipcAddr      string
	router       *goczmq.Channeler
	connections  map[string]*ExecutorConnection
	mutex        sync.RWMutex
}

// ExecutorConnection 执行器连接
type ExecutorConnection struct {
	Name    string
	Frame   []byte
	Ready   bool
	Futures map[string]chan *ReturnMessage
	mutex   sync.RWMutex
}

// NewContainerExecutor 创建新的容器执行器
func NewContainerExecutor(pythonPath, executorPath string, ipcAddr string) *ContainerExecutor {
	if pythonPath == "" {
		pythonPath = "python3"
	}
	if executorPath == "" {
		executorPath = "/app/py/executor.py" // 默认容器内路径
	}
	if ipcAddr == "" {
		ipcAddr = "ipc:///tmp/container-executor"
	}

	return &ContainerExecutor{
		pythonPath:   pythonPath,
		executorPath: executorPath,
		timeout:      30 * time.Second,
		ipcAddr:      ipcAddr,
		connections:  make(map[string]*ExecutorConnection),
	}
}

// SetTimeout 设置执行超时时间
func (ce *ContainerExecutor) SetTimeout(timeout time.Duration) {
	ce.timeout = timeout
}

// Start 启动ZMQ路由器
func (ce *ContainerExecutor) Start(ctx context.Context) error {
	ce.router = goczmq.NewRouterChanneler(ce.ipcAddr)
	
	go ce.handleMessages(ctx)
	return nil
}

// Stop 停止执行器
func (ce *ContainerExecutor) Stop() {
	if ce.router != nil {
		ce.router.Destroy()
	}
}

// handleMessages 处理ZMQ消息
func (ce *ContainerExecutor) handleMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ce.router.RecvChan:
			if len(msg) < 2 {
				continue
			}
			frame, data := msg[0], msg[1]
			ce.onReceive(frame, data)
		}
	}
}

// onReceive 处理接收到的消息
func (ce *ContainerExecutor) onReceive(frame []byte, data []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "ready":
		ce.handleReady(frame, msg)
	case "return":
		ce.handleReturn(msg)
	}
}

// handleReady 处理Ready消息
func (ce *ContainerExecutor) handleReady(frame []byte, msg map[string]interface{}) {
	connName, ok := msg["conn"].(string)
	if !ok {
		return
	}

	ce.mutex.Lock()
	defer ce.mutex.Unlock()

	conn := &ExecutorConnection{
		Name:    connName,
		Frame:   frame,
		Ready:   true,
		Futures: make(map[string]chan *ReturnMessage),
	}
	ce.connections[connName] = conn
}

// handleReturn 处理返回消息
func (ce *ContainerExecutor) handleReturn(msg map[string]interface{}) {
	corrID, ok := msg["corr_id"].(string)
	if !ok {
		return
	}

	returnMsg := &ReturnMessage{
		Type:   "return",
		CorrID: corrID,
	}

	if success, ok := msg["success"].(bool); ok {
		returnMsg.Success = success
		if success {
			returnMsg.Result = msg["result"]
		} else {
			if errMsg, ok := msg["error"].(string); ok {
				returnMsg.Error = errMsg
			}
		}
	}

	// 查找对应的future
	ce.mutex.RLock()
	defer ce.mutex.RUnlock()

	for _, conn := range ce.connections {
		conn.mutex.RLock()
		if future, exists := conn.Futures[corrID]; exists {
			future <- returnMsg
			close(future)
			delete(conn.Futures, corrID)
		}
		conn.mutex.RUnlock()
	}
}

// Execute 执行函数
func (ce *ContainerExecutor) Execute(ctx context.Context, connName, funcName string, params map[string]interface{}) (interface{}, error) {
	ce.mutex.RLock()
	conn, exists := ce.connections[connName]
	ce.mutex.RUnlock()

	if !exists || !conn.Ready {
		return nil, fmt.Errorf("connection %s not ready", connName)
	}

	// 生成correlation ID
	corrID := fmt.Sprintf("%s-%d", connName, time.Now().UnixNano())

	// 创建future
	future := make(chan *ReturnMessage, 1)
	conn.mutex.Lock()
	conn.Futures[corrID] = future
	conn.mutex.Unlock()

	// 发送执行消息
	execMsg := ExecuteMessage{
		Type:     "execute",
		Function: funcName,
		Params:   params,
		CorrID:   corrID,
	}

	msgData, err := json.Marshal(execMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal execute message: %w", err)
	}

	ce.router.SendChan <- [][]byte{conn.Frame, msgData}

	// 等待结果
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-future:
		if !result.Success {
			return nil, fmt.Errorf("function execution failed: %s", result.Error)
		}
		return result.Result, nil
	case <-time.After(ce.timeout):
		return nil, fmt.Errorf("function execution timeout")
	}
}

// StartPythonExecutor 启动Python执行器进程
func (ce *ContainerExecutor) StartPythonExecutor(ctx context.Context, connName string) error {
	cmd := exec.CommandContext(ctx, "python3", ce.executorPath, 
		"--serve", "--remote", ce.ipcAddr, "--conn", connName)
	
	// 设置环境变量
	cmd.Env = append(os.Environ(), "PYTHONPATH=/app")
	
	return cmd.Start()
}

// ExecuteFunction 执行指定的函数
func (ce *ContainerExecutor) ExecuteFunction(ctx context.Context, funcPath, funcName string, args map[string]interface{}) (*ExecutionResult, error) {
	// 序列化参数
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal arguments: %w", err)
	}

	// 创建执行上下文
	execCtx, cancel := context.WithTimeout(ctx, ce.timeout)
	defer cancel()

	// 构建命令
	cmd := exec.CommandContext(execCtx, ce.pythonPath, ce.executorPath, "execute", funcPath, funcName, string(argsJSON))

	// 执行命令
	output, err := cmd.Output()
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return &ExecutionResult{
				Success:   false,
				Error:     "execution timeout",
				ErrorType: "TimeoutError",
			}, nil
		}
		return nil, fmt.Errorf("failed to execute function: %w", err)
	}

	// 解析结果
	var result ExecutionResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse execution result: %w", err)
	}

	return &result, nil
}

// LoadFunction 加载函数（验证函数是否可用）
func (ce *ContainerExecutor) LoadFunction(ctx context.Context, funcPath, funcName string) error {
	execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, ce.pythonPath, ce.executorPath, "load", funcPath, funcName)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return fmt.Errorf("failed to parse load result: %w", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		message := "unknown error"
		if msg, ok := result["message"].(string); ok {
			message = msg
		}
		return fmt.Errorf("failed to load function: %s", message)
	}

	return nil
}

// ContainerFunction 实现 ignis Function 接口的容器函数
type ContainerFunction struct {
	name         string
	params       []string
	funcPath     string
	executor     *ContainerExecutor
	language     proto.Language
}

// NewContainerFunction 创建新的容器函数
func NewContainerFunction(name, funcPath string, params []string, executor *ContainerExecutor) *ContainerFunction {
	return &ContainerFunction{
		name:     name,
		params:   params,
		funcPath: funcPath,
		executor: executor,
		language: proto.Language_LANG_PYTHON, // 目前只支持 Python
	}
}

// Name 返回函数名
func (cf *ContainerFunction) Name() string {
	return cf.name
}

// Params 返回参数列表
func (cf *ContainerFunction) Params() []string {
	return cf.params
}

// Language 返回语言类型
func (cf *ContainerFunction) Language() proto.Language {
	return cf.language
}

// Call 调用函数
func (cf *ContainerFunction) Call(params map[string]objects.Interface) (objects.Interface, error) {
	// 转换参数格式
	args := make(map[string]interface{})
	for key, obj := range params {
		value, err := obj.Value()
		if err != nil {
			return nil, fmt.Errorf("failed to get value for parameter %s: %w", key, err)
		}
		args[key] = value
	}

	// 执行函数
	result, err := cf.executor.ExecuteFunction(context.Background(), cf.funcPath, cf.name, args)
	if err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("function execution failed: %s (%s)", result.Error, result.ErrorType)
	}

	// 返回结果
	return objects.NewLocal(result.Result, objects.LangJson), nil
}

// TimedCall 带时间测量的函数调用
func (cf *ContainerFunction) TimedCall(params map[string]objects.Interface) (time.Duration, objects.Interface, error) {
	start := time.Now()
	result, err := cf.Call(params)
	duration := time.Since(start)
	return duration, result, err
}

// ValidateFunction 验证函数是否可用
func (cf *ContainerFunction) ValidateFunction(ctx context.Context) error {
	return cf.executor.LoadFunction(ctx, cf.funcPath, cf.name)
}

// FunctionBuilder 函数构建器，用于从函数定义创建容器函数
type FunctionBuilder struct {
	executor *ContainerExecutor
}

// NewFunctionBuilder 创建函数构建器
func NewFunctionBuilder(executor *ContainerExecutor) *FunctionBuilder {
	return &FunctionBuilder{
		executor: executor,
	}
}

// BuildFromPath 从文件路径构建函数
func (fb *FunctionBuilder) BuildFromPath(funcPath, funcName string, params []string) (*ContainerFunction, error) {
	// 验证文件路径
	absPath, err := filepath.Abs(funcPath)
	if err != nil {
		return nil, fmt.Errorf("invalid function path: %w", err)
	}

	// 创建函数
	function := NewContainerFunction(funcName, absPath, params, fb.executor)

	// 验证函数
	if err := function.ValidateFunction(context.Background()); err != nil {
		logrus.Warnf("Function validation failed for %s: %v", funcName, err)
		// 不返回错误，允许运行时验证
	}

	return function, nil
}

// BuildFromCode 从代码字符串构建函数（将代码写入临时文件）
func (fb *FunctionBuilder) BuildFromCode(funcName, code string, params []string) (*ContainerFunction, error) {
	// TODO: 实现从代码字符串创建临时文件的逻辑
	// tmpFile := fmt.Sprintf("/tmp/func_%s_%d.py", funcName, time.Now().UnixNano())
	
	return nil, fmt.Errorf("BuildFromCode not implemented yet")
}