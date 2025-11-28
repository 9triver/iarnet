"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { useForm } from "react-hook-form"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form"
import { useIARNetStore } from "@/lib/store"
import { FullscreenLoader } from "@/components/ui/fullscreen-loader"

interface SetupFormValues {
  username: string
  password: string
  confirmPassword: string
}

export default function SetupPage() {
  const router = useRouter()
  const setupCompleted = useIARNetStore((state) => state.setupCompleted)
  const createInitialAdmin = useIARNetStore((state) => state.createInitialAdmin)
  const [error, setError] = useState<string | null>(null)

  const form = useForm<SetupFormValues>({
    defaultValues: {
      username: "",
      password: "",
      confirmPassword: "",
    },
  })

  useEffect(() => {
    if (setupCompleted) {
      router.replace("/")
    }
  }, [setupCompleted, router])

  const onSubmit = async (values: SetupFormValues) => {
    setError(null)
    if (values.password !== values.confirmPassword) {
      form.setError("confirmPassword", {
        type: "manual",
        message: "两次输入的密码不一致",
      })
      return
    }
    try {
      await createInitialAdmin({ username: values.username, password: values.password })
      router.replace("/")
    } catch (err) {
      const message = err instanceof Error ? err.message : "管理员配置失败"
      setError(message)
    }
  }

  if (setupCompleted) {
    return <FullscreenLoader message="正在跳转..." />
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/30 p-4">
      <Card className="w-full max-w-md shadow-lg">
        <CardHeader>
          <CardTitle className="text-2xl font-bold">初始化管理员账户</CardTitle>
          <CardDescription>首次使用 IARNet 前，请先配置平台管理员账户。</CardDescription>
        </CardHeader>
        <CardContent>
          <Form {...form}>
            <form className="space-y-4" onSubmit={form.handleSubmit(onSubmit)}>
              <FormField
                control={form.control}
                name="username"
                rules={{ required: "请输入管理员用户名" }}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>管理员用户名</FormLabel>
                    <FormControl>
                      <Input placeholder="例如: admin" {...field} autoComplete="username" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="password"
                rules={{ required: "请输入密码", minLength: { value: 6, message: "至少 6 位字符" } }}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>登录密码</FormLabel>
                    <FormControl>
                      <Input type="password" placeholder="至少 6 位" {...field} autoComplete="new-password" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="confirmPassword"
                rules={{ required: "请再次输入密码确认" }}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>确认密码</FormLabel>
                    <FormControl>
                      <Input type="password" placeholder="再次输入密码" {...field} autoComplete="new-password" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              {error && <p className="text-sm text-red-600">{error}</p>}
              <Button type="submit" className="w-full" disabled={form.formState.isSubmitting}>
                {form.formState.isSubmitting ? "配置中..." : "完成配置"}
              </Button>
            </form>
          </Form>
        </CardContent>
      </Card>
    </div>
  )
}

