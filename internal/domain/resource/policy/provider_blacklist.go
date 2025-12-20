package policy

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// ProviderBlacklistPolicy Provider 黑名单策略
// 拒绝来自黑名单中 Provider 的调度结果
type ProviderBlacklistPolicy struct {
	blacklist map[string]bool // provider_id -> true
}

// NewProviderBlacklistPolicy 创建 Provider 黑名单策略
func NewProviderBlacklistPolicy(providerIDs []string) *ProviderBlacklistPolicy {
	blacklist := make(map[string]bool)
	for _, id := range providerIDs {
		if id != "" {
			blacklist[strings.TrimSpace(id)] = true
		}
	}
	logrus.Infof("ProviderBlacklistPolicy initialized with %d providers", len(blacklist))
	return &ProviderBlacklistPolicy{
		blacklist: blacklist,
	}
}

// Name 返回策略名称
func (p *ProviderBlacklistPolicy) Name() string {
	return "provider_blacklist"
}

// Evaluate 评估调度结果
func (p *ProviderBlacklistPolicy) Evaluate(ctx *Context) Result {
	if ctx.ProviderID == "" {
		return Result{
			Decision: DecisionAccept,
			Reason:   "provider ID is empty, skipping blacklist check",
			Policy:   p.Name(),
		}
	}

	if p.blacklist[ctx.ProviderID] {
		return Result{
			Decision: DecisionReject,
			Reason:   fmt.Sprintf("provider %s is in blacklist", ctx.ProviderID),
			Policy:   p.Name(),
		}
	}

	return Result{
		Decision: DecisionAccept,
		Reason:   "provider not in blacklist",
		Policy:   p.Name(),
	}
}
