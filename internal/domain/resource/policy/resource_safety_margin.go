package policy

import (
	"fmt"
)

// ResourceSafetyMarginPolicy 资源安全裕度策略
// 要求可用资源至少是请求资源的指定倍数（默认 1.2 倍）
type ResourceSafetyMarginPolicy struct {
	cpuRatio    float64
	memoryRatio float64
	gpuRatio    float64
}

// NewResourceSafetyMarginPolicy 创建资源安全裕度策略
func NewResourceSafetyMarginPolicy(cpuRatio, memoryRatio, gpuRatio float64) *ResourceSafetyMarginPolicy {
	if cpuRatio <= 0 {
		cpuRatio = 1.2 // 默认 1.2 倍
	}
	if memoryRatio <= 0 {
		memoryRatio = 1.2
	}
	if gpuRatio <= 0 {
		gpuRatio = 1.0 // GPU 默认不放大
	}
	return &ResourceSafetyMarginPolicy{
		cpuRatio:    cpuRatio,
		memoryRatio: memoryRatio,
		gpuRatio:    gpuRatio,
	}
}

// Name 返回策略名称
func (p *ResourceSafetyMarginPolicy) Name() string {
	return "resource_safety_margin"
}

// Evaluate 评估调度结果
func (p *ResourceSafetyMarginPolicy) Evaluate(ctx *Context) Result {
	if ctx.Request == nil || ctx.Available == nil {
		return Result{
			Decision: DecisionReject,
			Reason:   "request or available resources is nil",
			Policy:   p.Name(),
		}
	}

	req := ctx.Request
	avail := ctx.Available

	// CPU 校验
	if req.CPU > 0 {
		required := int64(float64(req.CPU) * p.cpuRatio)
		if avail.CPU < required {
			return Result{
				Decision: DecisionReject,
				Reason:   fmt.Sprintf("insufficient CPU margin: required %d (request %d * ratio %.2f), available %d", required, req.CPU, p.cpuRatio, avail.CPU),
				Policy:   p.Name(),
			}
		}
	}

	// Memory 校验
	if req.Memory > 0 {
		required := int64(float64(req.Memory) * p.memoryRatio)
		if avail.Memory < required {
			return Result{
				Decision: DecisionReject,
				Reason:   fmt.Sprintf("insufficient memory margin: required %d (request %d * ratio %.2f), available %d", required, req.Memory, p.memoryRatio, avail.Memory),
				Policy:   p.Name(),
			}
		}
	}

	// GPU 校验
	if req.GPU > 0 {
		required := int64(float64(req.GPU) * p.gpuRatio)
		if avail.GPU < required {
			return Result{
				Decision: DecisionReject,
				Reason:   fmt.Sprintf("insufficient GPU: required %d (request %d * ratio %.2f), available %d", required, req.GPU, p.gpuRatio, avail.GPU),
				Policy:   p.Name(),
			}
		}
	}

	return Result{
		Decision: DecisionAccept,
		Reason:   "resource safety margin check passed",
		Policy:   p.Name(),
	}
}
