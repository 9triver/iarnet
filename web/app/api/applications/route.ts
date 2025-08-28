import { type NextRequest, NextResponse } from "next/server"

// GET /api/applications - 获取所有应用
export async function GET() {
  try {
    // 这里应该从数据库获取数据
    const applications = [
      {
        id: "1",
        name: "用户管理系统",
        description: "基于React和Node.js的用户管理后台系统",
        gitUrl: "https://github.com/company/user-management",
        branch: "main",
        status: "running",
        type: "web",
        lastDeployed: new Date().toISOString(),
        runningOn: ["生产环境集群"],
        port: 3000,
        healthCheck: "/health",
      },
    ]

    return NextResponse.json({ success: true, data: applications })
  } catch (error) {
    return NextResponse.json({ success: false, error: "Failed to fetch applications" }, { status: 500 })
  }
}

// POST /api/applications - 创建新应用
export async function POST(request: NextRequest) {
  try {
    const body = await request.json()
    const { name, gitUrl, branch, type, description, port, healthCheck } = body

    // 验证必填字段
    if (!name || !gitUrl || !branch || !type) {
      return NextResponse.json({ success: false, error: "Missing required fields" }, { status: 400 })
    }

    // 这里应该验证Git仓库并保存到数据库
    const newApplication = {
      id: Date.now().toString(),
      name,
      gitUrl,
      branch,
      type,
      description: description || "",
      status: "idle",
      port: port || 3000,
      healthCheck: healthCheck || "",
    }

    return NextResponse.json({ success: true, data: newApplication }, { status: 201 })
  } catch (error) {
    return NextResponse.json({ success: false, error: "Failed to create application" }, { status: 500 })
  }
}
