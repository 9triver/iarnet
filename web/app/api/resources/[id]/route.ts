import { type NextRequest, NextResponse } from "next/server"

// PUT /api/resources/[id] - 更新资源
export async function PUT(request: NextRequest, { params }: { params: { id: string } }) {
  try {
    const { id } = params
    const body = await request.json()

    // 这里应该更新数据库中的资源
    const updatedResource = {
      id,
      ...body,
      lastUpdated: new Date().toISOString(),
    }

    return NextResponse.json({ success: true, data: updatedResource })
  } catch (error) {
    return NextResponse.json({ success: false, error: "Failed to update resource" }, { status: 500 })
  }
}

// DELETE /api/resources/[id] - 删除资源
export async function DELETE(request: NextRequest, { params }: { params: { id: string } }) {
  try {
    const { id } = params

    // 这里应该从数据库删除资源
    return NextResponse.json({ success: true, message: "Resource deleted" })
  } catch (error) {
    return NextResponse.json({ success: false, error: "Failed to delete resource" }, { status: 500 })
  }
}
