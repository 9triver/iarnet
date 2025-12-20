package policy

import (
	"fmt"

	"github.com/9triver/iarnet/internal/domain/resource/types"
	"github.com/sirupsen/logrus"
)

// Decision 策略决策结果
type Decision int

const (
	DecisionAccept Decision = iota // 接受
	DecisionReject                 // 拒绝
)

// Result 策略评估结果
type Result struct {
	Decision Decision
	Reason   string
	Policy   string // 策略名称
}

// Context 策略评估上下文
type Context struct {
	// 来自远端的调度结果
	NodeID     string
	NodeName   string
	ProviderID string
	Available  *types.Info

	// 调度请求本身
	Request    *types.Info
	RuntimeEnv types.RuntimeEnv

	// 本地节点信息（可选）
	LocalNodeID   string
	LocalDomainID string
}

// Policy 调度策略接口
type Policy interface {
	// Name 返回策略名称
	Name() string
	// Evaluate 评估调度结果，返回决策和原因
	Evaluate(ctx *Context) Result
}

// Chain 策略链，按顺序执行多个策略
type Chain struct {
	policies []Policy
}

// NewChain 创建策略链
func NewChain(policies ...Policy) *Chain {
	return &Chain{
		policies: policies,
	}
}

// Evaluate 执行所有策略，遇到第一个拒绝即返回
func (c *Chain) Evaluate(ctx *Context) Result {
	for _, policy := range c.policies {
		result := policy.Evaluate(ctx)
		if result.Decision == DecisionReject {
			logrus.Debugf("Policy %s rejected schedule: %s", result.Policy, result.Reason)
			return result
		}
	}
	return Result{
		Decision: DecisionAccept,
		Reason:   "all policies passed",
		Policy:   "chain",
	}
}

// AddPolicy 添加策略到链中
func (c *Chain) AddPolicy(policy Policy) {
	c.policies = append(c.policies, policy)
}

// PolicyConfig 策略配置（用于从配置文件加载）
type PolicyConfig struct {
	Type   string                 `yaml:"type"`   // 策略类型：resource_safety_margin, node_blacklist
	Enable bool                   `yaml:"enable"` // 是否启用
	Params map[string]interface{} `yaml:"params"` // 策略参数
}

// Factory 策略工厂，根据配置创建策略实例
type Factory struct{}

// NewFactory 创建策略工厂
func NewFactory() *Factory {
	return &Factory{}
}

// CreatePolicy 根据配置创建策略实例
func (f *Factory) CreatePolicy(cfg PolicyConfig) (Policy, error) {
	if !cfg.Enable {
		return nil, fmt.Errorf("policy %s is disabled", cfg.Type)
	}

	switch cfg.Type {
	case "resource_safety_margin":
		return f.createResourceSafetyMarginPolicy(cfg.Params)
	case "node_blacklist":
		return f.createNodeBlacklistPolicy(cfg.Params)
	case "provider_blacklist":
		return f.createProviderBlacklistPolicy(cfg.Params)
	default:
		return nil, fmt.Errorf("unknown policy type: %s", cfg.Type)
	}
}

// CreateChain 根据配置列表创建策略链
func (f *Factory) CreateChain(configs []PolicyConfig) (*Chain, error) {
	chain := NewChain()
	for _, cfg := range configs {
		if !cfg.Enable {
			logrus.Debugf("Skipping disabled policy: %s", cfg.Type)
			continue
		}
		policy, err := f.CreatePolicy(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create policy %s: %w", cfg.Type, err)
		}
		chain.AddPolicy(policy)
		logrus.Infof("Added policy to chain: %s", policy.Name())
	}
	return chain, nil
}

// createResourceSafetyMarginPolicy 创建资源安全裕度策略
func (f *Factory) createResourceSafetyMarginPolicy(params map[string]interface{}) (Policy, error) {
	cpuRatio := 1.2
	memoryRatio := 1.2
	gpuRatio := 1.0

	if params != nil {
		if v, ok := params["cpu_ratio"].(float64); ok {
			cpuRatio = v
		}
		if v, ok := params["memory_ratio"].(float64); ok {
			memoryRatio = v
		}
		if v, ok := params["gpu_ratio"].(float64); ok {
			gpuRatio = v
		}
	}

	return NewResourceSafetyMarginPolicy(cpuRatio, memoryRatio, gpuRatio), nil
}

// createNodeBlacklistPolicy 创建节点黑名单策略
func (f *Factory) createNodeBlacklistPolicy(params map[string]interface{}) (Policy, error) {
	var nodeIDs []string
	if params != nil {
		if v, ok := params["node_ids"].([]interface{}); ok {
			nodeIDs = make([]string, 0, len(v))
			for _, id := range v {
				if str, ok := id.(string); ok {
					nodeIDs = append(nodeIDs, str)
				}
			}
		}
	}
	return NewNodeBlacklistPolicy(nodeIDs), nil
}

// createProviderBlacklistPolicy 创建 Provider 黑名单策略
func (f *Factory) createProviderBlacklistPolicy(params map[string]interface{}) (Policy, error) {
	var providerIDs []string
	if params != nil {
		if v, ok := params["provider_ids"].([]interface{}); ok {
			providerIDs = make([]string, 0, len(v))
			for _, id := range v {
				if str, ok := id.(string); ok {
					providerIDs = append(providerIDs, str)
				}
			}
		}
	}
	return NewProviderBlacklistPolicy(providerIDs), nil
}
