"use client"

const encoder = new TextEncoder()

const bufferToHex = (buffer: ArrayBuffer) => {
  return Array.from(new Uint8Array(buffer))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("")
}

const fallbackHash = (input: string) => {
  let hash = 0
  for (let i = 0; i < input.length; i++) {
    hash = (hash << 5) - hash + input.charCodeAt(i)
    hash |= 0
  }
  return hash.toString(16)
}

export async function hashPassword(password: string): Promise<string> {
  try {
    if (typeof crypto !== "undefined" && crypto.subtle) {
      const data = encoder.encode(password)
      const digest = await crypto.subtle.digest("SHA-256", data)
      return bufferToHex(digest)
    }
  } catch {
    // ignore, fallback below
  }
  return fallbackHash(password)
}

export async function verifyPassword(password: string, hash: string): Promise<boolean> {
  const calculated = await hashPassword(password)
  return calculated === hash
}

