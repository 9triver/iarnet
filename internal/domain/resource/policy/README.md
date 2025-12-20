# 调度策略系统

调度策略系统用于在跨节点委托调度时，对远程节点返回的调度结果进行二次评估和筛选，支持拒绝不符合策略要求的调度方案。

## 配置示例

在 `config.yaml` 的 `resource` 部分添加 `schedule_policies` 配置：

```yaml
resource:
  # ... 其他配置 ...
  schedule_policies:
    # 资源安全裕度策略：要求可用资源至少是请求资源的指定倍数
    - type: resource_safety_margin
      enable: true
      params:
        cpu_ratio: 1.2      # CPU 安全系数，默认 1.2（要求可用 >= 请求 * 1.2）
        memory_ratio: 1.2   # 内存安全系数，默认 1.2
        gpu_ratio: 1.0      # GPU 安全系数，默认 1.0（不放大）
    
    # 节点黑名单策略：拒绝来自黑名单节点的调度结果
    - type: node_blacklist
      enable: true
      params:
        node_ids:
          - "node.xxx"       # 要拒绝的节点 ID 列表
          - "node.yyy"
    
    # Provider 黑名单策略：拒绝来自黑名单 Provider 的调度结果
    - type: provider_blacklist
      enable: true
      params:
        provider_ids:
          - "prov.xxx"       # 要拒绝的 Provider ID 列表
          - "prov.yyy"
```

## 策略类型

### 1. resource_safety_margin（资源安全裕度策略）

确保可用资源有足够的安全裕度，避免资源用满导致的问题。

**参数：**
- `cpu_ratio` (float64, 可选): CPU 安全系数，默认 1.2
- `memory_ratio` (float64, 可选): 内存安全系数，默认 1.2
- `gpu_ratio` (float64, 可选): GPU 安全系数，默认 1.0

**示例：**
```yaml
- type: resource_safety_margin
  enable: true
  params:
    cpu_ratio: 1.5      # 要求可用 CPU >= 请求 * 1.5
    memory_ratio: 1.3   # 要求可用内存 >= 请求 * 1.3
    gpu_ratio: 1.0      # GPU 不放大
```

### 2. node_blacklist（节点黑名单策略）

拒绝来自指定节点的调度结果。

**参数：**
- `node_ids` ([]string): 要拒绝的节点 ID 列表

**示例：**
```yaml
- type: node_blacklist
  enable: true
  params:
    node_ids:
      - "node.unstable"
      - "node.maintenance"
```

### 3. provider_blacklist（Provider 黑名单策略）

拒绝来自指定 Provider 的调度结果。

**参数：**
- `provider_ids` ([]string): 要拒绝的 Provider ID 列表

**示例：**
```yaml
- type: provider_blacklist
  enable: true
  params:
    provider_ids:
      - "prov.failed"
      - "prov.testing"
```

## 使用方式

策略链会在 `bootstrap` 阶段根据配置文件自动初始化，并在 `Manager.EvaluateLocalScheduleSafety` 方法中自动执行。

当本地 iarnet 调用远程节点的 `ProposeLocalSchedule` 获取调度结果后，调用 `EvaluateLocalScheduleSafety` 进行评估：

```go
ok, reason := resourceManager.EvaluateLocalScheduleSafety(resourceRequest, result)
if !ok {
    logrus.Warnf("Schedule rejected: %s", reason)
    // 拒绝该调度结果，可以尝试其他节点或返回错误
} else {
    // 通过策略校验，可以继续部署
}
```

## 扩展新策略

要添加新策略，需要：

1. 实现 `policy.Policy` 接口：
```go
type MyPolicy struct {
    // 策略参数
}

func (p *MyPolicy) Name() string {
    return "my_policy"
}

func (p *MyPolicy) Evaluate(ctx *Context) Result {
    // 评估逻辑
    if shouldReject {
        return Result{
            Decision: DecisionReject,
            Reason:   "rejection reason",
            Policy:   p.Name(),
        }
    }
    return Result{
        Decision: DecisionAccept,
        Reason:   "passed",
        Policy:   p.Name(),
    }
}
```

2. 在 `policy.Factory.CreatePolicy` 中添加策略创建逻辑

3. 在配置文件中使用新策略类型

