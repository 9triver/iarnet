import { type NextRequest, NextResponse } from "next/server"

// GET /api/resources - 获取所有资源
export async function GET() {
  try {
    // 这里应该从数据库获取数据
    // 目前返回模拟数据
    const resources = [
      {
        id: "1",
        name: "生产环境集群",
        type: "kubernetes",
        url: "https://k8s-prod.example.com",
        status: "connected",
        cpu: { total: 32, used: 18 },
        memory: { total: 128, used: 76 },
        storage: { total: 2048, used: 1024 },
        lastUpdated: new Date().toISOString(),
      },
    ]

    return NextResponse.json({ success: true, data: resources })
  } catch (error) {
    return NextResponse.json({ success: false, error: "Failed to fetch resources" }, { status: 500 })
  }
}

// POST /api/resources - 创建新资源
export async function POST(request: NextRequest) {
  try {
    const body = await request.json()
    const { name, type, url, token, description } = body

    // 验证必填字段
    if (!name || !type || !url || !token) {
      return NextResponse.json({ success: false, error: "Missing required fields" }, { status: 400 })
    }

    // 这里应该验证资源连接并保存到数据库
    const newResource = {
      id: Date.now().toString(),
      name,
      type,
      url,
      status: "connected",
      cpu: { total: 0, used: 0 },
      memory: { total: 0, used: 0 },
      storage: { total: 0, used: 0 },
      lastUpdated: new Date().toISOString(),
    }

    return NextResponse.json({ success: true, data: newResource }, { status: 201 })
  } catch (error) {
    return NextResponse.json({ success: false, error: "Failed to create resource" }, { status: 500 })
  }
}
