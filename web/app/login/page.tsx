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

interface LoginFormValues {
  username: string
  password: string
}

export default function LoginPage() {
  const router = useRouter()
  const currentUser = useIARNetStore((state) => state.currentUser)
  const login = useIARNetStore((state) => state.login)
  const [error, setError] = useState<string | null>(null)

  const form = useForm<LoginFormValues>({
    defaultValues: {
      username: "",
      password: "",
    },
  })

  useEffect(() => {
    if (currentUser) {
      router.replace("/resources")
    }
  }, [currentUser, router])

  const onSubmit = async (values: LoginFormValues) => {
    setError(null)
    try {
      await login(values.username, values.password)
      router.replace("/resources")
    } catch (err) {
      const message = err instanceof Error ? err.message : "登录失败"
      setError(message)
    }
  }

  if (currentUser) {
    return <FullscreenLoader message="正在跳转到控制台..." />
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/30 p-4">
      <Card className="w-full max-w-md shadow-lg">
        <CardHeader>
          <CardTitle className="text-2xl font-bold">登录 IARNet</CardTitle>
          <CardDescription>输入账号密码以访问算力资源管理平台。</CardDescription>
        </CardHeader>
        <CardContent>
          <Form {...form}>
            <form className="space-y-4" onSubmit={form.handleSubmit(onSubmit)}>
              <FormField
                control={form.control}
                name="username"
                rules={{ required: "请输入用户名" }}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>用户名</FormLabel>
                    <FormControl>
                      <Input placeholder="账号" {...field} autoComplete="username" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="password"
                rules={{ required: "请输入密码" }}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>密码</FormLabel>
                    <FormControl>
                      <Input type="password" placeholder="密码" {...field} autoComplete="current-password" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              {error && <p className="text-sm text-red-600">{error}</p>}
              <Button type="submit" className="w-full" disabled={form.formState.isSubmitting}>
                {form.formState.isSubmitting ? "登录中..." : "登录"}
              </Button>
            </form>
          </Form>
        </CardContent>
      </Card>
    </div>
  )
}

