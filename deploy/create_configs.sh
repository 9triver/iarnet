#!/bin/bash
# 为每个 iarnet 实例创建配置文件

BASE_CONFIG="/home/zhangyx/iarnet/config.yaml"

for i in {4..10}; do
  NODE_NUM=$i
  HTTP_PORT=$((8082 + i))
  NODE_NAME="node.$i"
  NODE_DESC="node.$i description"
  
  # 创建配置文件
  cat > "iarnet/i$i/config.yaml" << YAML
host: "172.30.0.$((9 + i))"

super_admin:
  name: "admin"
  password: "123456"

peer_listen_addr: ":50051"
initial_peers: []

data_dir: "./data"

application:
  workspace_dir: "../workspaces"
  runner_images:
    "python:3.11-latest": "iarnet/runner:python_3.11-latest"

resource:
  peer_port: 50051
  schedule_port: 50051
  global_registry_addr: "localhost:50010"
  name: "$NODE_NAME"
  description: "$NODE_DESC"
  domain_id: "domain.nwwNPjSgUFM9DCv74J8LbM"
  component_images:
    "python": "iarnet/component:python_3.11-latest"
  discovery:
    enabled: true
    gossip_interval_seconds: 30
    node_ttl_seconds: 180
    max_gossip_peers: 10
    max_hops: 5
    query_timeout_seconds: 5
    fanout: 3
    use_anti_entropy: true
    anti_entropy_interval_seconds: 300
  schedule_policies:
    - type: resource_safety_margin
      enable: true
      params:
        cpu_ratio: 1.2
        memory_ratio: 1.2
        gpu_ratio: 1.0
    - type: node_blacklist
      enable: false
      params:
        node_ids: []
    - type: provider_blacklist
      enable: false
      params:
        provider_ids: []

enable_local_docker: true

database:
  application_db_path: "./data/application.db"
  resource_provider_db_path: "./data/resource_provider.db"
  resource_logger_db_path: "./data/resource_logger.db"
  max_open_conns: 10
  max_idle_conns: 5
  conn_max_lifetime_seconds: 300

transport:
  http:
    port: $HTTP_PORT
  zmq:
    port: 5555
  rpc:
    resource:
      port: 50051
    ignis:
      port: 50001
    store:
      port: 50002
    logger:
      port: 50003
    resource_logger:
      port: 50004
    discovery:
      port: 50005
    scheduler:
      port: 50006

logging:
  enabled: true
  data_dir: "./data/logs"
  db_path: "./data/logs.db"
  chunk_duration_minutes: 5
  chunk_max_lines: 10000
  chunk_max_size_mb: 10
  compression_level: 6
  retention_days: 7
  cleanup_interval_hours: 1
  max_disk_usage_gb: 10
  buffer_size: 10000
  flush_interval_seconds: 5
  batch_size: 1000
YAML
  
  echo "Created config for iarnet-$i"
done

echo "All configs created successfully!"
