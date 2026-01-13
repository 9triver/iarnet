import { NextRequest, NextResponse } from "next/server"

// 简单的内存存储验证码（生产环境应使用 Redis 等）
const captchaStore = new Map<string, { code: string; expiresAt: number }>()

// 清理过期验证码
function cleanExpiredCaptchas() {
  const now = Date.now()
  for (const [key, value] of captchaStore.entries()) {
    if (value.expiresAt < now) {
      captchaStore.delete(key)
    }
  }
}

// 生成随机验证码（4位数字+字母混合）
function generateCaptchaCode(): string {
  const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // 排除容易混淆的字符
  let code = ""
  for (let i = 0; i < 4; i++) {
    code += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  return code
}

// 生成 SVG 验证码图片
function generateCaptchaImage(code: string): string {
  const width = 120
  const height = 40
  
  // 生成随机背景色
  const bgColor = `rgb(${200 + Math.floor(Math.random() * 55)}, ${200 + Math.floor(Math.random() * 55)}, ${200 + Math.floor(Math.random() * 55)})`
  
  // 生成随机干扰线
  const lines: string[] = []
  for (let i = 0; i < 3; i++) {
    const x1 = Math.random() * width
    const y1 = Math.random() * height
    const x2 = Math.random() * width
    const y2 = Math.random() * height
    const color = `rgb(${Math.floor(Math.random() * 200)}, ${Math.floor(Math.random() * 200)}, ${Math.floor(Math.random() * 200)})`
    lines.push(`<line x1="${x1}" y1="${y1}" x2="${x2}" y2="${y2}" stroke="${color}" stroke-width="1" opacity="0.3"/>`)
  }
  
  // 生成随机干扰点
  const dots: string[] = []
  for (let i = 0; i < 30; i++) {
    const x = Math.random() * width
    const y = Math.random() * height
    const color = `rgb(${Math.floor(Math.random() * 200)}, ${Math.floor(Math.random() * 200)}, ${Math.floor(Math.random() * 200)})`
    dots.push(`<circle cx="${x}" cy="${y}" r="1" fill="${color}" opacity="0.5"/>`)
  }
  
  // 生成字符位置和样式
  const chars: string[] = []
  const charWidth = width / code.length
  code.split("").forEach((char, index) => {
    const x = charWidth * index + charWidth / 2
    const y = height / 2 + (Math.random() - 0.5) * 8 // 随机垂直偏移
    const rotation = (Math.random() - 0.5) * 30 // 随机旋转角度
    const fontSize = 20 + Math.floor(Math.random() * 8)
    const color = `rgb(${Math.floor(Math.random() * 100)}, ${Math.floor(Math.random() * 100)}, ${Math.floor(Math.random() * 100)})`
    
    chars.push(
      `<text x="${x}" y="${y}" font-family="Arial, sans-serif" font-size="${fontSize}" font-weight="bold" fill="${color}" text-anchor="middle" transform="rotate(${rotation} ${x} ${y})">${char}</text>`
    )
  })
  
  const svg = `
    <svg width="${width}" height="${height}" xmlns="http://www.w3.org/2000/svg">
      <rect width="${width}" height="${height}" fill="${bgColor}"/>
      ${lines.join("")}
      ${dots.join("")}
      ${chars.join("")}
    </svg>
  `.trim()
  
  return svg
}

// GET: 生成验证码图片
export async function GET(request: NextRequest) {
  cleanExpiredCaptchas()
  
  // 生成验证码 ID
  const captchaId = Math.random().toString(36).substring(2, 15)
  
  // 生成验证码
  const code = generateCaptchaCode()
  
  // 计算过期时间（2分钟）
  const expiresAt = Date.now() + 2 * 60 * 1000
  
  // 存储验证码（使用后端计算的过期时间）
  captchaStore.set(captchaId, {
    code: code.toUpperCase(), // 统一转换为大写进行比较
    expiresAt: expiresAt, // 使用后端计算的过期时间
  })
  
  // 生成 SVG 图片
  const svgImage = generateCaptchaImage(code)
  
  // 返回 SVG 图片和验证码 ID，以及过期时间（由后端控制）
  return new NextResponse(svgImage, {
    headers: {
      "Content-Type": "image/svg+xml",
      "X-Captcha-Id": captchaId,
      "X-Captcha-Expires-At": expiresAt.toString(), // 后端返回的过期时间戳
      "Cache-Control": "no-cache, no-store, must-revalidate",
      "Pragma": "no-cache",
      "Expires": "0",
    },
  })
}

// POST: 验证验证码
export async function POST(request: NextRequest) {
  try {
    const body = await request.json()
    const { captchaId, answer } = body
    
    if (!captchaId || answer === undefined) {
      return NextResponse.json(
        { valid: false, message: "验证码ID和答案不能为空" },
        { status: 400 }
      )
    }
    
    const captcha = captchaStore.get(captchaId)
    
    if (!captcha) {
      return NextResponse.json(
        { valid: false, message: "验证码不存在或已过期" },
        { status: 400 }
      )
    }
    
    // 检查是否过期
    if (captcha.expiresAt < Date.now()) {
      captchaStore.delete(captchaId)
      return NextResponse.json(
        { valid: false, message: "验证码已过期，请刷新" },
        { status: 400 }
      )
    }
    
    // 验证答案（不区分大小写）
    const isValid = captcha.code.toUpperCase() === answer.toString().toUpperCase().trim()
    
    // 只有验证成功时才删除验证码（允许验证失败后重试）
    if (isValid) {
      captchaStore.delete(captchaId)
    }
    
    return NextResponse.json({
      valid: isValid,
      message: isValid ? "验证码正确" : "验证码错误",
    })
  } catch (error) {
    return NextResponse.json(
      { valid: false, message: "验证失败" },
      { status: 500 }
    )
  }
}
