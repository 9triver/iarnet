package provider

import "context"

// DeploymentEnvOverride 描述一次部署需要使用的上游通信地址
type DeploymentEnvOverride struct {
	ZMQAddress    string
	StoreAddress  string
	LoggerAddress string
}

type envOverrideCtxKey struct{}

// WithDeploymentEnvOverride 在 context 中附加上游地址覆盖
func WithDeploymentEnvOverride(ctx context.Context, override *DeploymentEnvOverride) context.Context {
	if override == nil {
		return ctx
	}
	return context.WithValue(ctx, envOverrideCtxKey{}, override)
}

// GetDeploymentEnvOverride 从 context 获取上游地址覆盖
func GetDeploymentEnvOverride(ctx context.Context) (*DeploymentEnvOverride, bool) {
	val := ctx.Value(envOverrideCtxKey{})
	if val == nil {
		return nil, false
	}
	override, ok := val.(*DeploymentEnvOverride)
	return override, ok
}
