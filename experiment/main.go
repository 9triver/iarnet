package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"log"
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

// AggregateMetrics 聚合指标
type AggregateMetrics struct {
	TotalTasks              int
	SuccessTasks            int
	FailedTasks             int // 失败任务数（包含超时和错误）
	LocalExecutionCount     int
	CrossNodeExecutionCount int
	AvgResponseLatency      float64       // 平均响应时延（ms）
	P50ResponseLatency      float64       // P50 响应时延（ms）
	P95ResponseLatency      float64       // P95 响应时延（ms）
	P99ResponseLatency      float64       // P99 响应时延（ms）
	AvgFullLatency          float64       // 平均完成时延（ms）
	P50FullLatency          float64       // P50 完成时延（ms）
	P95FullLatency          float64       // P95 完成时延（ms）
	P99FullLatency          float64       // P99 完成时延（ms）
	Throughput              float64       // 系统吞吐量（req/s）
	FailureRate             float64       // 失败率（百分比）
	LocalExecutionRate      float64       // 本地执行比例（百分比）
	CrossNodeExecutionRate  float64       // 跨节点执行比例（百分比）
	TotalDuration           time.Duration // 总实验时长
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
		TotalTasks:  1000,
		Rate:        10,
		SmallRatio:  0.3,
		MediumRatio: 0.5,
		LargeRatio:  0.2,
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
	// 获取 ResourceManager，用于直接调用 DeployComponent
	resourceManager := iarnet.ResourceManager
	if resourceManager == nil {
		return fmt.Errorf("resource manager is nil")
	}

	// 获取入口节点 ID，用于判断是否跨节点调度
	entryNodeID := resourceManager.GetNodeID()
	logrus.Infof("Experiment started, entry node ID: %s", entryNodeID)

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
			taskType := pickTaskType(id, wcfg)
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
			const retryDelay = 10 * time.Second
			var comp *component.Component
			var err error
			var lastErr error
			var shouldRetry bool

			rec.DispatchTime = time.Now()
			for attempt := 0; attempt < maxRetries; attempt++ {
				// 为每个请求设置上下文超时
				ctx, cancel := context.WithTimeout(context.Background(), wcfg.Timeout)

				// 直接调用 ResourceManager 的 DeployComponent 方法，模拟执行引擎的行为
				comp, err = resourceManager.DeployComponent(ctx, types.RuntimeEnvPython, resourceRequest)
				cancel()

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

				// 等待10秒后重试
				logrus.Debugf("Task %d: Retry attempt %d/%d after %v (error: %v)", id, attempt+1, maxRetries, retryDelay, err)
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

	// 写入任务级别指标 CSV
	if err := writeRecordsToCSV(ecfg.MetricCSV, records); err != nil {
		return fmt.Errorf("write csv: %w", err)
	}

	// 写入资源利用率 CSV
	if ecfg.ResourceUtilizationCSV != "" {
		if err := writeResourceUtilizationToCSV(ecfg.ResourceUtilizationCSV, resourceUtilRecords); err != nil {
			logrus.Warnf("Failed to write resource utilization CSV: %v", err)
		}
	}

	// 计算并输出聚合指标
	metrics := calculateAggregateMetrics(records, totalDuration, entryNodeID)
	if err := writeAggregateMetrics(metrics, entryNodeID); err != nil {
		logrus.Warnf("Failed to write aggregate metrics: %v", err)
	} else {
		printAggregateMetrics(metrics)
	}

	return nil
}

// pickTaskType 根据任务 ID 和比例，生成稳定符合 30/50/20 比例的任务类型。
// 这里使用简单的区间划分，保证总体数量精确满足比例。
func pickTaskType(id int, w WorkloadConfig) TaskType {
	total := w.TotalTasks
	smallCount := int(float64(total) * w.SmallRatio)
	mediumCount := int(float64(total) * w.MediumRatio)

	switch {
	case id < smallCount:
		return TaskSmall
	case id < smallCount+mediumCount:
		return TaskMedium
	default:
		return TaskLarge
	}
}

func cpuForTask(t TaskType) int64 {
	switch t {
	case TaskSmall:
		return 500
	case TaskMedium:
		return 2000
	case TaskLarge:
		return 4000
	default:
		return 500
	}
}

func memForTask(t TaskType) int64 {
	const mb = 1024 * 1024
	switch t {
	case TaskSmall:
		return 256 * mb
	case TaskMedium:
		return 2 * 1024 * mb
	case TaskLarge:
		return 4 * 1024 * mb
	default:
		return 256 * mb
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

// collectResourceUtilization 收集资源利用率数据
func collectResourceUtilization(iarnet *bootstrap.Iarnet, batchNumber int, taskCount int, records *[]*ResourceUtilizationRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 获取本地节点的资源利用率
	if iarnet.SchedulerService != nil {
		resp, err := iarnet.SchedulerService.ListProviders(ctx, true)
		if err != nil {
			return fmt.Errorf("list providers: %w", err)
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

			*records = append(*records, &ResourceUtilizationRecord{
				BatchNumber:       batchNumber,
				TaskCount:         taskCount,
				Timestamp:         time.Now(),
				NodeID:            resp.NodeID,
				NodeName:          resp.NodeName,
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
func calculateAggregateMetrics(records []*TaskRecord, totalDuration time.Duration, entryNodeID string) *AggregateMetrics {
	metrics := &AggregateMetrics{
		TotalTasks: len(records),
	}

	var responseLatencies []float64
	var fullLatencies []float64

	for _, rec := range records {
		if rec == nil {
			continue
		}

		// 统计任务状态
		switch rec.Status {
		case "success":
			metrics.SuccessTasks++
		case "failed", "timeout", "error":
			// 统一将失败、超时、错误都计入失败任务
			metrics.FailedTasks++
		}

		// 统计执行位置
		if rec.Status == "success" {
			if rec.IsCrossNode {
				metrics.CrossNodeExecutionCount++
			} else {
				metrics.LocalExecutionCount++
			}
		}

		// 计算时延
		if !rec.SubmitTime.IsZero() && !rec.DeployTime.IsZero() {
			latResp := rec.DeployTime.Sub(rec.SubmitTime).Seconds() * 1000 // 转换为毫秒
			responseLatencies = append(responseLatencies, latResp)
		}

		if !rec.SubmitTime.IsZero() && !rec.FinishTime.IsZero() {
			latFull := rec.FinishTime.Sub(rec.SubmitTime).Seconds() * 1000 // 转换为毫秒
			fullLatencies = append(fullLatencies, latFull)
		}
	}

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

	return metrics
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
	fmt.Fprintf(f, "失败率: %.2f%%\n\n", metrics.FailureRate)

	fmt.Fprintf(f, "=== 执行位置统计 ===\n")
	fmt.Fprintf(f, "本地执行数: %d\n", metrics.LocalExecutionCount)
	fmt.Fprintf(f, "跨节点执行数: %d\n", metrics.CrossNodeExecutionCount)
	fmt.Fprintf(f, "本地执行比例: %.2f%%\n", metrics.LocalExecutionRate)
	fmt.Fprintf(f, "跨节点执行比例: %.2f%%\n\n", metrics.CrossNodeExecutionRate)

	fmt.Fprintf(f, "=== 响应时延统计（毫秒）===\n")
	fmt.Fprintf(f, "平均响应时延: %.2f ms\n", metrics.AvgResponseLatency)
	fmt.Fprintf(f, "P50 响应时延: %.2f ms\n", metrics.P50ResponseLatency)
	fmt.Fprintf(f, "P95 响应时延: %.2f ms\n", metrics.P95ResponseLatency)
	fmt.Fprintf(f, "P99 响应时延: %.2f ms\n\n", metrics.P99ResponseLatency)

	fmt.Fprintf(f, "=== 完成时延统计（毫秒）===\n")
	fmt.Fprintf(f, "平均完成时延: %.2f ms\n", metrics.AvgFullLatency)
	fmt.Fprintf(f, "P50 完成时延: %.2f ms\n", metrics.P50FullLatency)
	fmt.Fprintf(f, "P95 完成时延: %.2f ms\n", metrics.P95FullLatency)
	fmt.Fprintf(f, "P99 完成时延: %.2f ms\n\n", metrics.P99FullLatency)

	fmt.Fprintf(f, "=== 系统性能 ===\n")
	fmt.Fprintf(f, "系统吞吐量: %.2f req/s\n", metrics.Throughput)

	return nil
}

// printAggregateMetrics 打印聚合指标到控制台
func printAggregateMetrics(metrics *AggregateMetrics) {
	logrus.Info("=== 实验聚合指标统计 ===")
	logrus.Infof("总任务数: %d, 成功: %d, 失败: %d",
		metrics.TotalTasks, metrics.SuccessTasks, metrics.FailedTasks)
	logrus.Infof("失败率: %.2f%%", metrics.FailureRate)
	logrus.Infof("本地执行: %d (%.2f%%), 跨节点执行: %d (%.2f%%)",
		metrics.LocalExecutionCount, metrics.LocalExecutionRate,
		metrics.CrossNodeExecutionCount, metrics.CrossNodeExecutionRate)
	logrus.Infof("平均响应时延: %.2f ms (P50: %.2f, P95: %.2f, P99: %.2f ms)",
		metrics.AvgResponseLatency, metrics.P50ResponseLatency,
		metrics.P95ResponseLatency, metrics.P99ResponseLatency)
	logrus.Infof("平均完成时延: %.2f ms (P50: %.2f, P95: %.2f, P99: %.2f ms)",
		metrics.AvgFullLatency, metrics.P50FullLatency,
		metrics.P95FullLatency, metrics.P99FullLatency)
	logrus.Infof("系统吞吐量: %.2f req/s", metrics.Throughput)
}
