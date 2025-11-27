package testutil

import (
	"fmt"
	"strings"
	"testing"

	"github.com/9triver/iarnet/internal/domain/resource/discovery"
	"github.com/9triver/iarnet/internal/domain/resource/types"
)

const (
	colorReset = "\033[0m"
	colorBold  = "\033[1m"

	colorGreen  = "\033[32m"
	colorCyan   = "\033[36m"
	colorBlue   = "\033[34m"
	colorYellow = "\033[33m"
	colorWhite  = "\033[37m"
	colorRed    = "\033[31m"
)

func colorize(text, color string) string {
	if text == "" {
		return ""
	}
	return color + text + colorReset
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

func formatBytes(bytes int64) string {
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
