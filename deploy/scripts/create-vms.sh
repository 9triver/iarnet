#!/bin/bash
# 批量创建虚拟机脚本
# 支持创建不同类型的虚拟机：iarnet, k8s-master, k8s-worker, docker

set -e

# 加载配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${SCRIPT_DIR}/../vm-config.yaml"
BASE_DIR="${SCRIPT_DIR}/.."

# 检查依赖
command -v virsh >/dev/null 2>&1 || { echo "错误: 需要安装 libvirt"; exit 1; }
command -v virt-install >/dev/null 2>&1 || { echo "错误: 需要安装 virt-install"; exit 1; }
command -v yq >/dev/null 2>&1 || { echo "错误: 需要安装 yq (https://github.com/mikefarah/yq)"; exit 1; }

# 解析配置
NETWORK_NAME=$(yq eval '.global.network_name' "$CONFIG_FILE")
BASE_IMAGE=$(yq eval '.global.base_image' "$CONFIG_FILE")
SSH_KEY=$(yq eval '.global.ssh_key_path' "$CONFIG_FILE" | sed "s|~|$HOME|")
VM_USER=$(yq eval '.global.user' "$CONFIG_FILE")

# 检查基础镜像是否存在
if [ ! -f "$BASE_IMAGE" ]; then
    echo "错误: 基础镜像不存在: $BASE_IMAGE"
    echo "请先下载 Ubuntu Cloud Image 并放置到该路径"
    exit 1
fi

# 检查网络是否存在
if ! virsh net-info "$NETWORK_NAME" >/dev/null 2>&1; then
    echo "错误: 网络 $NETWORK_NAME 不存在"
    echo "请先创建网络: virsh net-define ${BASE_DIR}/networks/${NETWORK_NAME}.xml && virsh net-start $NETWORK_NAME && virsh net-autostart $NETWORK_NAME"
    exit 1
fi

# 创建目录
mkdir -p "${BASE_DIR}/cloud-init"
mkdir -p "${BASE_DIR}/images"
mkdir -p "${BASE_DIR}/logs"

# 函数：创建单个虚拟机
create_vm() {
    local vm_name=$1
    local vm_type=$2
    local ip=$3
    local cpu=$4
    local memory=$5
    local disk=$6
    local extra_config=$7
    
    echo "创建虚拟机: $vm_name (类型: $vm_type, IP: $ip)"
    
    # 创建磁盘镜像（从基础镜像克隆）
    local disk_path="${BASE_DIR}/images/${vm_name}.qcow2"
    if [ -f "$disk_path" ]; then
        echo "  警告: 磁盘镜像已存在，跳过: $disk_path"
        return
    fi
    
    qemu-img create -f qcow2 -b "$BASE_IMAGE" -F qcow2 "$disk_path" "${disk}G"
    
    # 生成 cloud-init 配置
    local cloud_init_dir="${BASE_DIR}/cloud-init/${vm_name}"
    mkdir -p "$cloud_init_dir"
    
    # 根据类型生成不同的 cloud-init 配置
    case $vm_type in
        iarnet)
            generate_cloud_init_iarnet "$cloud_init_dir" "$vm_name" "$ip" "$extra_config"
            ;;
        k8s-master)
            generate_cloud_init_k8s_master "$cloud_init_dir" "$vm_name" "$ip" "$extra_config"
            ;;
        k8s-worker)
            generate_cloud_init_k8s_worker "$cloud_init_dir" "$vm_name" "$ip" "$extra_config"
            ;;
        docker)
            generate_cloud_init_docker "$cloud_init_dir" "$vm_name" "$ip" "$extra_config"
            ;;
        *)
            echo "  错误: 未知的虚拟机类型: $vm_type"
            return 1
            ;;
    esac
    
    # 创建 cloud-init ISO
    cloud-localds "${cloud_init_dir}/cloud-init.iso" \
        "${cloud_init_dir}/user-data" \
        "${cloud_init_dir}/meta-data" \
        "${cloud_init_dir}/network-config" 2>/dev/null || \
    genisoimage -output "${cloud_init_dir}/cloud-init.iso" \
        -volid cidata -joliet -rock \
        "${cloud_init_dir}/user-data" \
        "${cloud_init_dir}/meta-data" \
        "${cloud_init_dir}/network-config" 2>/dev/null
    
    # 使用 virt-install 创建虚拟机
    virt-install \
        --name "$vm_name" \
        --ram "$memory" \
        --vcpus "$cpu" \
        --disk path="$disk_path",format=qcow2 \
        --disk path="${cloud_init_dir}/cloud-init.iso",device=cdrom \
        --network network="$NETWORK_NAME" \
        --graphics none \
        --console pty,target_type=serial \
        --import \
        --noautoconsole \
        --os-variant ubuntu22.04 \
        > "${BASE_DIR}/logs/${vm_name}.log" 2>&1
    
    echo "  ✓ 虚拟机 $vm_name 创建完成"
}

# 函数：生成 iarnet 节点的 cloud-init 配置
generate_cloud_init_iarnet() {
    local dir=$1
    local hostname=$2
    local ip=$3
    local extra=$4
    
    cat > "${dir}/user-data" <<EOF
#cloud-config
hostname: ${hostname}
users:
  - name: ${VM_USER}
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh-authorized-keys:
      - $(cat "$SSH_KEY")
package_update: true
packages:
  - docker.io
  - docker-compose
  - curl
  - wget
  - git
runcmd:
  - systemctl enable docker
  - systemctl start docker
  - usermod -aG docker ${VM_USER}
write_files:
  - path: /etc/docker/daemon.json
    content: |
      {
        "log-driver": "json-file",
        "log-opts": {
          "max-size": "10m",
          "max-file": "3"
        }
      }
final_message: "iarnet node ${hostname} is ready"
EOF

    cat > "${dir}/meta-data" <<EOF
instance-id: ${hostname}
local-hostname: ${hostname}
EOF

    cat > "${dir}/network-config" <<EOF
version: 2
ethernets:
  eth0:
    dhcp4: false
    addresses:
      - ${ip}/24
    gateway4: 172.30.0.1
    nameservers:
      addresses:
        - 8.8.8.8
        - 8.8.4.4
EOF
}

# 函数：生成 K8s Master 节点的 cloud-init 配置
generate_cloud_init_k8s_master() {
    local dir=$1
    local hostname=$2
    local ip=$3
    local extra=$4  # 格式: "cluster_id:1,port:50052,pod_cidr:10.244.0.0/16"
    
    # 解析额外配置
    local cluster_id=$(echo "$extra" | grep -oP 'cluster_id:\K[^,]*')
    local port=$(echo "$extra" | grep -oP 'port:\K[^,]*')
    local pod_cidr=$(echo "$extra" | grep -oP 'pod_cidr:\K[^,]*')
    
    cat > "${dir}/user-data" <<EOF
#cloud-config
hostname: ${hostname}
users:
  - name: ${VM_USER}
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh-authorized-keys:
      - $(cat "$SSH_KEY")
package_update: true
packages:
  - apt-transport-https
  - ca-certificates
  - curl
  - wget
  - git
write_files:
  - path: /etc/modules-load.d/k8s.conf
    content: |
      overlay
      br_netfilter
  - path: /etc/sysctl.d/k8s.conf
    content: |
      net.bridge.bridge-nf-call-iptables  = 1
      net.bridge.bridge-nf-call-ip6tables = 1
      net.ipv4.ip_forward                 = 1
  - path: /etc/containerd/config.toml
    content: |
      version = 2
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
          SystemdCgroup = true
  - path: /root/k8s-init.sh
    permissions: '0755'
    content: |
      #!/bin/bash
      set -e
      # 等待网络就绪
      sleep 10
      # 初始化 Kubernetes 集群
      kubeadm init --pod-network-cidr=${pod_cidr} --apiserver-advertise-address=${ip} --apiserver-cert-extra-sans=${hostname}
      # 配置 kubectl
      mkdir -p /home/${VM_USER}/.kube
      cp -i /etc/kubernetes/admin.conf /home/${VM_USER}/.kube/config
      chown -R ${VM_USER}:${VM_USER} /home/${VM_USER}/.kube
      # 安装 Flannel 网络插件
      kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml
      # 部署 metrics-server
      kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
      # 生成 join token 并保存
      kubeadm token create --print-join-command > /root/k8s-join-command.sh
      chmod +x /root/k8s-join-command.sh
      # 等待集群就绪
      kubectl wait --for=condition=Ready nodes --all --timeout=300s || true
runcmd:
  - modprobe overlay
  - modprobe br_netfilter
  - sysctl --system
  - curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
  - echo "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu \$(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
  - apt-get update
  - apt-get install -y containerd.io
  - mkdir -p /etc/containerd
  - containerd config default | tee /etc/containerd/config.toml > /dev/null
  - sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
  - systemctl restart containerd
  - systemctl enable containerd
  - curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.28/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
  - echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.28/deb/ /' | tee /etc/apt/sources.list.d/kubernetes.list
  - apt-get update
  - apt-get install -y kubelet kubeadm kubectl
  - apt-mark hold kubelet kubeadm kubectl
  - systemctl enable kubelet
  - bash /root/k8s-init.sh
final_message: "K8s master node ${hostname} (cluster ${cluster_id}) is ready"
EOF

    cat > "${dir}/meta-data" <<EOF
instance-id: ${hostname}
local-hostname: ${hostname}
EOF

    cat > "${dir}/network-config" <<EOF
version: 2
ethernets:
  eth0:
    dhcp4: false
    addresses:
      - ${ip}/24
    gateway4: 172.30.0.1
    nameservers:
      addresses:
        - 8.8.8.8
        - 8.8.4.4
EOF
}

# 函数：生成 K8s Worker 节点的 cloud-init 配置
generate_cloud_init_k8s_worker() {
    local dir=$1
    local hostname=$2
    local ip=$3
    local extra=$4  # 格式: "cluster_id:1,master_ip:172.30.2.1"
    
    # 解析额外配置
    local cluster_id=$(echo "$extra" | grep -oP 'cluster_id:\K[^,]*')
    local master_ip=$(echo "$extra" | grep -oP 'master_ip:\K[^,]*')
    
    cat > "${dir}/user-data" <<EOF
#cloud-config
hostname: ${hostname}
users:
  - name: ${VM_USER}
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh-authorized-keys:
      - $(cat "$SSH_KEY")
package_update: true
packages:
  - apt-transport-https
  - ca-certificates
  - curl
  - wget
write_files:
  - path: /etc/modules-load.d/k8s.conf
    content: |
      overlay
      br_netfilter
  - path: /etc/sysctl.d/k8s.conf
    content: |
      net.bridge.bridge-nf-call-iptables  = 1
      net.bridge.bridge-nf-call-ip6tables = 1
      net.ipv4.ip_forward                 = 1
  - path: /root/k8s-join.sh
    permissions: '0755'
    content: |
      #!/bin/bash
      set -e
      # 等待 master 节点就绪
      for i in {1..60}; do
        if ssh -o StrictHostKeyChecking=no ${VM_USER}@${master_ip} "kubectl get nodes" 2>/dev/null; then
          break
        fi
        echo "等待 master 节点就绪... (\$i/60)"
        sleep 10
      done
      # 从 master 节点获取 join 命令
      JOIN_CMD=\$(ssh -o StrictHostKeyChecking=no ${VM_USER}@${master_ip} "cat /root/k8s-join-command.sh" 2>/dev/null)
      if [ -n "\$JOIN_CMD" ]; then
        sudo \$JOIN_CMD
      else
        echo "错误: 无法获取 join 命令"
        exit 1
      fi
runcmd:
  - modprobe overlay
  - modprobe br_netfilter
  - sysctl --system
  - curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
  - echo "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu \$(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
  - apt-get update
  - apt-get install -y containerd.io
  - mkdir -p /etc/containerd
  - containerd config default | tee /etc/containerd/config.toml > /dev/null
  - sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
  - systemctl restart containerd
  - systemctl enable containerd
  - curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.28/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
  - echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.28/deb/ /' | tee /etc/apt/sources.list.d/kubernetes.list
  - apt-get update
  - apt-get install -y kubelet kubeadm kubectl
  - apt-mark hold kubelet kubeadm kubectl
  - systemctl enable kubelet
  - bash /root/k8s-join.sh
final_message: "K8s worker node ${hostname} (cluster ${cluster_id}) is ready"
EOF

    cat > "${dir}/meta-data" <<EOF
instance-id: ${hostname}
local-hostname: ${hostname}
EOF

    cat > "${dir}/network-config" <<EOF
version: 2
ethernets:
  eth0:
    dhcp4: false
    addresses:
      - ${ip}/24
    gateway4: 172.30.0.1
    nameservers:
      addresses:
        - 8.8.8.8
        - 8.8.4.4
EOF
}

# 函数：生成 Docker Provider 节点的 cloud-init 配置
generate_cloud_init_docker() {
    local dir=$1
    local hostname=$2
    local ip=$3
    local extra=$4
    
    cat > "${dir}/user-data" <<EOF
#cloud-config
hostname: ${hostname}
users:
  - name: ${VM_USER}
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh-authorized-keys:
      - $(cat "$SSH_KEY")
package_update: true
packages:
  - docker.io
  - docker-compose
  - curl
  - wget
runcmd:
  - systemctl enable docker
  - systemctl start docker
  - usermod -aG docker ${VM_USER}
write_files:
  - path: /etc/docker/daemon.json
    content: |
      {
        "log-driver": "json-file",
        "log-opts": {
          "max-size": "10m",
          "max-file": "3"
        }
      }
final_message: "Docker provider node ${hostname} is ready"
EOF

    cat > "${dir}/meta-data" <<EOF
instance-id: ${hostname}
local-hostname: ${hostname}
EOF

    cat > "${dir}/network-config" <<EOF
version: 2
ethernets:
  eth0:
    dhcp4: false
    addresses:
      - ${ip}/24
    gateway4: 172.30.0.1
    nameservers:
      addresses:
        - 8.8.8.8
        - 8.8.4.4
EOF
}

# 主函数：批量创建虚拟机
main() {
    echo "=== 开始批量创建虚拟机 ==="
    
    # 创建 iarnet 节点
    echo "创建 iarnet 节点..."
    IARNET_COUNT=$(yq eval '.vm_types.iarnet.count' "$CONFIG_FILE")
    IARNET_IP_BASE=$(yq eval '.vm_types.iarnet.ip_base' "$CONFIG_FILE")
    IARNET_IP_START=$(yq eval '.vm_types.iarnet.ip_start' "$CONFIG_FILE")
    IARNET_CPU=$(yq eval '.vm_types.iarnet.cpu' "$CONFIG_FILE")
    IARNET_MEM=$(yq eval '.vm_types.iarnet.memory' "$CONFIG_FILE")
    IARNET_DISK=$(yq eval '.vm_types.iarnet.disk' "$CONFIG_FILE")
    IARNET_PREFIX=$(yq eval '.vm_types.iarnet.hostname_prefix' "$CONFIG_FILE")
    
    for i in $(seq 1 $IARNET_COUNT); do
        vm_name="${IARNET_PREFIX}-${i}"
        ip="${IARNET_IP_BASE}.$((IARNET_IP_START + i - 1))"
        create_vm "$vm_name" "iarnet" "$ip" "$IARNET_CPU" "$IARNET_MEM" "$IARNET_DISK" ""
    done
    
    # 创建 K8s 集群
    echo "创建 K8s 集群..."
    CLUSTER_COUNT=$(yq eval '.vm_types.k8s_clusters.count' "$CONFIG_FILE")
    K8S_IP_BASE=$(yq eval '.vm_types.k8s_clusters.master.ip_base' "$CONFIG_FILE")
    K8S_IP_START=$(yq eval '.vm_types.k8s_clusters.master.ip_start' "$CONFIG_FILE")
    K8S_IP_STEP=$(yq eval '.vm_types.k8s_clusters.master.ip_step' "$CONFIG_FILE")
    MASTER_CPU=$(yq eval '.vm_types.k8s_clusters.master.cpu' "$CONFIG_FILE")
    MASTER_MEM=$(yq eval '.vm_types.k8s_clusters.master.memory' "$CONFIG_FILE")
    MASTER_DISK=$(yq eval '.vm_types.k8s_clusters.master.disk' "$CONFIG_FILE")
    MASTER_PREFIX=$(yq eval '.vm_types.k8s_clusters.master.hostname_prefix' "$CONFIG_FILE")
    MASTER_SUFFIX=$(yq eval '.vm_types.k8s_clusters.master.hostname_suffix' "$CONFIG_FILE")
    WORKER_COUNT=$(yq eval '.vm_types.k8s_clusters.worker.count_per_cluster' "$CONFIG_FILE")
    WORKER_CPU=$(yq eval '.vm_types.k8s_clusters.worker.cpu' "$CONFIG_FILE")
    WORKER_MEM=$(yq eval '.vm_types.k8s_clusters.worker.memory' "$CONFIG_FILE")
    WORKER_DISK=$(yq eval '.vm_types.k8s_clusters.worker.disk' "$CONFIG_FILE")
    WORKER_PREFIX=$(yq eval '.vm_types.k8s_clusters.worker.hostname_prefix' "$CONFIG_FILE")
    WORKER_SUFFIX=$(yq eval '.vm_types.k8s_clusters.worker.hostname_suffix' "$CONFIG_FILE")
    PORT_BASE=$(yq eval '.vm_types.k8s_clusters.master.provider_port_base' "$CONFIG_FILE")
    
    # 读取 Pod CIDR 列表
    POD_CIDRS=($(yq eval '.k8s_pod_cidrs.[]' "$CONFIG_FILE"))
    
    for cluster_id in $(seq 1 $CLUSTER_COUNT); do
        echo "  创建集群 $cluster_id..."
        
        # 计算 IP
        master_ip_num=$((K8S_IP_START + (cluster_id - 1) * K8S_IP_STEP))
        master_ip="${K8S_IP_BASE}.${master_ip_num}"
        provider_port=$((PORT_BASE + cluster_id - 1))
        pod_cidr="${POD_CIDRS[$((cluster_id - 1))]}"
        
        # 创建 master 节点
        master_name="${MASTER_PREFIX}${cluster_id}${MASTER_SUFFIX}"
        create_vm "$master_name" "k8s-master" "$master_ip" "$MASTER_CPU" "$MASTER_MEM" "$MASTER_DISK" \
            "cluster_id:${cluster_id},port:${provider_port},pod_cidr:${pod_cidr}"
        
        # 创建 worker 节点
        for worker_id in $(seq 1 $WORKER_COUNT); do
            worker_ip_num=$((master_ip_num + worker_id))
            worker_ip="${K8S_IP_BASE}.${worker_ip_num}"
            worker_name="${WORKER_PREFIX}${cluster_id}${WORKER_SUFFIX}${worker_id}"
            create_vm "$worker_name" "k8s-worker" "$worker_ip" "$WORKER_CPU" "$WORKER_MEM" "$WORKER_DISK" \
                "cluster_id:${cluster_id},master_ip:${master_ip}"
        done
    done
    
    # 创建 Docker Provider 节点
    echo "创建 Docker Provider 节点..."
    DOCKER_COUNT=$(yq eval '.vm_types.docker.count' "$CONFIG_FILE")
    DOCKER_IP_BASE=$(yq eval '.vm_types.docker.ip_base' "$CONFIG_FILE")
    DOCKER_IP_START=$(yq eval '.vm_types.docker.ip_start' "$CONFIG_FILE")
    DOCKER_CPU=$(yq eval '.vm_types.docker.cpu' "$CONFIG_FILE")
    DOCKER_MEM=$(yq eval '.vm_types.docker.memory' "$CONFIG_FILE")
    DOCKER_DISK=$(yq eval '.vm_types.docker.disk' "$CONFIG_FILE")
    DOCKER_PREFIX=$(yq eval '.vm_types.docker.hostname_prefix' "$CONFIG_FILE")
    
    for i in $(seq 1 $DOCKER_COUNT); do
        vm_name="${DOCKER_PREFIX}-${i}"
        ip="${DOCKER_IP_BASE}.$((DOCKER_IP_START + i - 1))"
        create_vm "$vm_name" "docker" "$ip" "$DOCKER_CPU" "$DOCKER_MEM" "$DOCKER_DISK" ""
    done
    
    echo "=== 虚拟机创建完成 ==="
    echo "注意: 虚拟机需要几分钟时间完成初始化，请等待后再部署服务"
}

# 执行主函数
main "$@"

