package policy

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// NodeBlacklistPolicy 节点黑名单策略
// 拒绝来自黑名单中节点的调度结果
type NodeBlacklistPolicy struct {
	blacklist map[string]bool // node_id -> true
}

// NewNodeBlacklistPolicy 创建节点黑名单策略
func NewNodeBlacklistPolicy(nodeIDs []string) *NodeBlacklistPolicy {
	blacklist := make(map[string]bool)
	for _, id := range nodeIDs {
		if id != "" {
			blacklist[strings.TrimSpace(id)] = true
		}
	}
	logrus.Infof("NodeBlacklistPolicy initialized with %d nodes", len(blacklist))
	return &NodeBlacklistPolicy{
		blacklist: blacklist,
	}
}

// Name 返回策略名称
func (p *NodeBlacklistPolicy) Name() string {
	return "node_blacklist"
}

// Evaluate 评估调度结果
func (p *NodeBlacklistPolicy) Evaluate(ctx *Context) Result {
	if ctx.NodeID == "" {
		return Result{
			Decision: DecisionAccept,
			Reason:   "node ID is empty, skipping blacklist check",
			Policy:   p.Name(),
		}
	}

	if p.blacklist[ctx.NodeID] {
		return Result{
			Decision: DecisionReject,
			Reason:   fmt.Sprintf("node %s (%s) is in blacklist", ctx.NodeID, ctx.NodeName),
			Policy:   p.Name(),
		}
	}

	return Result{
		Decision: DecisionAccept,
		Reason:   "node not in blacklist",
		Policy:   p.Name(),
	}
}

