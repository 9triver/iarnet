package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/9triver/iarnet/internal/bootstrap"
	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/domain/application/types"
	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/domain/resource/scheduler"
	"github.com/9triver/iarnet/internal/util"
	"github.com/sirupsen/logrus"
)

// TaskType 表示小/中/大三类任务
type TaskType string

const (
	TaskSmall  TaskType = "small"
	TaskMedium TaskType = "medium"
	TaskLarge  TaskType = "large"
)

// TaskRecord 用于记录单个任务的关键指标，便于后续按"已提交任务数"做统计与绘图
type TaskRecord struct {
	ID           int
	Type         TaskType
	SubmitTime   time.Time
	DispatchTime time.Time
	DeployTime   time.Time
	FinishTime   time.Time
	Status       string // success / timeout / error
	Error        string
	NodeID       string
	ProviderID   string
	IsCrossNode  bool
	IsTimeout    bool // 是否超时（明确标记）
}

// WorkloadConfig 描述实验中使用的任务负载分布
type WorkloadConfig struct {
	TotalTasks int
	Rate       float64 // 请求速率，req/s

	SmallRatio  float64
	MediumRatio float64
	LargeRatio  float64

	// 调度请求超时时间
	Timeout time.Duration
}

// ExperimentConfig 描述一次完整实验运行所需的配置
type ExperimentConfig struct {
	// 逻辑批大小：每提交 B 个任务做一次资源利用统计
	BatchSize int

	// 输出 CSV 路径，用于后续绘制图表
	MetricCSV string

	// 资源利用率 CSV 路径
	ResourceUtilizationCSV string
}

// ResourceUtilizationRecord 记录批次资源利用率
type ResourceUtilizationRecord struct {
	BatchNumber       int       // 批次编号（第几个批次）
	TaskCount         int       // 已提交任务数
	Timestamp         time.Time // 统计时间戳
	NodeID            string    // 节点 ID
	NodeName          string    // 节点名称
	CPUUtilization    float64   // CPU 利用率（百分比）
	MemoryUtilization float64   // 内存利用率（百分比）
	GPUUtilization    float64   // GPU 利用率（百分比）
	CPUUsed           int64     // 已使用 CPU（millicores）
	CPUTotal          int64     // 总 CPU（millicores）
	MemoryUsed        int64     // 已使用内存（bytes）
	MemoryTotal       int64     // 总内存（bytes）
	GPUUsed           int64     // 已使用 GPU
	GPUTotal          int64     // 总 GPU
}

// NodeResourceUtilization 节点资源利用率统计
type NodeResourceUtilization struct {
	NodeID               string
	NodeName             string
	AvgCPUUtilization    float64 // 平均 CPU 利用率（百分比）
	AvgMemoryUtilization float64 // 平均内存利用率（百分比）
	AvgGPUUtilization    float64 // 平均 GPU 利用率（百分比）
	MaxCPUUtilization    float64 // 最大 CPU 利用率（百分比）
	MaxMemoryUtilization float64 // 最大内存利用率（百分比）
	MaxGPUUtilization    float64 // 最大 GPU 利用率（百分比）
	SampleCount          int     // 采样次数
}

// TaskTypeStats 任务类型统计
type TaskTypeStats struct {
	Type         TaskType
	TotalCount   int     // 总任务数
	SuccessCount int     // 成功任务数
	FailedCount  int     // 失败任务数
	SuccessRate  float64 // 成功率（百分比）
	FailureRate  float64 // 失败率（百分比）
}

// NodeExecutionStats 节点执行统计
type NodeExecutionStats struct {
	NodeID         string
	NodeName       string
	ExecutionCount int     // 执行任务数
	ExecutionRate  float64 // 执行比例（百分比）
	// 任务类型统计
	SmallCount  int     // 小任务数
	MediumCount int     // 中任务数
	LargeCount  int     // 大任务数
	SmallRate   float64 // 小任务比例（百分比）
	MediumRate  float64 // 中任务比例（百分比）
	LargeRate   float64 // 大任务比例（百分比）
}

// AggregateMetrics 聚合指标
type AggregateMetrics struct {
	TotalTasks               int
	SuccessTasks             int
	FailedTasks              int // 失败任务数（包含超时和错误）
	LocalExecutionCount      int
	CrossNodeExecutionCount  int
	AvgResponseLatency       float64                    // 平均响应时延（ms）
	P50ResponseLatency       float64                    // P50 响应时延（ms）
	P95ResponseLatency       float64                    // P95 响应时延（ms）
	P99ResponseLatency       float64                    // P99 响应时延（ms）
	AvgFullLatency           float64                    // 平均完成时延（ms）
	P50FullLatency           float64                    // P50 完成时延（ms）
	P95FullLatency           float64                    // P95 完成时延（ms）
	P99FullLatency           float64                    // P99 完成时延（ms）
	Throughput               float64                    // 系统吞吐量（req/s）
	FailureRate              float64                    // 失败率（百分比）
	LocalExecutionRate       float64                    // 本地执行比例（百分比）
	CrossNodeExecutionRate   float64                    // 跨节点执行比例（百分比）
	TotalDuration            time.Duration              // 总实验时长
	TaskTypeStats            []*TaskTypeStats           // 各类型任务统计
	NodeExecutionStats       []*NodeExecutionStats      // 各节点执行统计
	NodeResourceUtilizations []*NodeResourceUtilization // 各节点资源占用统计
}

func main() {
	// 添加全局 panic 恢复机制，防止未捕获的 panic 导致程序崩溃
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unhandled panic in main: %v", r)
			logrus.Error("Program will exit due to unhandled panic")
			os.Exit(1)
		}
	}()

	cfg, err := config.LoadConfig("./config.yaml")
	if err != nil {
		log.Fatalf("Load config: %v", err)
	}
	util.InitLogger()

	// 初始化日志文件输出
	logDir := filepath.Join(cfg.DataDir, "logs")
	if logPath, err := util.InitLoggerWithFile(logDir); err != nil {
		logrus.Warnf("Failed to initialize log file: %v, continuing with console output only", err)
	} else {
		logrus.Infof("Log file initialized: %s", logPath)
		defer util.CloseLogFile()
	}

	// 使用 Bootstrap 初始化所有模块（包含 ZMQ panic 恢复）
	iarnet, err := bootstrap.Initialize(cfg)
	if err != nil {
		logrus.Fatalf("Failed to initialize: %v", err)
	}
	defer iarnet.Stop()

	// 创建上下文用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动所有服务
	if err := iarnet.Start(ctx); err != nil {
		logrus.Fatalf("Failed to start services: %v", err)
	}

	iarnet.IgnisPlatform.CreateController(ctx, "test")

	logrus.Info("Iarnet started successfully")

	// 等待用户输入以开始执行实验
	fmt.Print("Iarnet 已启动，按回车键开始执行实验...")
	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadString('\n')
	fmt.Println("开始执行实验...")

	err = runExperiment(iarnet, ExperimentConfig{
		BatchSize:              100,
		MetricCSV:              "experiment_results.csv",
		ResourceUtilizationCSV: "resource_utilization.csv",
	}, WorkloadConfig{
		TotalTasks:  2000,
		Rate:        20,
		SmallRatio:  0.2,
		MediumRatio: 0.7,
		LargeRatio:  0.1,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		logrus.Fatalf("Failed to run experiment: %v", err)
	}

	// 优雅关闭
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logrus.Info("Shutting down...")

	// 取消上下文以停止组件管理器和 ZMQ 接收器
	cancel()

	logrus.Info("Shutdown complete")
}

// runExperiment 只负责"一次 N 固定"的实验运行。
// 不同节点数 N 的对比，可通过多次运行（指向不同规模的集群）并离线汇总结果完成。
// 实验代码模拟执行引擎的行为，直接调用 ResourceManager 的 DeployComponent 方法。
// 注意：mock provider 会根据资源使用情况在本地自动模拟执行时间，因此不需要在代码中指定任务执行时间。
func runExperiment(iarnet *bootstrap.Iarnet, ecfg ExperimentConfig, wcfg WorkloadConfig) error {
	// 初始化随机数种子，确保每次运行都有不同的随机资源分配
	rand.Seed(time.Now().UnixNano())

	// 获取 ResourceManager，用于直接调用 DeployComponent
	resourceManager := iarnet.ResourceManager
	if resourceManager == nil {
		return fmt.Errorf("resource manager is nil")
	}

	// 获取入口节点 ID，用于判断是否跨节点调度
	entryNodeID := resourceManager.GetNodeID()
	logrus.Infof("Experiment started, entry node ID: %s", entryNodeID)

	// 生成带时间戳的 CSV 文件名，避免覆盖之前的实验结果
	timestamp := time.Now().Format("20060102_150405")

	// 检查文件名是否已经包含时间戳格式（8位数字_6位数字）
	hasTimestamp := func(filename string) bool {
		// 检查是否包含类似 "20060102_150405" 的时间戳格式
		// 使用正则表达式检查：8位数字_6位数字
		for i := 0; i <= len(filename)-15; i++ {
			substr := filename[i : i+15]
			if len(substr) == 15 {
				// 检查格式：8位数字_6位数字
				valid := true
				for j := 0; j < 8; j++ {
					if substr[j] < '0' || substr[j] > '9' {
						valid = false
						break
					}
				}
				if valid && substr[8] == '_' {
					for j := 9; j < 15; j++ {
						if substr[j] < '0' || substr[j] > '9' {
							valid = false
							break
						}
					}
					if valid {
						return true
					}
				}
			}
		}
		return false
	}

	// 生成带时间戳的文件名
	metricCSVPath := ecfg.MetricCSV
	if !hasTimestamp(metricCSVPath) {
		baseName := strings.TrimSuffix(metricCSVPath, ".csv")
		metricCSVPath = fmt.Sprintf("%s_%s.csv", baseName, timestamp)
	}

	resourceUtilizationCSVPath := ecfg.ResourceUtilizationCSV
	if resourceUtilizationCSVPath != "" && !hasTimestamp(resourceUtilizationCSVPath) {
		baseName := strings.TrimSuffix(resourceUtilizationCSVPath, ".csv")
		resourceUtilizationCSVPath = fmt.Sprintf("%s_%s.csv", baseName, timestamp)
	}

	logrus.Infof("Results will be saved to: %s, %s", metricCSVPath, resourceUtilizationCSVPath)

	// 预先生成任务类型序列并打乱，确保小中大任务混合发送
	taskTypes := generateShuffledTaskTypes(wcfg)
	logrus.Infof("Generated task type sequence: Small=%d, Medium=%d, Large=%d",
		countTaskType(taskTypes, TaskSmall),
		countTaskType(taskTypes, TaskMedium),
		countTaskType(taskTypes, TaskLarge))

	records := make([]*TaskRecord, wcfg.TotalTasks)
	var wg sync.WaitGroup
	wg.Add(wcfg.TotalTasks)

	// 控制固定速率发出请求
	interval := time.Duration(float64(time.Second) / wcfg.Rate)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	startTime := time.Now()
	taskID := 0

	// 用于批次统计的互斥锁和计数器
	var mu sync.Mutex
	completedTasks := 0
	resourceUtilRecords := []*ResourceUtilizationRecord{}

	for taskID < wcfg.TotalTasks {
		<-ticker.C

		go func(id int) {
			defer func() {
				wg.Done()
				// 每完成一个批次的任务，统计一次资源利用率
				mu.Lock()
				completedTasks++
				currentBatch := completedTasks / ecfg.BatchSize
				prevBatch := (completedTasks - 1) / ecfg.BatchSize
				shouldCollect := currentBatch > prevBatch && completedTasks%ecfg.BatchSize == 0
				mu.Unlock()

				if shouldCollect {
					// 收集资源利用率数据
					if err := collectResourceUtilization(iarnet, currentBatch, completedTasks, &resourceUtilRecords); err != nil {
						logrus.Warnf("Failed to collect resource utilization at batch %d: %v", currentBatch, err)
					}
				}
			}()
			rec := &TaskRecord{ID: id}
			// 使用预生成的打乱后的任务类型序列
			taskType := taskTypes[id]
			rec.Type = taskType
			rec.SubmitTime = time.Now()

			// 构造资源请求（模拟执行引擎向 resource manager 提交任务）
			resourceRequest := &types.Info{
				CPU:    cpuForTask(taskType),
				Memory: memForTask(taskType),
				GPU:    gpuForTask(taskType),
			}

			// 重试调度，最多重试3次
			const maxRetries = 3
			// 所有任务使用统一的随机重试延迟范围，避免震荡
			const retryDelayMin = 5 * time.Second  // 最小重试延迟：5秒
			const retryDelayMax = 15 * time.Second // 最大重试延迟：15秒
			var comp *component.Component
			var err error
			var lastErr error
			var shouldRetry bool

			rec.DispatchTime = time.Now()
			for attempt := 0; attempt < maxRetries; attempt++ {
				// 使用 context.Background()，不设置超时
				ctx := context.Background()

				// 直接调用 ResourceManager 的 DeployComponent 方法，模拟执行引擎的行为
				comp, err = resourceManager.DeployComponent(ctx, types.RuntimeEnvPython, resourceRequest)

				if err == nil && comp != nil {
					// 调度成功
					break
				}

				lastErr = err
				shouldRetry = false

				if err != nil {
					// 判断失败原因
					errStr := err.Error()
					// 检查是否是无可用 provider 导致的失败
					if strings.Contains(errStr, "failed to find available provider") ||
						strings.Contains(errStr, "no available provider") ||
						strings.Contains(errStr, "resource not available") {
						shouldRetry = true
					}
					// 检查是否是超时
					if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
						shouldRetry = true
					}
				}

				// 如果是最后一次尝试或不应该重试，则退出循环
				if attempt == maxRetries-1 || !shouldRetry {
					break
				}

				// 在范围内随机选择重试延迟，避免所有任务同时重试导致震荡
				delta := retryDelayMax - retryDelayMin
				randomMs := retryDelayMin.Milliseconds() + rand.Int63n(delta.Milliseconds()+1)
				retryDelay := time.Duration(randomMs) * time.Millisecond
				logrus.Debugf("Task %d (%s): Retry attempt %d/%d after %v (random delay in [%v, %v], error: %v)",
					id, taskType, attempt+1, maxRetries, retryDelay, retryDelayMin, retryDelayMax, err)
				time.Sleep(retryDelay)
			}

			// 记录 DeployTime / Status
			rec.DeployTime = time.Now()

			if err != nil || comp == nil {
				// 调度失败（经过重试后仍然失败）
				rec.Status = "failed"
				rec.IsTimeout = false // 不再单独统计超时，统一计入失败
				if lastErr != nil {
					rec.Error = lastErr.Error()
				} else if comp == nil {
					rec.Error = "component is nil"
				} else {
					rec.Error = "unknown error"
				}
				records[id] = rec
				return
			}

			// 从 component 中提取节点和 provider 信息
			providerID := comp.GetProviderID()
			rec.ProviderID = providerID

			// 判断是否跨节点：
			// - 本地部署：provider ID 格式为 "local.{providerID}"
			// - 跨节点部署：provider ID 格式为 "{providerID}@{nodeID}"
			if len(providerID) > 6 && providerID[:6] == "local." {
				// 本地部署
				rec.NodeID = entryNodeID
				rec.IsCrossNode = false
			} else if idx := len(providerID) - 1; idx >= 0 {
				// 检查是否包含 "@" 符号（跨节点部署的标识）
				atIdx := -1
				for i := len(providerID) - 1; i >= 0; i-- {
					if providerID[i] == '@' {
						atIdx = i
						break
					}
				}
				if atIdx >= 0 {
					// 跨节点部署：提取节点 ID
					rec.NodeID = providerID[atIdx+1:]
					rec.ProviderID = providerID[:atIdx] // 只保留 provider ID 部分
					rec.IsCrossNode = true
				} else {
					// 未知格式，默认认为是本地部署
					rec.NodeID = entryNodeID
					rec.IsCrossNode = false
				}
			} else {
				// 空 provider ID，默认认为是本地部署
				rec.NodeID = entryNodeID
				rec.IsCrossNode = false
			}

			// mock provider 会根据资源使用情况在本地自动模拟执行时间
			// 任务执行在 provider 内部完成，这里只需要记录部署完成时间
			// 由于 mock provider 是同步执行的，DeployComponent 返回时任务已完成
			rec.FinishTime = time.Now()
			rec.Status = "success"
			rec.IsTimeout = false

			records[id] = rec
		}(taskID)

		taskID++
	}

	wg.Wait()
	endTime := time.Now()
	totalDuration := endTime.Sub(startTime)
	logrus.Infof("Experiment finished, total duration: %v", totalDuration)

	// 写入任务级别指标 CSV（使用之前生成的文件名）
	if err := writeRecordsToCSV(metricCSVPath, records); err != nil {
		return fmt.Errorf("write csv: %w", err)
	}

	// 写入资源利用率 CSV（使用之前生成的文件名）
	if resourceUtilizationCSVPath != "" {
		if err := writeResourceUtilizationToCSV(resourceUtilizationCSVPath, resourceUtilRecords); err != nil {
			logrus.Warnf("Failed to write resource utilization CSV: %v", err)
		}
	}

	// 计算并输出聚合指标
	metrics := calculateAggregateMetrics(records, totalDuration, entryNodeID, resourceUtilRecords, iarnet)
	if err := writeAggregateMetrics(metrics, entryNodeID); err != nil {
		logrus.Warnf("Failed to write aggregate metrics: %v", err)
	} else {
		printAggregateMetrics(metrics)
	}

	return nil
}

// generateShuffledTaskTypes 生成符合比例要求的任务类型序列，并打乱顺序
// 确保小中大任务混合发送，而不是先发送完所有小任务再发送中任务
func generateShuffledTaskTypes(w WorkloadConfig) []TaskType {
	total := w.TotalTasks
	smallCount := int(float64(total) * w.SmallRatio)
	mediumCount := int(float64(total) * w.MediumRatio)
	largeCount := total - smallCount - mediumCount // 剩余的都是大任务

	// 创建任务类型序列
	taskTypes := make([]TaskType, 0, total)

	// 添加小任务
	for i := 0; i < smallCount; i++ {
		taskTypes = append(taskTypes, TaskSmall)
	}

	// 添加中任务
	for i := 0; i < mediumCount; i++ {
		taskTypes = append(taskTypes, TaskMedium)
	}

	// 添加大任务
	for i := 0; i < largeCount; i++ {
		taskTypes = append(taskTypes, TaskLarge)
	}

	// 使用当前时间作为随机种子，打乱序列
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := len(taskTypes) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		taskTypes[i], taskTypes[j] = taskTypes[j], taskTypes[i]
	}

	return taskTypes
}

// countTaskType 统计任务类型序列中指定类型的数量
func countTaskType(taskTypes []TaskType, taskType TaskType) int {
	count := 0
	for _, t := range taskTypes {
		if t == taskType {
			count++
		}
	}
	return count
}

func cpuForTask(t TaskType) int64 {
	// 真实场景中的任务 CPU 配置（在区间内随机）
	// Provider: 16核 (16000mc), 32GB, 2 GPU
	switch t {
	case TaskSmall:
		// 小任务：300-700mc (0.3-0.7核)
		return rand.Int63n(401) + 300 // 300-700
	case TaskMedium:
		// 中任务：1.5-2.5核 (1500-2500mc)
		return rand.Int63n(1001) + 1500 // 1500-2500
	case TaskLarge:
		// 大任务：3.5-4.5核 (3500-4500mc)
		return rand.Int63n(1001) + 3500 // 3500-4500
	default:
		return 500
	}
}

func memForTask(t TaskType) int64 {
	const mb = 1024 * 1024
	// 真实场景中的任务内存配置（在区间内随机）
	// 基于实际应用场景的典型内存需求，不强制匹配 Provider 比例
	switch t {
	case TaskSmall:
		// 小任务：512MB-1GB
		return (rand.Int63n(513) + 512) * mb // 512-1024 MB (0.5-1GB)
	case TaskMedium:
		// 中任务：2GB-3GB
		return (rand.Int63n(1025) + 2048) * mb // 2048-3072 MB (2-3GB)
	case TaskLarge:
		// 大任务：4GB-6GB
		return (rand.Int63n(2049) + 4096) * mb // 4096-6144 MB (4-6GB)
	default:
		return 512 * mb
	}
}

func gpuForTask(t TaskType) int64 {
	if t == TaskLarge {
		return 1
	}
	return 0
}

// writeRecordsToCSV 将所有任务记录写出到 CSV，便于后续用 Python / R 绘图分析。
func writeRecordsToCSV(path string, records []*TaskRecord) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{
		"id", "type", "status", "error", "is_timeout",
		"submit_ts", "dispatch_ts", "deploy_ts", "finish_ts",
		"latency_resp_ms", "latency_full_ms",
		"node_id", "provider_id", "is_cross_node",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, rec := range records {
		if rec == nil {
			continue
		}
		latResp := rec.DeployTime.Sub(rec.SubmitTime).Milliseconds()
		latFull := rec.FinishTime.Sub(rec.SubmitTime).Milliseconds()
		row := []string{
			fmt.Sprintf("%d", rec.ID),
			string(rec.Type),
			rec.Status,
			rec.Error,
			fmt.Sprintf("%v", rec.IsTimeout),
			rec.SubmitTime.Format(time.RFC3339Nano),
			rec.DispatchTime.Format(time.RFC3339Nano),
			rec.DeployTime.Format(time.RFC3339Nano),
			rec.FinishTime.Format(time.RFC3339Nano),
			fmt.Sprintf("%d", latResp),
			fmt.Sprintf("%d", latFull),
			rec.NodeID,
			rec.ProviderID,
			fmt.Sprintf("%v", rec.IsCrossNode),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// collectResourceUtilization 收集资源利用率数据（包括本地节点和所有已知的远程节点）
func collectResourceUtilization(iarnet *bootstrap.Iarnet, batchNumber int, taskCount int, records *[]*ResourceUtilizationRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 收集所有节点的资源利用率
	nodesToCollect := []struct {
		nodeID   string
		nodeName string
		address  string
		isLocal  bool
	}{}

	// 1. 添加本地节点
	if iarnet.ResourceManager != nil {
		localNodeID := iarnet.ResourceManager.GetNodeID()
		localNodeName := ""
		// 尝试从 discovery service 获取本地节点名称
		if iarnet.DiscoveryService != nil {
			localNode := iarnet.DiscoveryService.GetLocalNode()
			if localNode != nil {
				localNodeName = localNode.NodeName
			}
		}
		nodesToCollect = append(nodesToCollect, struct {
			nodeID   string
			nodeName string
			address  string
			isLocal  bool
		}{nodeID: localNodeID, nodeName: localNodeName, address: "", isLocal: true})
	}

	// 2. 添加所有已知的远程节点
	if iarnet.DiscoveryService != nil {
		knownNodes := iarnet.DiscoveryService.GetKnownNodes()
		for _, node := range knownNodes {
			if node != nil && node.NodeID != "" && node.SchedulerAddress != "" {
				// 跳过本地节点（已在上面添加）
				if iarnet.ResourceManager != nil && node.NodeID == iarnet.ResourceManager.GetNodeID() {
					continue
				}
				nodesToCollect = append(nodesToCollect, struct {
					nodeID   string
					nodeName string
					address  string
					isLocal  bool
				}{nodeID: node.NodeID, nodeName: node.NodeName, address: node.SchedulerAddress, isLocal: false})
			}
		}
	}

	// 3. 对每个节点收集资源利用率
	for _, nodeInfo := range nodesToCollect {
		var resp *scheduler.ProviderListResponse
		var err error

		if nodeInfo.isLocal {
			// 本地节点：直接调用 scheduler service
			if iarnet.SchedulerService != nil {
				resp, err = iarnet.SchedulerService.ListProviders(ctx, true)
			} else {
				continue
			}
		} else {
			// 远程节点：通过 scheduler service 调用远程节点的 ListProviders
			if iarnet.SchedulerService != nil {
				// 使用 scheduler service 的 ListRemoteProviders 方法
				resp, err = iarnet.SchedulerService.ListRemoteProviders(ctx, nodeInfo.nodeID, nodeInfo.address, true)
			} else {
				continue
			}
		}

		if err != nil {
			logrus.Warnf("Failed to collect resource utilization for node %s (%s): %v", nodeInfo.nodeName, nodeInfo.nodeID, err)
			continue
		}

		if resp != nil && resp.Success && len(resp.Providers) > 0 {
			// 计算节点级聚合资源利用率
			var totalCPU, usedCPU, totalMemory, usedMemory, totalGPU, usedGPU int64
			for _, provider := range resp.Providers {
				if provider.TotalCapacity != nil {
					totalCPU += provider.TotalCapacity.CPU
					totalMemory += provider.TotalCapacity.Memory
					totalGPU += provider.TotalCapacity.GPU
				}
				if provider.Used != nil {
					usedCPU += provider.Used.CPU
					usedMemory += provider.Used.Memory
					usedGPU += provider.Used.GPU
				}
			}

			var cpuUtil, memUtil, gpuUtil float64
			if totalCPU > 0 {
				cpuUtil = float64(usedCPU) / float64(totalCPU) * 100
			}
			if totalMemory > 0 {
				memUtil = float64(usedMemory) / float64(totalMemory) * 100
			}
			if totalGPU > 0 {
				gpuUtil = float64(usedGPU) / float64(totalGPU) * 100
			}

			// 使用节点信息中的名称，如果响应中有名称则优先使用响应中的
			nodeName := nodeInfo.nodeName
			if resp.NodeName != "" {
				nodeName = resp.NodeName
			}
			nodeID := nodeInfo.nodeID
			if resp.NodeID != "" {
				nodeID = resp.NodeID
			}

			*records = append(*records, &ResourceUtilizationRecord{
				BatchNumber:       batchNumber,
				TaskCount:         taskCount,
				Timestamp:         time.Now(),
				NodeID:            nodeID,
				NodeName:          nodeName,
				CPUUtilization:    cpuUtil,
				MemoryUtilization: memUtil,
				GPUUtilization:    gpuUtil,
				CPUUsed:           usedCPU,
				CPUTotal:          totalCPU,
				MemoryUsed:        usedMemory,
				MemoryTotal:       totalMemory,
				GPUUsed:           usedGPU,
				GPUTotal:          totalGPU,
			})
		}
	}

	return nil
}

// writeResourceUtilizationToCSV 将资源利用率记录写入 CSV
func writeResourceUtilizationToCSV(path string, records []*ResourceUtilizationRecord) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{
		"batch_number", "task_count", "timestamp",
		"node_id", "node_name",
		"cpu_utilization_pct", "memory_utilization_pct", "gpu_utilization_pct",
		"cpu_used_mc", "cpu_total_mc",
		"memory_used_bytes", "memory_total_bytes",
		"gpu_used", "gpu_total",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, rec := range records {
		row := []string{
			fmt.Sprintf("%d", rec.BatchNumber),
			fmt.Sprintf("%d", rec.TaskCount),
			rec.Timestamp.Format(time.RFC3339Nano),
			rec.NodeID,
			rec.NodeName,
			fmt.Sprintf("%.2f", rec.CPUUtilization),
			fmt.Sprintf("%.2f", rec.MemoryUtilization),
			fmt.Sprintf("%.2f", rec.GPUUtilization),
			fmt.Sprintf("%d", rec.CPUUsed),
			fmt.Sprintf("%d", rec.CPUTotal),
			fmt.Sprintf("%d", rec.MemoryUsed),
			fmt.Sprintf("%d", rec.MemoryTotal),
			fmt.Sprintf("%d", rec.GPUUsed),
			fmt.Sprintf("%d", rec.GPUTotal),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// calculateAggregateMetrics 计算聚合指标
func calculateAggregateMetrics(records []*TaskRecord, totalDuration time.Duration, entryNodeID string, resourceUtilRecords []*ResourceUtilizationRecord, iarnet *bootstrap.Iarnet) *AggregateMetrics {
	metrics := &AggregateMetrics{
		TotalTasks: len(records),
	}

	var responseLatencies []float64
	var fullLatencies []float64

	// 按任务类型统计
	taskTypeMap := make(map[TaskType]*TaskTypeStats)
	taskTypeMap[TaskSmall] = &TaskTypeStats{Type: TaskSmall}
	taskTypeMap[TaskMedium] = &TaskTypeStats{Type: TaskMedium}
	taskTypeMap[TaskLarge] = &TaskTypeStats{Type: TaskLarge}

	// 按节点统计执行任务数
	nodeExecutionMap := make(map[string]*NodeExecutionStats)

	for _, rec := range records {
		if rec == nil {
			continue
		}

		// 统计任务状态
		switch rec.Status {
		case "success":
			metrics.SuccessTasks++
			// 按任务类型统计成功
			if stats, ok := taskTypeMap[rec.Type]; ok {
				stats.SuccessCount++
			}
		case "failed", "timeout", "error":
			// 统一将失败、超时、错误都计入失败任务
			metrics.FailedTasks++
			// 按任务类型统计失败
			if stats, ok := taskTypeMap[rec.Type]; ok {
				stats.FailedCount++
			}
		}

		// 统计任务类型总数
		if stats, ok := taskTypeMap[rec.Type]; ok {
			stats.TotalCount++
		}

		// 统计执行位置
		if rec.Status == "success" {
			if rec.IsCrossNode {
				metrics.CrossNodeExecutionCount++
			} else {
				metrics.LocalExecutionCount++
			}

			// 统计各节点执行任务数及任务类型
			if rec.NodeID != "" {
				if stats, ok := nodeExecutionMap[rec.NodeID]; ok {
					stats.ExecutionCount++
					// 按任务类型统计
					switch rec.Type {
					case TaskSmall:
						stats.SmallCount++
					case TaskMedium:
						stats.MediumCount++
					case TaskLarge:
						stats.LargeCount++
					}
				} else {
					stats := &NodeExecutionStats{
						NodeID:         rec.NodeID,
						NodeName:       rec.NodeID, // 如果没有节点名称，使用 ID
						ExecutionCount: 1,
					}
					// 按任务类型统计
					switch rec.Type {
					case TaskSmall:
						stats.SmallCount = 1
					case TaskMedium:
						stats.MediumCount = 1
					case TaskLarge:
						stats.LargeCount = 1
					}
					nodeExecutionMap[rec.NodeID] = stats
				}
			}
		}

		// 计算时延（仅统计成功任务）
		// 响应时延：只统计部署成功的任务
		if rec.Status == "success" && !rec.SubmitTime.IsZero() && !rec.DeployTime.IsZero() {
			latResp := rec.DeployTime.Sub(rec.SubmitTime).Seconds() * 1000 // 转换为毫秒
			responseLatencies = append(responseLatencies, latResp)
		}

		// 完成时延：只统计成功任务
		if rec.Status == "success" && !rec.SubmitTime.IsZero() && !rec.FinishTime.IsZero() {
			latFull := rec.FinishTime.Sub(rec.SubmitTime).Seconds() * 1000 // 转换为毫秒
			fullLatencies = append(fullLatencies, latFull)
		}
	}

	// 计算各类型任务的成功率和失败率
	metrics.TaskTypeStats = make([]*TaskTypeStats, 0, len(taskTypeMap))
	for _, stats := range taskTypeMap {
		if stats.TotalCount > 0 {
			stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalCount) * 100
			stats.FailureRate = float64(stats.FailedCount) / float64(stats.TotalCount) * 100
		}
		metrics.TaskTypeStats = append(metrics.TaskTypeStats, stats)
	}
	// 按任务类型排序：small, medium, large
	sort.Slice(metrics.TaskTypeStats, func(i, j int) bool {
		order := map[TaskType]int{TaskSmall: 0, TaskMedium: 1, TaskLarge: 2}
		return order[metrics.TaskTypeStats[i].Type] < order[metrics.TaskTypeStats[j].Type]
	})

	// 计算各类型任务在各节点的执行比例
	// 统计各类型任务的总成功数
	smallTotalSuccess := 0
	mediumTotalSuccess := 0
	largeTotalSuccess := 0
	for _, stats := range taskTypeMap {
		switch stats.Type {
		case TaskSmall:
			smallTotalSuccess = stats.SuccessCount
		case TaskMedium:
			mediumTotalSuccess = stats.SuccessCount
		case TaskLarge:
			largeTotalSuccess = stats.SuccessCount
		}
	}

	// 计算各节点执行比例及任务类型比例
	metrics.NodeExecutionStats = make([]*NodeExecutionStats, 0, len(nodeExecutionMap))
	for _, stats := range nodeExecutionMap {
		if metrics.SuccessTasks > 0 {
			stats.ExecutionRate = float64(stats.ExecutionCount) / float64(metrics.SuccessTasks) * 100
		}
		// 计算各类型任务在该节点的执行比例（相对于该类型任务的总成功数）
		if smallTotalSuccess > 0 {
			stats.SmallRate = float64(stats.SmallCount) / float64(smallTotalSuccess) * 100
		}
		if mediumTotalSuccess > 0 {
			stats.MediumRate = float64(stats.MediumCount) / float64(mediumTotalSuccess) * 100
		}
		if largeTotalSuccess > 0 {
			stats.LargeRate = float64(stats.LargeCount) / float64(largeTotalSuccess) * 100
		}
		metrics.NodeExecutionStats = append(metrics.NodeExecutionStats, stats)
	}
	// 按节点名称排序
	sort.Slice(metrics.NodeExecutionStats, func(i, j int) bool {
		return metrics.NodeExecutionStats[i].NodeName < metrics.NodeExecutionStats[j].NodeName
	})

	// 计算时延统计
	if len(responseLatencies) > 0 {
		metrics.AvgResponseLatency = average(responseLatencies)
		metrics.P50ResponseLatency = percentile(responseLatencies, 50)
		metrics.P95ResponseLatency = percentile(responseLatencies, 95)
		metrics.P99ResponseLatency = percentile(responseLatencies, 99)
	}

	if len(fullLatencies) > 0 {
		metrics.AvgFullLatency = average(fullLatencies)
		metrics.P50FullLatency = percentile(fullLatencies, 50)
		metrics.P95FullLatency = percentile(fullLatencies, 95)
		metrics.P99FullLatency = percentile(fullLatencies, 99)
	}

	// 计算吞吐量
	if totalDuration > 0 {
		metrics.Throughput = float64(metrics.SuccessTasks) / totalDuration.Seconds()
	}

	// 计算失败率和执行比例
	if metrics.TotalTasks > 0 {
		metrics.FailureRate = float64(metrics.FailedTasks) / float64(metrics.TotalTasks) * 100
		if metrics.SuccessTasks > 0 {
			metrics.LocalExecutionRate = float64(metrics.LocalExecutionCount) / float64(metrics.SuccessTasks) * 100
			metrics.CrossNodeExecutionRate = float64(metrics.CrossNodeExecutionCount) / float64(metrics.SuccessTasks) * 100
		}
	}

	metrics.TotalDuration = totalDuration

	// 计算各节点的平均资源占用
	metrics.NodeResourceUtilizations = calculateNodeResourceUtilizations(resourceUtilRecords)

	// 更新节点执行统计中的节点名称
	// 优先从 discovery service 获取所有已知节点的信息（包括跨节点）
	nodeNameMap := make(map[string]string)

	// 首先从资源利用率记录中获取节点名称（本地节点）
	for _, nodeUtil := range metrics.NodeResourceUtilizations {
		nodeNameMap[nodeUtil.NodeID] = nodeUtil.NodeName
	}

	// 然后从 discovery service 获取所有已知节点的信息（包括远程节点）
	// 这样可以获取跨节点执行任务的节点名称
	if iarnet != nil && iarnet.DiscoveryService != nil {
		knownNodes := iarnet.DiscoveryService.GetKnownNodes()
		for _, node := range knownNodes {
			if node != nil && node.NodeID != "" && node.NodeName != "" {
				nodeNameMap[node.NodeID] = node.NodeName
			}
		}
		// 也添加本地节点
		if iarnet.ResourceManager != nil {
			localNodeID := iarnet.ResourceManager.GetNodeID()
			if localNodeID != "" {
				// 尝试从已知节点中获取本地节点名称
				for _, node := range knownNodes {
					if node != nil && node.NodeID == localNodeID && node.NodeName != "" {
						nodeNameMap[localNodeID] = node.NodeName
						break
					}
				}
			}
		}
	}

	// 更新节点执行统计中的节点名称
	for _, stats := range metrics.NodeExecutionStats {
		if name, ok := nodeNameMap[stats.NodeID]; ok && name != "" {
			stats.NodeName = name
		}
	}

	return metrics
}

// calculateNodeResourceUtilizations 计算各节点的资源利用率统计
func calculateNodeResourceUtilizations(records []*ResourceUtilizationRecord) []*NodeResourceUtilization {
	if len(records) == 0 {
		return nil
	}

	// 按节点 ID 分组
	nodeMap := make(map[string][]*ResourceUtilizationRecord)
	for _, rec := range records {
		if rec == nil {
			continue
		}
		nodeMap[rec.NodeID] = append(nodeMap[rec.NodeID], rec)
	}

	// 计算每个节点的统计信息
	result := make([]*NodeResourceUtilization, 0, len(nodeMap))
	for nodeID, nodeRecords := range nodeMap {
		if len(nodeRecords) == 0 {
			continue
		}

		var sumCPU, sumMemory, sumGPU float64
		var maxCPU, maxMemory, maxGPU float64
		nodeName := nodeRecords[0].NodeName

		for _, rec := range nodeRecords {
			sumCPU += rec.CPUUtilization
			sumMemory += rec.MemoryUtilization
			sumGPU += rec.GPUUtilization

			if rec.CPUUtilization > maxCPU {
				maxCPU = rec.CPUUtilization
			}
			if rec.MemoryUtilization > maxMemory {
				maxMemory = rec.MemoryUtilization
			}
			if rec.GPUUtilization > maxGPU {
				maxGPU = rec.GPUUtilization
			}
		}

		count := float64(len(nodeRecords))
		result = append(result, &NodeResourceUtilization{
			NodeID:               nodeID,
			NodeName:             nodeName,
			AvgCPUUtilization:    sumCPU / count,
			AvgMemoryUtilization: sumMemory / count,
			AvgGPUUtilization:    sumGPU / count,
			MaxCPUUtilization:    maxCPU,
			MaxMemoryUtilization: maxMemory,
			MaxGPUUtilization:    maxGPU,
			SampleCount:          len(nodeRecords),
		})
	}

	// 按节点名称排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].NodeName < result[j].NodeName
	})

	return result
}

// average 计算平均值
func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// percentile 计算百分位数
func percentile(values []float64, p int) float64 {
	if len(values) == 0 {
		return 0
	}
	// 复制并排序
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	// 计算百分位索引
	index := float64(len(sorted)-1) * float64(p) / 100.0
	lower := int(index)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	// 线性插值
	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

// writeAggregateMetrics 将聚合指标写入文件
func writeAggregateMetrics(metrics *AggregateMetrics, entryNodeID string) error {
	filename := fmt.Sprintf("aggregate_metrics_%s.txt", time.Now().Format("20060102_150405"))
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "=== 实验聚合指标统计 ===\n\n")
	fmt.Fprintf(f, "入口节点 ID: %s\n", entryNodeID)
	fmt.Fprintf(f, "总实验时长: %v\n\n", metrics.TotalDuration)

	fmt.Fprintf(f, "=== 任务统计 ===\n")
	fmt.Fprintf(f, "总任务数: %d\n", metrics.TotalTasks)
	fmt.Fprintf(f, "成功任务数: %d\n", metrics.SuccessTasks)
	fmt.Fprintf(f, "失败任务数: %d\n", metrics.FailedTasks)
	fmt.Fprintf(f, "失败率: %.2f%%\n", metrics.FailureRate)

	// 输出各类型任务统计
	if len(metrics.TaskTypeStats) > 0 {
		fmt.Fprintf(f, "\n各类型任务统计:\n")
		for _, stats := range metrics.TaskTypeStats {
			typeName := string(stats.Type)
			if len(typeName) > 0 {
				typeName = strings.ToUpper(typeName[:1]) + typeName[1:]
			}
			fmt.Fprintf(f, "  %s任务: 总数=%d, 成功=%d (%.2f%%), 失败=%d (%.2f%%)\n",
				typeName,
				stats.TotalCount,
				stats.SuccessCount, stats.SuccessRate,
				stats.FailedCount, stats.FailureRate)
		}
	}
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "=== 执行位置统计 ===\n")
	fmt.Fprintf(f, "本地执行数: %d\n", metrics.LocalExecutionCount)
	fmt.Fprintf(f, "跨节点执行数: %d\n", metrics.CrossNodeExecutionCount)
	fmt.Fprintf(f, "本地执行比例: %.2f%%\n", metrics.LocalExecutionRate)
	fmt.Fprintf(f, "跨节点执行比例: %.2f%%\n", metrics.CrossNodeExecutionRate)

	// 输出各类型任务在各节点的执行统计（按任务类型分组）
	if len(metrics.NodeExecutionStats) > 0 {
		fmt.Fprintf(f, "\n=== 各类型任务在各节点的执行比例 ===\n")
		fmt.Fprintf(f, "（比例基于该类型任务的总成功数，体现本地优先策略和跨域调度框架的作用）\n\n")

		// 小任务在各节点的分布
		fmt.Fprintf(f, "小任务在各节点的执行比例:\n")
		for _, stats := range metrics.NodeExecutionStats {
			if stats.SmallCount > 0 {
				fmt.Fprintf(f, "  %s (%s): %d 个任务, 占比=%.2f%%\n",
					stats.NodeName, stats.NodeID, stats.SmallCount, stats.SmallRate)
			}
		}

		// 中任务在各节点的分布
		fmt.Fprintf(f, "\n中任务在各节点的执行比例:\n")
		for _, stats := range metrics.NodeExecutionStats {
			if stats.MediumCount > 0 {
				fmt.Fprintf(f, "  %s (%s): %d 个任务, 占比=%.2f%%\n",
					stats.NodeName, stats.NodeID, stats.MediumCount, stats.MediumRate)
			}
		}

		// 大任务在各节点的分布
		fmt.Fprintf(f, "\n大任务在各节点的执行比例:\n")
		for _, stats := range metrics.NodeExecutionStats {
			if stats.LargeCount > 0 {
				fmt.Fprintf(f, "  %s (%s): %d 个任务, 占比=%.2f%%\n",
					stats.NodeName, stats.NodeID, stats.LargeCount, stats.LargeRate)
			}
		}
	}
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "=== 响应时延统计（毫秒）===\n")
	fmt.Fprintf(f, "注意: 响应时延仅统计部署成功的任务，失败任务不统计时延\n")
	fmt.Fprintf(f, "平均响应时延: %.2f ms\n", metrics.AvgResponseLatency)
	fmt.Fprintf(f, "P50 响应时延: %.2f ms\n", metrics.P50ResponseLatency)
	fmt.Fprintf(f, "P95 响应时延: %.2f ms\n", metrics.P95ResponseLatency)
	fmt.Fprintf(f, "P99 响应时延: %.2f ms\n\n", metrics.P99ResponseLatency)

	fmt.Fprintf(f, "=== 完成时延统计（毫秒）===\n")
	fmt.Fprintf(f, "注意: 完成时延仅统计部署成功的任务，失败任务不统计时延\n")
	fmt.Fprintf(f, "平均完成时延: %.2f ms\n", metrics.AvgFullLatency)
	fmt.Fprintf(f, "P50 完成时延: %.2f ms\n", metrics.P50FullLatency)
	fmt.Fprintf(f, "P95 完成时延: %.2f ms\n", metrics.P95FullLatency)
	fmt.Fprintf(f, "P99 完成时延: %.2f ms\n\n", metrics.P99FullLatency)

	fmt.Fprintf(f, "=== 系统性能 ===\n")
	fmt.Fprintf(f, "系统吞吐量: %.2f req/s\n\n", metrics.Throughput)

	// 输出各节点资源占用统计
	if len(metrics.NodeResourceUtilizations) > 0 {
		fmt.Fprintf(f, "=== 各节点平均资源占用 ===\n")
		for _, nodeUtil := range metrics.NodeResourceUtilizations {
			fmt.Fprintf(f, "节点: %s (%s)\n", nodeUtil.NodeName, nodeUtil.NodeID)
			fmt.Fprintf(f, "  采样次数: %d\n", nodeUtil.SampleCount)
			fmt.Fprintf(f, "  CPU: 平均 %.2f%%, 最大 %.2f%%\n", nodeUtil.AvgCPUUtilization, nodeUtil.MaxCPUUtilization)
			fmt.Fprintf(f, "  内存: 平均 %.2f%%, 最大 %.2f%%\n", nodeUtil.AvgMemoryUtilization, nodeUtil.MaxMemoryUtilization)
			fmt.Fprintf(f, "  GPU: 平均 %.2f%%, 最大 %.2f%%\n", nodeUtil.AvgGPUUtilization, nodeUtil.MaxGPUUtilization)
			fmt.Fprintf(f, "\n")
		}
	}

	return nil
}

// printAggregateMetrics 打印聚合指标到控制台
func printAggregateMetrics(metrics *AggregateMetrics) {
	logrus.Info("=== 实验聚合指标统计 ===")
	logrus.Infof("总任务数: %d, 成功: %d, 失败: %d",
		metrics.TotalTasks, metrics.SuccessTasks, metrics.FailedTasks)
	logrus.Infof("失败率: %.2f%%", metrics.FailureRate)

	// 输出各类型任务统计
	if len(metrics.TaskTypeStats) > 0 {
		for _, stats := range metrics.TaskTypeStats {
			typeName := string(stats.Type)
			if len(typeName) > 0 {
				typeName = strings.ToUpper(typeName[:1]) + typeName[1:]
			}
			logrus.Infof("%s任务: 总数=%d, 成功=%d (%.2f%%), 失败=%d (%.2f%%)",
				typeName,
				stats.TotalCount,
				stats.SuccessCount, stats.SuccessRate,
				stats.FailedCount, stats.FailureRate)
		}
	}

	logrus.Infof("本地执行: %d (%.2f%%), 跨节点执行: %d (%.2f%%)",
		metrics.LocalExecutionCount, metrics.LocalExecutionRate,
		metrics.CrossNodeExecutionCount, metrics.CrossNodeExecutionRate)

	// 输出各类型任务在各节点的执行统计（按任务类型分组）
	if len(metrics.NodeExecutionStats) > 0 {
		logrus.Info("各类型任务在各节点的执行比例:")
		logrus.Info("（比例基于该类型任务的总成功数，体现本地优先策略和跨域调度框架的作用）")

		// 小任务在各节点的分布
		logrus.Info("小任务在各节点的执行比例:")
		for _, stats := range metrics.NodeExecutionStats {
			if stats.SmallCount > 0 {
				logrus.Infof("  %s (%s): %d 个任务, 占比=%.2f%%",
					stats.NodeName, stats.NodeID, stats.SmallCount, stats.SmallRate)
			}
		}

		// 中任务在各节点的分布
		logrus.Info("中任务在各节点的执行比例:")
		for _, stats := range metrics.NodeExecutionStats {
			if stats.MediumCount > 0 {
				logrus.Infof("  %s (%s): %d 个任务, 占比=%.2f%%",
					stats.NodeName, stats.NodeID, stats.MediumCount, stats.MediumRate)
			}
		}

		// 大任务在各节点的分布
		logrus.Info("大任务在各节点的执行比例:")
		for _, stats := range metrics.NodeExecutionStats {
			if stats.LargeCount > 0 {
				logrus.Infof("  %s (%s): %d 个任务, 占比=%.2f%%",
					stats.NodeName, stats.NodeID, stats.LargeCount, stats.LargeRate)
			}
		}
	}
	logrus.Infof("平均响应时延: %.2f ms (P50: %.2f, P95: %.2f, P99: %.2f ms) [仅统计成功任务]",
		metrics.AvgResponseLatency, metrics.P50ResponseLatency,
		metrics.P95ResponseLatency, metrics.P99ResponseLatency)
	logrus.Infof("平均完成时延: %.2f ms (P50: %.2f, P95: %.2f, P99: %.2f ms) [仅统计成功任务]",
		metrics.AvgFullLatency, metrics.P50FullLatency,
		metrics.P95FullLatency, metrics.P99FullLatency)
	logrus.Infof("系统吞吐量: %.2f req/s", metrics.Throughput)

	// 输出各节点资源占用统计
	if len(metrics.NodeResourceUtilizations) > 0 {
		logrus.Info("=== 各节点平均资源占用 ===")
		for _, nodeUtil := range metrics.NodeResourceUtilizations {
			logrus.Infof("节点 %s (%s): CPU 平均 %.2f%% (最大 %.2f%%), 内存 平均 %.2f%% (最大 %.2f%%), GPU 平均 %.2f%% (最大 %.2f%%) [采样 %d 次]",
				nodeUtil.NodeName, nodeUtil.NodeID,
				nodeUtil.AvgCPUUtilization, nodeUtil.MaxCPUUtilization,
				nodeUtil.AvgMemoryUtilization, nodeUtil.MaxMemoryUtilization,
				nodeUtil.AvgGPUUtilization, nodeUtil.MaxGPUUtilization,
				nodeUtil.SampleCount)
		}
	}
}
