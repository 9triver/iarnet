# 态势感知测试 - Docker Provider

## 测试用例标识

**用例标识**: T3-1-001  
**用例名称**: Docker 资源接入测试

## 测试目的

研究将应用开发后的运行部署过程中的算力调度任务交由应用执行引擎与算力网络内建调度机制协同完成。在实施中，着重解决在执行引擎自主或半自主形成调度决策的情况下，应用所需算力网络资源态势成为决策基准的问题。

这需要深入研究不同基础设施如数据中心云、边缘云的平台特性，包括计算负载状态、通信信道状态、存储吞吐状态等的获取感知方法。通过设计有效的态势感知机制，构建完善的态势模型，形成统一的态势数据集合和序列。这样的实施方案为应用执行引擎提供了基础支持，使其能够更加智能、高效地进行算力调度，适应不同环境下的运行需求。

## 测试范围

本测试用例主要验证 Docker provider 的资源态势感知能力，包括：

1. **资源容量感知** (`GetCapacity`)
   - 总资源容量（CPU、内存、GPU）
   - 已使用资源
   - 可用资源
   - 容量计算的正确性

2. **可用资源感知** (`GetAvailable`)
   - 实时可用资源查询
   - 资源可用性验证

3. **已分配资源感知** (`GetAllocated`)
   - 当前已分配的资源统计
   - 资源分配准确性

4. **健康检查中的资源态势** (`HealthCheck`)
   - 健康检查接口返回的资源容量信息
   - 资源标签信息
   - 资源态势的完整性

5. **资源态势一致性**
   - 不同接口返回的资源信息一致性
   - 资源计算的准确性

6. **资源态势实时性**
   - 资源状态的实时更新能力
   - 接口调用的实时性

## 测试环境要求

### 前置条件

1. **Docker 环境**
   - Docker daemon 必须运行
   - 可以通过 `unix:///var/run/docker.sock` 访问（默认）
   - 或通过 `DOCKER_HOST` 环境变量指定 Docker 主机

2. **Go 环境**
   - Go 1.25.0 或更高版本
   - 已安装项目依赖

3. **测试权限**
   - 需要有权限访问 Docker daemon
   - 需要有权限查询容器信息

### 环境变量

- `DOCKER_HOST`: Docker daemon 地址（可选，默认为 `unix:///var/run/docker.sock`）

## 运行测试

### 运行所有测试

```bash
cd /home/kekwy/cfn-proj/iarnet/test/situation-awareness
go test -v
```

### 运行特定测试

```bash
# 运行资源态势感知测试
go test -v -run TestDockerProvider_ResourceSituationAwareness

# 运行连接状态下的测试
go test -v -run TestDockerProvider_ResourceSituationAwareness_WithConnection
```

### 跳过 Docker 不可用的情况

如果 Docker 不可用，测试会自动跳过：

```bash
go test -v -short
```

## 测试用例说明

### TestDockerProvider_ResourceSituationAwareness

主要的资源态势感知测试，包括以下子测试：

1. **GetCapacity - 获取资源容量信息**
   - 测试未连接状态下获取容量（应该允许）
   - 验证容量信息的完整性（Total、Used、Available）
   - 验证容量计算的正确性
   - 验证资源约束（已使用不超过总资源，可用资源不为负）

2. **GetAvailable - 获取可用资源信息**
   - 测试未连接状态下获取可用资源（应该允许）
   - 验证可用资源不为负数

3. **GetAllocated - 获取已分配资源信息**
   - 测试已分配资源的获取
   - 验证已分配资源不为负数

4. **HealthCheck - 健康检查包含资源态势信息**
   - 测试连接后的健康检查
   - 验证健康检查返回的资源容量信息
   - 验证资源标签信息
   - 验证容量计算的正确性

5. **ResourceSituationConsistency - 资源态势一致性验证**
   - 验证不同接口返回的资源信息一致性
   - 确保 `GetCapacity`、`GetAvailable`、`GetAllocated` 返回的数据一致

6. **ResourceSituationRealTime - 资源态势实时性验证**
   - 验证资源状态能够实时更新
   - 验证接口调用的实时性

### TestDockerProvider_ResourceSituationAwareness_WithConnection

测试连接状态下的资源态势感知，包括：

1. **GetCapacity with ProviderID**
   - 测试连接状态下使用正确的 ProviderID 获取容量

2. **GetAvailable with ProviderID**
   - 测试连接状态下使用正确的 ProviderID 获取可用资源

3. **GetCapacity with wrong ProviderID should fail**
   - 测试使用错误的 ProviderID 应该失败
   - 验证鉴权机制

## 预期结果

### 成功标准

1. 所有测试用例能够成功执行
2. 资源容量信息能够正确获取
3. 资源计算（Total = Used + Available）正确
4. 不同接口返回的资源信息一致
5. 资源态势信息实时更新
6. 鉴权机制正常工作

### 失败处理

如果测试失败，请检查：

1. Docker daemon 是否运行
2. Docker 访问权限是否正确
3. 网络连接是否正常
4. 资源是否足够（CPU、内存）

## 测试数据

测试使用以下配置：

- **Provider ID**: `test-provider-situation-awareness`（未连接状态测试）
- **Provider ID**: `test-provider-connected`（连接状态测试）
- **资源标签**: CPU、Memory

## 注意事项

1. 测试需要实际的 Docker 环境，不能使用 mock
2. 测试可能会受到当前系统资源使用情况的影响
3. 某些测试可能需要等待一小段时间以确保实时性
4. 测试会自动清理资源，但建议在测试前后检查 Docker 容器状态

## 相关文档

- [Docker Provider 实现](../../providers/docker/provider/service.go)
- [Provider Proto 定义](../../internal/proto/resource/provider/provider.proto)
- [资源 Proto 定义](../../internal/proto/resource/resource.proto)

## 问题反馈

如遇到问题，请检查：

1. Docker daemon 日志
2. 测试输出日志
3. 系统资源使用情况
4. 网络连接状态

