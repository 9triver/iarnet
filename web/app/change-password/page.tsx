"use client"

import { Suspense, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { FullscreenLoader } from "@/components/ui/fullscreen-loader"
import { authAPI, APIError, getErrorMessage } from "@/lib/api"
import { validatePasswordComplexity } from "@/lib/utils"
import { toast } from "sonner"

function ChangePasswordForm() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const presetUsername = searchParams.get("username") || ""

  const [username, setUsername] = useState(presetUsername)
  const [oldPassword, setOldPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [isSubmitting, setIsSubmitting] = useState(false)

  const handleSubmit = async () => {
    const trimmedUsername = username.trim()
    if (!trimmedUsername) {
      toast.error("用户名不能为空")
      return
    }
    if (!oldPassword || !newPassword || !confirmPassword) {
      toast.error("请填写所有字段")
      return
    }
    if (newPassword !== confirmPassword) {
      toast.error("新密码和确认密码不一致")
      return
    }

    const passwordError = validatePasswordComplexity(newPassword)
    if (passwordError) {
      toast.error(passwordError)
      return
    }

    setIsSubmitting(true)
    try {
      await authAPI.changePasswordWithCredential({
        username: trimmedUsername,
        oldPassword,
        newPassword,
      })
      toast.success("密码修改成功，请使用新密码重新登录")
      // 修改成功后跳转回登录页，并带上用户名便于用户输入
      const loginUrl = `/login?username=${encodeURIComponent(trimmedUsername)}`
      router.replace(loginUrl)
    } catch (error) {
      const message =
        error instanceof APIError
          ? getErrorMessage(error.message)
          : error instanceof Error
          ? error.message
          : "修改密码失败"
      toast.error("修改密码失败", {
        description: message,
      })
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/30 p-4">
      <Card className="w-full max-w-md shadow-lg">
        <CardHeader>
          <CardTitle className="text-2xl font-bold">修改过期密码</CardTitle>
          <CardDescription>
            由于密码已超过3个月未修改，为保障账号安全，请先完成密码更新后再登录系统。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="username">用户名</Label>
            <Input
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="请输入用户名"
              autoComplete="username"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="old-password">当前密码</Label>
            <Input
              id="old-password"
              type="password"
              value={oldPassword}
              onChange={(e) => setOldPassword(e.target.value)}
              placeholder="请输入当前密码"
              autoComplete="current-password"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="new-password">新密码</Label>
            <Input
              id="new-password"
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              placeholder="请输入新密码（8-16位，包含大小写字母、数字、特殊字符）"
              autoComplete="new-password"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="confirm-password">确认新密码</Label>
            <Input
              id="confirm-password"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              placeholder="请再次输入新密码"
              autoComplete="new-password"
              onKeyDown={(e) => {
                if (e.key === "Enter" && !isSubmitting) {
                  void handleSubmit()
                }
              }}
            />
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <Button
              variant="outline"
              onClick={() => {
                router.replace("/login")
              }}
              disabled={isSubmitting}
            >
              返回登录
            </Button>
            <Button onClick={handleSubmit} disabled={isSubmitting}>
              {isSubmitting ? "提交中..." : "确认修改"}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

export default function ChangePasswordPage() {
  return (
    <Suspense fallback={<FullscreenLoader message="加载中..." />}>
      <ChangePasswordForm />
    </Suspense>
  )
}

