"use client"

import { useMemo, useState } from "react"
import { Sidebar } from "@/components/sidebar"
import { AuthGuard } from "@/components/auth-guard"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { useForm } from "react-hook-form"
import { Users, ShieldCheck, Shield, Trash2 } from "lucide-react"
import { useIARNetStore, type UserRole } from "@/lib/store"
import { toast } from "sonner"

interface UserFormValues {
  username: string
  password: string
  role: UserRole
}

export default function UsersPage() {
  const users = useIARNetStore((state) => state.users)
  const currentUser = useIARNetStore((state) => state.currentUser)
  const addUserAccount = useIARNetStore((state) => state.addUserAccount)
  const deleteUserAccount = useIARNetStore((state) => state.deleteUserAccount)
  const [error, setError] = useState<string | null>(null)
  const [deletingId, setDeletingId] = useState<string | null>(null)

  const form = useForm<UserFormValues>({
    defaultValues: {
      username: "",
      password: "",
      role: "user",
    },
  })

  const sortedUsers = useMemo(
    () =>
      [...users].sort((a, b) => {
        if (a.role === b.role) {
          return a.username.localeCompare(b.username, "zh-CN")
        }
        return a.role === "admin" ? -1 : 1
      }),
    [users],
  )

  const onSubmit = async (values: UserFormValues) => {
    setError(null)
    try {
      await addUserAccount(values)
      toast.success(`用户 ${values.username} 创建成功`)
      form.reset({ username: "", password: "", role: "user" })
    } catch (err) {
      const message = err instanceof Error ? err.message : "添加用户失败"
      setError(message)
      toast.error(message)
    }
  }

  const handleDelete = async (id: string) => {
    setError(null)
    setDeletingId(id)
    try {
      await deleteUserAccount(id)
      toast.success("用户已删除")
    } catch (err) {
      const message = err instanceof Error ? err.message : "删除用户失败"
      setError(message)
      toast.error(message)
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <AuthGuard requireAdmin>
      <div className="flex h-screen bg-background">
        <Sidebar />
        <main className="flex-1 overflow-auto p-8">
          <div className="mb-8 flex items-center justify-between">
            <div>
              <h1 className="text-3xl font-playfair font-bold">用户管理</h1>
              <p className="text-muted-foreground">管理员可以在此维护平台访问账户。</p>
            </div>
          </div>

          <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center space-x-2">
                  <Users className="h-5 w-5 text-primary" />
                  <span>现有账户</span>
                </CardTitle>
                <CardDescription>管理所有可登录 IARNet 的账号。</CardDescription>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>用户名</TableHead>
                      <TableHead>角色</TableHead>
                      <TableHead className="text-right">操作</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {sortedUsers.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={3} className="text-center text-muted-foreground">
                          暂无账户
                        </TableCell>
                      </TableRow>
                    ) : (
                      sortedUsers.map((user) => (
                        <TableRow key={user.id}>
                          <TableCell>
                            <div className="font-medium">{user.username}</div>
                            {currentUser?.id === user.id && (
                              <div className="text-xs text-primary">当前登录</div>
                            )}
                          </TableCell>
                          <TableCell>
                            <Badge variant={user.role === "admin" ? "default" : "outline"} className="space-x-1">
                              {user.role === "admin" ? <ShieldCheck className="h-3 w-3" /> : <Shield className="h-3 w-3" />}
                              <span>{user.role === "admin" ? "管理员" : "普通用户"}</span>
                            </Badge>
                          </TableCell>
                          <TableCell className="text-right">
                            <Button
                              variant="ghost"
                              size="sm"
                              disabled={deletingId === user.id || currentUser?.id === user.id}
                              onClick={() => handleDelete(user.id)}
                            >
                              <Trash2 className="h-4 w-4" />
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
                {error && <p className="mt-4 text-sm text-red-600">{error}</p>}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>添加新用户</CardTitle>
                <CardDescription>创建可访问平台的普通或管理员账号。</CardDescription>
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
                            <Input placeholder="例如: ops_user" {...field} />
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
                          <FormLabel>初始密码</FormLabel>
                          <FormControl>
                            <Input type="password" placeholder="至少 6 位" {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name="role"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>角色</FormLabel>
                          <Select onValueChange={field.onChange} value={field.value}>
                            <FormControl>
                              <SelectTrigger>
                                <SelectValue placeholder="选择角色" />
                              </SelectTrigger>
                            </FormControl>
                            <SelectContent>
                              <SelectItem value="user">普通用户</SelectItem>
                              <SelectItem value="admin">管理员</SelectItem>
                            </SelectContent>
                          </Select>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <Button type="submit" className="w-full" disabled={form.formState.isSubmitting}>
                      {form.formState.isSubmitting ? "创建中..." : "添加用户"}
                    </Button>
                  </form>
                </Form>
              </CardContent>
            </Card>
          </div>
        </main>
      </div>
    </AuthGuard>
  )
}

