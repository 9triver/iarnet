import { type NextRequest, NextResponse } from "next/server"

// POST /api/status/[id]/restart - 重启应用
export async function POST(request: NextRequest, { params }: { params: { id: string } }) {
  try {
    const { id } = params

    // 这里应该重启应用实例
    return NextResponse.json({
      success: true,
      message: "Application restarted",
      data: {
        id,
        lastRestart: new Date().toISOString(),
        uptime: "0分钟",
      },
    })
  } catch (error) {
    return NextResponse.json({ success: false, error: "Failed to restart application" }, { status: 500 })
  }
}
