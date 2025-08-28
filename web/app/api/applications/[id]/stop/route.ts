import { type NextRequest, NextResponse } from "next/server"

// POST /api/applications/[id]/stop - 停止应用
export async function POST(request: NextRequest, { params }: { params: { id: string } }) {
  try {
    const { id } = params

    // 这里应该停止应用的运行实例
    return NextResponse.json({
      success: true,
      message: "Application stopped",
      data: {
        id,
        status: "stopped",
      },
    })
  } catch (error) {
    return NextResponse.json({ success: false, error: "Failed to stop application" }, { status: 500 })
  }
}
