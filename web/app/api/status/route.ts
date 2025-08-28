import { NextResponse } from "next/server"

// GET /api/status - 获取所有应用状态
export async function GET() {
  try {
    // 这里应该从监控系统获取实时数据
    const statuses = [
      {
        id: "1",
        name: "用户管理系统",
        status: "running",
        uptime: "7天 12小时 30分钟",
        cpu: Math.floor(Math.random() * 100),
        memory: Math.floor(Math.random() * 100),
        network: Math.floor(Math.random() * 100),
        storage: Math.floor(Math.random() * 100),
        instances: 3,
        healthCheck: "healthy",
        lastRestart: "2024-01-08 09:15:00",
        runningOn: ["生产环境集群"],
        logs: [
          {
            timestamp: new Date().toISOString(),
            level: "info",
            message: "Application is running normally",
          },
        ],
        metrics: Array.from({ length: 6 }, (_, i) => ({
          timestamp: `14:${25 + i}`,
          cpu: Math.floor(Math.random() * 100),
          memory: Math.floor(Math.random() * 100),
          network: Math.floor(Math.random() * 100),
          requests: Math.floor(Math.random() * 500),
        })),
      },
    ]

    return NextResponse.json({ success: true, data: statuses })
  } catch (error) {
    return NextResponse.json({ success: false, error: "Failed to fetch status" }, { status: 500 })
  }
}
