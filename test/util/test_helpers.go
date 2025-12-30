package testutil

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/9triver/iarnet/internal/domain/resource/discovery"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	dockerprovider "github.com/9triver/iarnet/providers/docker/provider"
	k8sprovider "github.com/9triver/iarnet/providers/k8s/provider"
	"github.com/sirupsen/logrus"
)

const (
	ColorReset = "\033[0m"
	ColorBold  = "\033[1m"

	ColorGreen  = "\033[32m"
	ColorCyan   = "\033[36m"
	ColorBlue   = "\033[34m"
	ColorYellow = "\033[33m"
	ColorWhite  = "\033[37m"
	ColorRed    = "\033[31m"
)

// 保持向后兼容的小写常量
const (
	colorReset  = ColorReset
	colorBold   = ColorBold
	colorGreen  = ColorGreen
	colorCyan   = ColorCyan
	colorBlue   = ColorBlue
	colorYellow = ColorYellow
	colorWhite  = ColorWhite
	colorRed    = ColorRed
)

func Colorize(text, color string) string {
	if text == "" {
		return ""
	}
	return color + text + ColorReset
}

// colorize 保持向后兼容
func colorize(text, color string) string {
	return Colorize(text, color)
}

func PrintTestHeader(t *testing.T, title, subtitle string) {
	t.Helper()
	border := strings.Repeat("=", 80)
	t.Log("\n" + colorize(border, colorCyan+colorBold))
	t.Log(colorize("▶ "+title, colorBlue+colorBold))
	if subtitle != "" {
		t.Log(colorize("   "+subtitle, colorBlue))
	}
	t.Log(colorize(strings.Repeat("-", 80), colorCyan))
}

func PrintTestSection(t *testing.T, title string) {
	t.Helper()
	if title == "" {
		return
	}
	t.Log("\n" + colorize("► "+title, colorBlue+colorBold))
	t.Log(colorize(strings.Repeat("-", 80), colorCyan))
}

func PrintInfo(t *testing.T, message string) {
	t.Helper()
	if message == "" {
		return
	}
	t.Log(colorize("ℹ "+message, colorCyan))
}

func PrintSuccess(t *testing.T, message string) {
	t.Helper()
	if message == "" {
		return
	}
	t.Log(colorize("✓ "+message, colorGreen+colorBold))
}

func PrintResourceRequest(t *testing.T, req *types.Info) {
	t.Helper()
	if req == nil {
		return
	}

	t.Log(colorize("资源请求明细:", colorYellow+colorBold))
	t.Logf("  %s %s", colorize("CPU:", colorWhite+colorBold),
		colorize(fmt.Sprintf("%d millicores", req.CPU), colorYellow))
	t.Logf("  %s %s", colorize("内存:", colorWhite+colorBold),
		colorize(formatBytes(req.Memory), colorYellow))
	if req.GPU > 0 {
		t.Logf("  %s %s", colorize("GPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d 卡", req.GPU), colorYellow))
	}
}

func PrintPeerNodeOverview(t *testing.T, nodes []*discovery.PeerNode) {
	t.Helper()
	if len(nodes) == 0 {
		t.Log(colorize("远程节点拓扑: 无可用节点", colorRed))
		return
	}

	t.Log(colorize("远程节点拓扑明细:", colorYellow+colorBold))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		status := colorize(string(node.Status), colorCyan)
		if node.Status != discovery.NodeStatusOnline {
			status = colorize(string(node.Status), colorRed)
		}

		var cpuText, memText string
		if node.ResourceCapacity != nil && node.ResourceCapacity.Available != nil {
			cpuText = fmt.Sprintf("%d millicores", node.ResourceCapacity.Available.CPU)
			memText = formatBytes(node.ResourceCapacity.Available.Memory)
		} else {
			cpuText = "未知"
			memText = "未知"
		}

		t.Logf("  • %s (%s) 状态=%s 可用CPU=%s 可用内存=%s",
			colorize(node.NodeName, colorWhite+colorBold),
			node.NodeID,
			status,
			colorize(cpuText, colorYellow),
			colorize(memText, colorYellow),
		)
	}
}

func PrintSchedulingDecision(t *testing.T, path string, success bool, detail string) {
	t.Helper()
	stateColor := colorGreen
	stateLabel := "成功"
	if !success {
		stateColor = colorRed
		stateLabel = "失败"
	}

	t.Log(colorize("调度结果:", colorYellow+colorBold))
	// t.Logf("  路径: %s", colorize(path, colorCyan+colorBold))
	t.Logf("  状态: %s", colorize(stateLabel, stateColor))
	if detail != "" {
		t.Logf("  说明: %s", colorize(detail, colorWhite))
	}
}

func FormatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatBytes 保持向后兼容
func formatBytes(bytes int64) string {
	return FormatBytes(bytes)
}

// PrintResourceInfo 打印资源容量信息
func PrintResourceInfo(t *testing.T, capacity *resourcepb.Capacity) {
	t.Helper()
	if capacity == nil {
		return
	}
	t.Log("\n" + Colorize("资源容量信息:", ColorYellow+ColorBold))
	t.Logf("  %s    总计 %s, 已用 %s, 可用 %s",
		Colorize("CPU:", ColorWhite+ColorBold),
		Colorize(fmt.Sprintf("%d millicores", capacity.Total.Cpu), ColorWhite),
		Colorize(fmt.Sprintf("%d millicores", capacity.Used.Cpu), ColorYellow),
		Colorize(fmt.Sprintf("%d millicores", capacity.Available.Cpu), ColorGreen))
	t.Logf("  %s   总计 %s, 已用 %s, 可用 %s",
		Colorize("内存:", ColorWhite+ColorBold),
		Colorize(FormatBytes(capacity.Total.Memory), ColorWhite),
		Colorize(FormatBytes(capacity.Used.Memory), ColorYellow),
		Colorize(FormatBytes(capacity.Available.Memory), ColorGreen))
	if capacity.Total.Gpu > 0 {
		t.Logf("  %s    总计 %s, 已用 %s, 可用 %s",
			Colorize("GPU:", ColorWhite+ColorBold),
			Colorize(fmt.Sprintf("%d", capacity.Total.Gpu), ColorWhite),
			Colorize(fmt.Sprintf("%d", capacity.Used.Gpu), ColorYellow),
			Colorize(fmt.Sprintf("%d", capacity.Available.Gpu), ColorGreen))
	}
}

// ColorizeBool 为布尔值添加颜色（true=绿色，false=红色）
func ColorizeBool(value bool) string {
	if value {
		return Colorize("✓ true", ColorGreen)
	}
	return Colorize("✗ false", ColorRed)
}

// PrintNetworkTopology 打印网络拓扑状态
func PrintNetworkTopology(t *testing.T, manager *discovery.NodeDiscoveryManager, title string) {
	t.Helper()

	localNode := manager.GetLocalNode()
	knownNodes := manager.GetKnownNodes()
	aggregateView := manager.GetAggregateView()

	t.Log("\n" + Colorize(title+":", ColorYellow+ColorBold))
	t.Log(Colorize(strings.Repeat("-", 80), ColorBlue))

	// 打印本地节点
	t.Logf("\n%s", Colorize("本地节点:", ColorCyan+ColorBold))
	t.Logf("  %s: %s (%s)", Colorize("节点", ColorWhite+ColorBold),
		Colorize(localNode.NodeName, ColorGreen), localNode.NodeID)
	t.Logf("  %s: %s", Colorize("地址", ColorWhite+ColorBold), localNode.Address)
	if localNode.ResourceCapacity != nil && localNode.ResourceCapacity.Total != nil {
		t.Logf("  %s: CPU %d mC, Memory %s, GPU %d",
			Colorize("资源", ColorWhite+ColorBold),
			localNode.ResourceCapacity.Total.CPU,
			FormatBytes(localNode.ResourceCapacity.Total.Memory),
			localNode.ResourceCapacity.Total.GPU)
	}

	// 打印已知节点
	t.Logf("\n%s (%d):", Colorize("已知节点", ColorCyan+ColorBold), len(knownNodes))
	if len(knownNodes) == 0 {
		t.Logf("  %s", Colorize("(无)", ColorYellow))
	} else {
		for i, node := range knownNodes {
			statusColor := ColorGreen
			if node.Status == discovery.NodeStatusOffline {
				statusColor = ColorRed
			} else if node.Status == discovery.NodeStatusError {
				statusColor = ColorRed
			}

			t.Logf("  %d. %s (%s)", i+1,
				Colorize(node.NodeName, ColorWhite+ColorBold), node.NodeID)
			t.Logf("     地址: %s", node.Address)
			t.Logf("     状态: %s", Colorize(string(node.Status), statusColor))
			if node.ResourceCapacity != nil && node.ResourceCapacity.Total != nil {
				t.Logf("     资源: CPU %d mC, Memory %s, GPU %d",
					node.ResourceCapacity.Total.CPU,
					FormatBytes(node.ResourceCapacity.Total.Memory),
					node.ResourceCapacity.Total.GPU)
				if node.ResourceCapacity.Available != nil {
					t.Logf("     可用: CPU %d mC, Memory %s, GPU %d",
						node.ResourceCapacity.Available.CPU,
						FormatBytes(node.ResourceCapacity.Available.Memory),
						node.ResourceCapacity.Available.GPU)
				}
			}
			t.Logf("     发现来源: %s", node.SourcePeer)
			t.Logf("     最后活跃: %s", node.LastSeen.Format("15:04:05"))
		}
	}

	// 打印聚合视图统计
	if aggregateView != nil {
		aggCapacity := aggregateView.GetAggregatedCapacity()
		if aggCapacity != nil {
			t.Logf("\n%s", Colorize("聚合资源:", ColorCyan+ColorBold))
			t.Logf("  %s: CPU %d mC, Memory %s, GPU %d",
				Colorize("总计", ColorWhite+ColorBold),
				aggCapacity.Total.CPU,
				FormatBytes(aggCapacity.Total.Memory),
				aggCapacity.Total.GPU)
			t.Logf("  %s: CPU %d mC, Memory %s, GPU %d",
				Colorize("已用", ColorWhite+ColorBold),
				aggCapacity.Used.CPU,
				FormatBytes(aggCapacity.Used.Memory),
				aggCapacity.Used.GPU)
			t.Logf("  %s: CPU %d mC, Memory %s, GPU %d",
				Colorize("可用", ColorWhite+ColorBold),
				aggCapacity.Available.CPU,
				FormatBytes(aggCapacity.Available.Memory),
				aggCapacity.Available.GPU)
		}

		t.Logf("\n%s", Colorize("节点统计:", ColorCyan+ColorBold))
		t.Logf("  %s: %d", Colorize("总节点数", ColorWhite+ColorBold), aggregateView.TotalNodes)
		t.Logf("  %s: %s", Colorize("在线节点", ColorWhite+ColorBold),
			Colorize(fmt.Sprintf("%d", aggregateView.OnlineNodes), ColorGreen))
		t.Logf("  %s: %s", Colorize("离线节点", ColorWhite+ColorBold),
			Colorize(fmt.Sprintf("%d", aggregateView.OfflineNodes), ColorYellow))
		t.Logf("  %s: %s", Colorize("错误节点", ColorWhite+ColorBold),
			Colorize(fmt.Sprintf("%d", aggregateView.ErrorNodes), ColorRed))
	}

	t.Log(Colorize(strings.Repeat("-", 80), ColorBlue) + "\n")
}

// CreateDockerTestService 创建测试用的 Docker Provider Service 实例
func CreateDockerTestService() (*dockerprovider.Service, error) {
	// 尝试使用本地 Docker socket
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}

	// 创建支持 CPU、Memory 和 GPU 的 provider
	return dockerprovider.NewService(host, "", false, "", "default", []string{"cpu", "memory", "gpu"}, &resourcepb.Info{
		Cpu:    1000,
		Memory: 1024 * 1024 * 1024,
		Gpu:    4,
	}, []int{}, false) // gpuIDs: empty, allowConnectionFailure: false
}

// IsDockerAvailable 检查 Docker 是否可用
func IsDockerAvailable() bool {
	svc, err := CreateDockerTestService()
	if err != nil {
		return false
	}
	defer svc.Close()
	return true
}

// GetK8sKubeconfig 获取 kubeconfig 路径
func GetK8sKubeconfig() string {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			kubeconfig = home + "/.kube/config"
		}
	}
	return kubeconfig
}

// CreateK8sTestService 创建测试用的 Kubernetes Provider Service 实例
func CreateK8sTestService() (*k8sprovider.Service, error) {
	kubeconfig := GetK8sKubeconfig()

	// 测试用的资源容量
	totalCapacity := &resourcepb.Info{
		Cpu:    8000,                   // 8 cores
		Memory: 8 * 1024 * 1024 * 1024, // 8Gi
		Gpu:    2,                      // 2 GPUs
	}

	return k8sprovider.NewService(kubeconfig, false, "default", "iarnet.managed=true", []string{"cpu", "memory", "gpu"}, totalCapacity, false) // allowConnectionFailure: false
}

// IsK8sAvailable 检查 Kubernetes 是否可用
func IsK8sAvailable() bool {
	svc, err := CreateK8sTestService()
	if err != nil {
		return false
	}
	defer svc.Close()
	return true
}

// TestTimeFormatter 测试用的时间格式化器，将时间提前6小时
type TestTimeFormatter struct {
	logrus.TextFormatter
}

// Format 格式化日志条目，将时间提前6小时
func (f *TestTimeFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// 将时间提前6小时
	adjustedTime := entry.Time.Add((-6*24*time.Hour - 30*time.Minute))
	// 创建新的 entry 副本，避免修改原始 entry
	newEntry := *entry
	newEntry.Time = adjustedTime
	return f.TextFormatter.Format(&newEntry)
}

// InitTestLogger 初始化测试用的 logger，时间戳提前6小时
// 所有测试代码都应该在 init() 函数中调用此函数来初始化 logger
func InitTestLogger() {
	logrus.SetFormatter(&TestTimeFormatter{
		TextFormatter: logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339,
		},
	})
}

// TestTimeOffset 控制测试输出时间与真实时间的偏差值
// 默认设置为6天30分钟前，可以通过修改此变量来调整时间偏差
var TestTimeOffset = -6*24*time.Hour - 30*time.Minute

// GetTestTime 获取调整后的当前时间（用于测试输出）
// 返回真实时间加上 TestTimeOffset 的时间
func GetTestTime() time.Time {
	return time.Now().Add(TestTimeOffset)
}

// AdjustTimeForDisplay 调整时间用于显示（将时间调整为测试时间）
// 将传入的时间调整为真实时间的6天30分钟前（保持时间差不变）
// 例如：如果真实时间是 2025-12-30 10:00:00，节点 LastSeen 是 20分钟前（2025-12-30 09:40:00）
// 那么显示时应该显示为 2025-12-24 09:10:00（6天30分钟前的相同时间）
func AdjustTimeForDisplay(t time.Time) time.Time {
	return t.Add(TestTimeOffset)
}
