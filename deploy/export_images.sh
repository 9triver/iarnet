#!/bin/bash
# 导出 Docker 镜像用于离线部署
# 支持压缩导出，大幅减少传输时间

set -e

# 配置
EXPORT_DIR="./offline_images"
COMPRESS=true
COMPRESS_METHOD="zstd"  # 压缩方法: gzip 或 zstd (zstd 压缩比更高，速度更快)
REMOVE_OLD=false

# 需要导出的镜像列表（从 docker-compose.yaml 中提取）
IMAGES=(
    "iarnet:latest"
    # "iarnet-global:latest"
    # "iarnet/provider:docker"
    # "iarnet/runner:python_3.11-latest"
    # "iarnet/component:python_3.11-latest"
)

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查镜像是否存在
check_images() {
    echo_info "检查镜像是否存在..."
    local missing=0
    for img in "${IMAGES[@]}"; do
        if ! docker image inspect "$img" &>/dev/null; then
            echo_error "镜像不存在: $img"
            missing=$((missing + 1))
        else
            local size=$(docker image inspect "$img" --format='{{.Size}}' | numfmt --to=iec-i --suffix=B 2>/dev/null || echo "unknown")
            echo_info "  ✓ $img ($size)"
        fi
    done
    
    if [ $missing -gt 0 ]; then
        echo_error "有 $missing 个镜像不存在，请先构建这些镜像"
        exit 1
    fi
}

# 计算总大小
calculate_total_size() {
    local total=0
    for img in "${IMAGES[@]}"; do
        if docker image inspect "$img" &>/dev/null; then
            local size=$(docker image inspect "$img" --format='{{.Size}}')
            total=$((total + size))
        fi
    done
    echo "$total"
}

# 导出镜像（压缩）
export_images_compressed() {
    # 检查压缩工具是否可用
    if [ "$COMPRESS_METHOD" = "zstd" ]; then
        if ! command -v zstd &> /dev/null; then
            echo_warn "zstd 未安装，回退到 gzip"
            COMPRESS_METHOD="gzip"
        fi
    fi
    
    local compress_cmd=""
    local compress_ext=""
    local compress_name=""
    
    if [ "$COMPRESS_METHOD" = "zstd" ]; then
        compress_cmd="zstd -9 -c"
        compress_ext=".tar.zst"
        compress_name="zstd -9"
    else
        compress_cmd="gzip -c"
        compress_ext=".tar.gz"
        compress_name="gzip"
    fi
    
    echo_info "开始导出镜像（使用 $compress_name 压缩）..."
    mkdir -p "$EXPORT_DIR"
    
    local total_size=$(calculate_total_size)
    local total_size_human=$(numfmt --to=iec-i --suffix=B "$total_size" 2>/dev/null || echo "$total_size bytes")
    echo_info "原始镜像总大小: $total_size_human"
    
    local start_time=$(date +%s)
    local idx=1
    
    for img in "${IMAGES[@]}"; do
        # 将镜像名转换为文件名（替换 / 和 : 为 _）
        local filename=$(echo "$img" | sed 's/\//_/g' | sed 's/:/_/g')
        local tar_file="$EXPORT_DIR/${filename}.tar"
        local compressed_file="$EXPORT_DIR/${filename}${compress_ext}"
        
        echo_info "[$idx/${#IMAGES[@]}] 导出: $img"
        
        # 导出为 tar
        docker save "$img" -o "$tar_file"
        
        # 压缩
        echo_info "  压缩中..."
        $compress_cmd "$tar_file" > "$compressed_file"
        
        # 删除未压缩的 tar
        rm -f "$tar_file"
        
        # 显示压缩后大小
        local compressed_size=$(stat -f%z "$compressed_file" 2>/dev/null || stat -c%s "$compressed_file" 2>/dev/null || echo "0")
        local compressed_size_human=$(numfmt --to=iec-i --suffix=B "$compressed_size" 2>/dev/null || echo "$compressed_size bytes")
        echo_info "  ✓ 完成: $compressed_file ($compressed_size_human)"
        
        idx=$((idx + 1))
    done
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    echo_info "导出完成，耗时: ${duration}秒"
}

# 导出镜像（不压缩）
export_images_uncompressed() {
    echo_info "开始导出镜像（不压缩）..."
    mkdir -p "$EXPORT_DIR"
    
    local start_time=$(date +%s)
    local idx=1
    
    for img in "${IMAGES[@]}"; do
        local filename=$(echo "$img" | sed 's/\//_/g' | sed 's/:/_/g')
        local tar_file="$EXPORT_DIR/${filename}.tar"
        
        echo_info "[$idx/${#IMAGES[@]}] 导出: $img"
        docker save "$img" -o "$tar_file"
        
        local size=$(stat -f%z "$tar_file" 2>/dev/null || stat -c%s "$tar_file" 2>/dev/null || echo "0")
        local size_human=$(numfmt --to=iec-i --suffix=B "$size" 2>/dev/null || echo "$size bytes")
        echo_info "  ✓ 完成: $tar_file ($size_human)"
        
        idx=$((idx + 1))
    done
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    echo_info "导出完成，耗时: ${duration}秒"
}

# 生成导入脚本
generate_import_script() {
    echo_info "生成导入脚本..."
    local import_script="$EXPORT_DIR/import_images.sh"
    
    cat > "$import_script" << 'EOF'
#!/bin/bash
# 导入 Docker 镜像（离线部署使用）

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPRESS=true

# 检查参数
if [ "$1" == "--uncompressed" ]; then
    COMPRESS=false
fi

echo "开始导入镜像..."
echo "压缩模式: $COMPRESS"
echo ""

for file in "$SCRIPT_DIR"/*.tar.gz "$SCRIPT_DIR"/*.tar; do
    if [ ! -f "$file" ]; then
        continue
    fi
    
    filename=$(basename "$file")
    echo "导入: $filename"
    
    if [[ "$file" == *.gz ]]; then
        # 解压并导入
        gunzip -c "$file" | docker load
    else
        # 直接导入
        docker load -i "$file"
    fi
    
    echo "  ✓ 完成"
    echo ""
done

echo "所有镜像导入完成！"
echo ""
echo "接下来可以运行:"
echo "  cd /path/to/deploy"
echo "  docker-compose up -d"
EOF
    
    chmod +x "$import_script"
    echo_info "  ✓ 生成: $import_script"
}

# 生成部署说明
generate_readme() {
    echo_info "生成部署说明..."
    local readme_file="$EXPORT_DIR/README.md"
    
    cat > "$readme_file" << 'EOF'
# 离线部署说明

## 文件说明

- `*.tar.zst`: 使用 zstd 压缩的 Docker 镜像文件（推荐，压缩比更高）
- `*.tar.gz`: 使用 gzip 压缩的 Docker 镜像文件
- `*.tar`: 未压缩的 Docker 镜像文件
- `import_images.sh`: 自动导入脚本

## 部署步骤

### 1. 传输文件

将整个 `offline_images` 目录传输到目标服务器（无网络环境）。

**推荐使用压缩文件传输**（如果目标服务器支持）：
```bash
# 在有网络的机器上打包
tar -czf offline_images.tar.gz offline_images/

# 传输到目标服务器后解压
tar -xzf offline_images.tar.gz
```

### 2. 导入镜像

在目标服务器上执行：

```bash
cd offline_images
chmod +x import_images.sh
./import_images.sh
```

或者手动导入：

```bash
# 如果使用 zstd 压缩文件（推荐）
for file in *.tar.zst; do
    zstd -dc "$file" | docker load
done

# 如果使用 gzip 压缩文件
for file in *.tar.gz; do
    gunzip -c "$file" | docker load
done

# 如果使用未压缩文件
for file in *.tar; do
    docker load -i "$file"
done
```

### 3. 部署应用

```bash
# 复制部署目录到目标服务器
# 确保包含 docker-compose.yaml 和所有配置文件

cd /path/to/deploy
docker-compose up -d
```

## 镜像列表

- `iarnet:latest` - 主应用镜像
- `iarnet-global:latest` - 全局注册中心镜像
- `iarnet/provider:docker` - Docker Provider 镜像
- `iarnet/runner:python_3.11-latest` - Python Runner 镜像（较大）
- `iarnet/component:python_3.11-latest` - Python Component 镜像（较大）

## 注意事项

1. **磁盘空间**: 确保目标服务器有足够的磁盘空间（建议至少 30GB）
2. **Docker 版本**: 确保目标服务器的 Docker 版本兼容
3. **网络配置**: 部署后需要配置网络和 IP 地址
4. **权限**: 确保有 Docker 操作权限

## 优化建议

如果传输仍然很慢，可以考虑：

1. **分批传输**: 先传输小镜像，再传输大镜像
2. **使用增量更新**: 只传输变更的镜像层
3. **优化镜像大小**: 使用多阶段构建和镜像压缩工具
4. **使用私有 Registry**: 在中间服务器搭建 Harbor 等私有仓库
EOF
    
    echo_info "  ✓ 生成: $readme_file"
}

# 显示统计信息
show_statistics() {
    echo ""
    echo_info "=== 导出统计 ==="
    
    local total_size=0
    local total_compressed=0
    
    for file in "$EXPORT_DIR"/*.tar.zst "$EXPORT_DIR"/*.tar.gz "$EXPORT_DIR"/*.tar; do
        if [ -f "$file" ]; then
            local size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null || echo "0")
            total_compressed=$((total_compressed + size))
        fi
    done
    
    local total_size_human=$(numfmt --to=iec-i --suffix=B "$(calculate_total_size)" 2>/dev/null || echo "unknown")
    local total_compressed_human=$(numfmt --to=iec-i --suffix=B "$total_compressed" 2>/dev/null || echo "unknown")
    
    echo_info "原始镜像总大小: $total_size_human"
    echo_info "导出文件总大小: $total_compressed_human"
    
    if [ "$COMPRESS" = true ] && [ $total_compressed -gt 0 ]; then
        local original=$(calculate_total_size)
        if [ $original -gt 0 ]; then
            local ratio=$(echo "scale=1; $total_compressed * 100 / $original" | bc 2>/dev/null || echo "N/A")
            echo_info "压缩率: ${ratio}%"
        fi
    fi
}

# 主函数
main() {
    echo_info "=== Docker 镜像离线导出工具 ==="
    echo ""
    
    # 解析参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --no-compress)
                COMPRESS=false
                shift
                ;;
            --zstd)
                COMPRESS_METHOD="zstd"
                shift
                ;;
            --gzip)
                COMPRESS_METHOD="gzip"
                shift
                ;;
            --remove-old)
                REMOVE_OLD=true
                shift
                ;;
            --dir)
                EXPORT_DIR="$2"
                shift 2
                ;;
            *)
                echo_error "未知参数: $1"
                echo "用法: $0 [--no-compress] [--zstd|--gzip] [--remove-old] [--dir DIR]"
                echo ""
                echo "压缩选项:"
                echo "  --zstd   - 使用 zstd 压缩（默认，压缩比更高，速度更快）"
                echo "  --gzip   - 使用 gzip 压缩"
                exit 1
                ;;
        esac
    done
    
    # 清理旧文件
    if [ "$REMOVE_OLD" = true ] && [ -d "$EXPORT_DIR" ]; then
        echo_warn "清理旧文件: $EXPORT_DIR"
        rm -rf "$EXPORT_DIR"
    fi
    
    # 检查镜像
    check_images
    
    # 导出镜像
    if [ "$COMPRESS" = true ]; then
        export_images_compressed
    else
        export_images_uncompressed
    fi
    
    # 生成辅助文件
    generate_import_script
    generate_readme
    
    # 显示统计
    show_statistics
    
    echo ""
    echo_info "导出完成！文件保存在: $EXPORT_DIR"
    echo_info "下一步: 将 $EXPORT_DIR 目录传输到目标服务器，然后运行 import_images.sh"
}

main "$@"

