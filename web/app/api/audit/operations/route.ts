import { type NextRequest, NextResponse } from "next/server"

// GET /api/audit/operations - 获取操作日志
export async function GET(request: NextRequest) {
  try {
    const searchParams = request.nextUrl.searchParams
    const limit = parseInt(searchParams.get("limit") || "100")
    
    // 这里应该从后端获取操作日志
    // 暂时返回模拟数据，实际应该调用后端 API
    const backendUrl = process.env.BACKEND_URL || "http://localhost:8083"
    
    try {
      const response = await fetch(`${backendUrl}/audit/operations?limit=${limit}`, {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
        },
      })

      if (!response.ok) {
        throw new Error(`Backend API error: ${response.status}`)
      }

      const data = await response.json()
      return NextResponse.json({ code: 200, message: "success", data })
    } catch (error: any) {
      // 如果后端 API 不存在，返回空数据
      console.warn("Audit operations API not available:", error.message)
      return NextResponse.json({
        code: 200,
        message: "success",
        data: {
          logs: [],
          total: 0,
        },
      })
    }
  } catch (error: any) {
    return NextResponse.json(
      { code: 500, message: error.message || "Failed to fetch operation logs" },
      { status: 500 }
    )
  }
}

