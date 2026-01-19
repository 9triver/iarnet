"use client"

import { useEffect, useState, Suspense } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { useForm } from "react-hook-form"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form"
import { useIARNetStore } from "@/lib/store"
import { FullscreenLoader } from "@/components/ui/fullscreen-loader"
import { validateRedirectUrl } from "@/lib/utils"
import { captchaAPI, APIError, tokenManager } from "@/lib/api"
import { RefreshCw } from "lucide-react"
import { toast } from "sonner"

interface LoginFormValues {
  username: string
  password: string
  captcha: string
}

function LoginForm() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const currentUser = useIARNetStore((state) => state.currentUser)
  const login = useIARNetStore((state) => state.login)
  const [isRedirecting, setIsRedirecting] = useState(false)
  const [captchaId, setCaptchaId] = useState<string | null>(null)
  const [captchaImageUrl, setCaptchaImageUrl] = useState<string>("")
  const [captchaExpiresAt, setCaptchaExpiresAt] = useState<number>(0)
  const [loadingCaptcha, setLoadingCaptcha] = useState(false)
  const [timeRemaining, setTimeRemaining] = useState<number>(0)

  const form = useForm<LoginFormValues>({
    defaultValues: {
      username: "",
      password: "",
      captcha: "",
    },
  })

  // 加载验证码
  const loadCaptcha = async () => {
    try {
      setLoadingCaptcha(true)
      // 释放旧的图片 URL
      if (captchaImageUrl) {
        URL.revokeObjectURL(captchaImageUrl)
      }
      const response = await captchaAPI.getCaptcha()
      setCaptchaId(response.captchaId)
      setCaptchaImageUrl(response.imageUrl)
      setCaptchaExpiresAt(response.expiresAt)
      // 清空验证码输入
      form.setValue("captcha", "")
    } catch (error) {
      console.error("Failed to load captcha:", error)
      toast.error("加载验证码失败，请刷新页面重试")
    } finally {
      setLoadingCaptcha(false)
    }
  }

  // 组件卸载时释放图片 URL
  useEffect(() => {
    return () => {
      if (captchaImageUrl) {
        URL.revokeObjectURL(captchaImageUrl)
      }
    }
  }, [captchaImageUrl])

  // 更新剩余时间
  useEffect(() => {
    if (!captchaExpiresAt) return

    const updateTimeRemaining = () => {
      const now = Date.now()
      const remaining = Math.max(0, Math.floor((captchaExpiresAt - now) / 1000))
      setTimeRemaining(remaining)
      
      // 如果过期，自动刷新验证码
      if (remaining === 0 && captchaId) {
        loadCaptcha()
      }
    }

    // 立即更新一次
    updateTimeRemaining()

    // 每秒更新一次
    const interval = setInterval(updateTimeRemaining, 1000)

    return () => clearInterval(interval)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [captchaExpiresAt, captchaId])

  // 更新剩余时间
  useEffect(() => {
    if (!captchaExpiresAt) return

    const updateTimeRemaining = () => {
      const now = Date.now()
      const remaining = Math.max(0, Math.floor((captchaExpiresAt - now) / 1000))
      setTimeRemaining(remaining)
      
      // 如果过期，自动刷新验证码
      if (remaining === 0 && captchaId) {
        loadCaptcha()
      }
    }

    // 立即更新一次
    updateTimeRemaining()

    // 每秒更新一次
    const interval = setInterval(updateTimeRemaining, 1000)

    return () => clearInterval(interval)
  }, [captchaExpiresAt, captchaId])

  // 组件加载时获取验证码
  useEffect(() => {
    loadCaptcha()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    // 只有在登录页面且用户已登录时才跳转
    // 如果 URL 中有 redirect 参数，验证后跳转到指定页面；否则跳转到资源管理页面
    if (currentUser && !isRedirecting) {
      setIsRedirecting(true)
      const redirectParam = searchParams.get("redirect")
      const redirectTo = validateRedirectUrl(redirectParam, "/resources")
      router.replace(redirectTo)
    }
  }, [currentUser, router, searchParams, isRedirecting])

  const onSubmit = async (values: LoginFormValues) => {
    // 验证验证码
    if (!captchaId || !values.captcha) {
      toast.error("请输入验证码")
      return
    }

    try {
      // 先验证验证码
      const verifyResponse = await captchaAPI.verifyCaptcha({
        captchaId,
        answer: values.captcha,
      })

      if (!verifyResponse.valid) {
        toast.error(verifyResponse.message || "验证码错误，请重新输入")
        // 验证失败时自动刷新验证码
        await loadCaptcha()
        return
      }

      // 验证码正确，执行登录
      await login(values.username, values.password)
      
      // 获取token并显示登录成功提示（包含有效期）
      const token = tokenManager.getToken()
      const expirationTime = token ? tokenManager.getTokenExpirationTime(token) : null
      
      if (expirationTime) {
        // 格式化过期时间为 "xxxx-xx-xx xx:xx" 格式（显示到分钟）
        const expirationDate = new Date(expirationTime)
        const year = expirationDate.getFullYear()
        const month = String(expirationDate.getMonth() + 1).padStart(2, '0')
        const day = String(expirationDate.getDate()).padStart(2, '0')
        const hours = String(expirationDate.getHours()).padStart(2, '0')
        const minutes = String(expirationDate.getMinutes()).padStart(2, '0')
        const expirationStr = `${year}-${month}-${day} ${hours}:${minutes}`
        
        toast.success("登录成功", {
          description: `本次登录有效期至 ${expirationStr}`,
        })
      } else {
        toast.success("登录成功")
      }
      
      // 登录成功后，如果有 redirect 参数，验证后跳转到指定页面；否则跳转到资源管理页面
      // 注意：login 函数会自动获取用户角色信息
      const redirectParam = searchParams.get("redirect")
      const redirectTo = validateRedirectUrl(redirectParam, "/resources")
      router.replace(redirectTo)
    } catch (err) {
      if (err instanceof APIError) {
        if (err.status === 401) {
          const locked = Boolean(err.data?.locked)
          const remaining = typeof err.data?.remainingAttempts === "number" ? err.data.remainingAttempts : undefined
          
          if (locked) {
            // 账号已锁定，显示锁定信息和解锁时间
            const lockedUntil = err.data?.lockedUntil
            if (lockedUntil) {
              toast.error("账号已锁定", {
                description: `解锁时间：${lockedUntil}`,
              })
            } else {
              toast.error("账号已锁定", {
                description: "请稍后重试或联系管理员",
              })
            }
          } else if (typeof remaining === "number") {
            // 用户名或密码错误，显示剩余重试次数
            toast.error("用户名或密码错误", {
              description: `还剩 ${remaining} 次重试机会`,
            })
          } else {
            toast.error("用户名或密码错误")
          }
        } else {
          toast.error(err.message || "登录失败")
        }
      } else {
        const message = err instanceof Error ? err.message : "登录失败"
        toast.error(message)
      }
      // 登录失败后重新加载验证码
      await loadCaptcha()
    }
  }

  if (currentUser) {
    return <FullscreenLoader message="正在跳转到控制台..." />
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/30 p-4">
      <Card className="w-full max-w-md shadow-lg">
        <CardHeader>
          <CardTitle className="text-2xl font-bold">登录 智能应用运行平台原型系统</CardTitle>
          <CardDescription>输入账号密码以访问资源管理平台。</CardDescription>
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
              <FormField
                control={form.control}
                name="captcha"
                rules={{ required: "请输入验证码" }}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>验证码</FormLabel>
                    <div className="flex flex-col gap-2">
                      <div className="flex items-center gap-2">
                        <FormControl>
                          <Input 
                            placeholder="请输入验证码" 
                            {...field} 
                            autoComplete="off"
                            className="flex-1 uppercase"
                            maxLength={4}
                          />
                        </FormControl>
                        <div className="flex items-center gap-2 border rounded-md bg-white overflow-hidden">
                          {captchaImageUrl ? (
                            <img 
                              src={captchaImageUrl} 
                              alt="验证码" 
                              className="h-10 w-[120px] cursor-pointer hover:opacity-80 transition-opacity"
                              onClick={loadCaptcha}
                              title="点击刷新验证码"
                            />
                          ) : (
                            <div className="h-10 w-[120px] flex items-center justify-center bg-muted">
                              {loadingCaptcha ? (
                                <RefreshCw className="h-4 w-4 animate-spin" />
                              ) : (
                                <span className="text-xs text-muted-foreground">加载中</span>
                              )}
                            </div>
                          )}
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={loadCaptcha}
                            disabled={loadingCaptcha}
                            className="h-10 w-10 p-0"
                            title="刷新验证码"
                          >
                            <RefreshCw className={`h-4 w-4 ${loadingCaptcha ? 'animate-spin' : ''}`} />
                          </Button>
                        </div>
                      </div>
                      {timeRemaining > 0 && (
                        <div className="text-xs text-muted-foreground text-right px-1">
                          有效期还剩 {Math.floor(timeRemaining / 60)}:{(timeRemaining % 60).toString().padStart(2, '0')}
                        </div>
                      )}
                      {timeRemaining === 0 && captchaId && (
                        <div className="text-xs text-destructive text-right px-1">
                          验证码已过期，正在刷新...
                        </div>
                      )}
                    </div>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <Button type="submit" className="w-full" disabled={form.formState.isSubmitting || loadingCaptcha}>
                {form.formState.isSubmitting ? "登录中..." : "登录"}
              </Button>
            </form>
          </Form>
        </CardContent>
      </Card>
    </div>
  )
}

export default function LoginPage() {
  return (
    <Suspense fallback={<FullscreenLoader message="加载中..." />}>
      <LoginForm />
    </Suspense>
  )
}

