package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/9triver/iarnet/internal/bootstrap"
	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/domain/application/types"
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

// TaskRecord 用于记录单个任务的关键指标，便于后续按“已提交任务数”做统计与绘图
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
}

// WorkloadConfig 描述实验中使用的任务负载分布
type WorkloadConfig struct {
	TotalTasks int
	Rate       float64 // 请求速率，req/s

	SmallRatio  float64
	MediumRatio float64
	LargeRatio  float64

	// 执行时延模拟区间（毫秒）
	SmallDurationMinMs  int
	SmallDurationMaxMs  int
	MediumDurationMinMs int
	MediumDurationMaxMs int
	LargeDurationMinMs  int
	LargeDurationMaxMs  int

	// 调度请求超时时间
	Timeout time.Duration
}

// ExperimentConfig 描述一次完整实验运行所需的配置
type ExperimentConfig struct {
	ConfigPath string // iarnet 配置文件路径，用于初始化 ResourceManager

	// 逻辑批大小：每提交 B 个任务做一次资源利用统计
	BatchSize int

	// 输出 CSV 路径，用于后续绘制图表
	MetricCSV string
}

func main() {
	// 基本参数可通过命令行指定，方便在不同 N 和不同集群规模下复用
	configPath := flag.String("config", "config.yaml", "path to iarnet config file")
	totalTasks := flag.Int("total", 10000, "total tasks to submit in one run")
	rate := flag.Float64("rate", 50, "submit rate (requests per second)")
	timeoutSec := flag.Int("timeout", 5, "request timeout in seconds")
	batchSize := flag.Int("batch", 100, "batch size for per-batch statistics")
	outputCSV := flag.String("out", "experiment_results.csv", "csv file to write task metrics")
	flag.Parse()

	util.InitLogger()

	wcfg := WorkloadConfig{
		TotalTasks: *totalTasks,
		Rate:       *rate,

		SmallRatio:  0.3,
		MediumRatio: 0.5,
		LargeRatio:  0.2,

		SmallDurationMinMs:  50,
		SmallDurationMaxMs:  200,
		MediumDurationMinMs: 200,
		MediumDurationMaxMs: 800,
		LargeDurationMinMs:  800,
		LargeDurationMaxMs:  2000,

		Timeout: time.Duration(*timeoutSec) * time.Second,
	}

	ecfg := ExperimentConfig{
		ConfigPath: *configPath,
		BatchSize:  *batchSize,
		MetricCSV:  *outputCSV,
	}

	if err := runExperiment(ecfg, wcfg); err != nil {
		fmt.Fprintf(os.Stderr, "experiment failed: %v\n", err)
		os.Exit(1)
	}
}

// runExperiment 只负责"一次 N 固定"的实验运行。
// 不同节点数 N 的对比，可通过多次运行（指向不同规模的集群）并离线汇总结果完成。
// 实验代码模拟执行引擎的行为，直接调用 ResourceManager 的 DeployComponent 方法。
func runExperiment(ecfg ExperimentConfig, wcfg WorkloadConfig) error {
	// 加载配置文件并初始化 iarnet（包括 ResourceManager）
	cfg, err := config.LoadConfig(ecfg.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	iarnet, err := bootstrap.Initialize(cfg)
	if err != nil {
		return fmt.Errorf("initialize iarnet: %w", err)
	}
	defer iarnet.Stop()

	// 启动服务（包括 discovery、scheduler 等）
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := iarnet.Start(ctx); err != nil {
		return fmt.Errorf("start iarnet: %w", err)
	}

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

	for taskID < wcfg.TotalTasks {
		<-ticker.C

		go func(id int) {
			defer wg.Done()
			rec := &TaskRecord{ID: id}
			taskType := pickTaskType(id, wcfg)
			rec.Type = taskType
			rec.SubmitTime = time.Now()

			// 为每个请求设置上下文超时
			ctx, cancel := context.WithTimeout(context.Background(), wcfg.Timeout)
			defer cancel()

			// 构造资源请求（模拟执行引擎向 resource manager 提交任务）
			resourceRequest := &types.Info{
				CPU:    cpuForTask(taskType),
				Memory: memForTask(taskType),
				GPU:    gpuForTask(taskType),
			}

			rec.DispatchTime = time.Now()
			// 直接调用 ResourceManager 的 DeployComponent 方法，模拟执行引擎的行为
			comp, err := resourceManager.DeployComponent(ctx, types.RuntimeEnvPython, resourceRequest)

			// 记录 DeployTime / Status
			rec.DeployTime = time.Now()

			if err != nil {
				// 调度失败
				if ctx.Err() == context.DeadlineExceeded {
					rec.Status = "timeout"
				} else {
					rec.Status = "error"
				}
				rec.Error = err.Error()
				records[id] = rec
				return
			}

			if comp == nil {
				rec.Status = "error"
				rec.Error = "component is nil"
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

			// 模拟任务执行完成时间（仅用于统计完整时延，不影响 Provider 内部资源释放逻辑）
			execDuration := randomDurationForTask(taskType, wcfg)
			time.Sleep(execDuration)

			rec.FinishTime = time.Now()
			// 如果在整个过程里 ctx 超时，也计为 timeout
			if ctx.Err() == context.DeadlineExceeded {
				rec.Status = "timeout"
			} else {
				rec.Status = "success"
			}

			records[id] = rec
		}(taskID)

		taskID++
	}

	wg.Wait()
	endTime := time.Now()
	logrus.Infof("Experiment finished, total duration: %v", endTime.Sub(startTime))

	// TODO: 此处可以钩入一次"批量资源利用统计"的接口，
	// 从各个 iarnet 节点暴露的 ListProviders / ListRemoteProviders 中聚合 Capacity.Used/Total，
	// 写入单独的 CSV 文件。这里仅实现任务级别的指标输出框架。

	if err := writeRecordsToCSV(ecfg.MetricCSV, records); err != nil {
		return fmt.Errorf("write csv: %w", err)
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

// randomDurationForTask 生成任务执行时延的随机值，在指定区间内均匀分布
func randomDurationForTask(t TaskType, w WorkloadConfig) time.Duration {
	var minMs, maxMs int
	switch t {
	case TaskSmall:
		minMs, maxMs = w.SmallDurationMinMs, w.SmallDurationMaxMs
	case TaskMedium:
		minMs, maxMs = w.MediumDurationMinMs, w.MediumDurationMaxMs
	case TaskLarge:
		minMs, maxMs = w.LargeDurationMinMs, w.LargeDurationMaxMs
	default:
		minMs, maxMs = 100, 200
	}
	if maxMs <= minMs {
		return time.Duration(minMs) * time.Millisecond
	}
	// 生成 [minMs, maxMs] 区间的随机值
	delta := maxMs - minMs
	randomMs := minMs + rand.Intn(delta+1)
	return time.Duration(randomMs) * time.Millisecond
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
		"id", "type", "status", "error",
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
