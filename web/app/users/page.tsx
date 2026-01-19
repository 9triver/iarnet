"use client"

import { useState, useEffect } from "react"
import { useRouter } from "next/navigation"
import { Sidebar } from "@/components/sidebar"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { useForm } from "react-hook-form"
import { Plus, Edit, Trash2, RefreshCw, Lock, Unlock, Users as UsersIcon } from "lucide-react"
import { usersAPI, getErrorMessage, APIError } from "@/lib/api"
import type { UserInfo, CreateUserRequest, UpdateUserRequest } from "@/lib/api"
import { AuthGuard } from "@/components/auth-guard"
import { canManageUsers } from "@/lib/permissions"
import { useIARNetStore } from "@/lib/store"
import { toast } from "sonner"
import { Skeleton } from "@/components/ui/skeleton"

interface UserFormValues {
  name: string
  password: string
  role: string
}

export default function UsersPage() {
  const router = useRouter()
  const currentUser = useIARNetStore((state) => state.currentUser)
  const [users, setUsers] = useState<UserInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [editingUser, setEditingUser] = useState<UserInfo | null>(null)
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [userToDelete, setUserToDelete] = useState<string | null>(null)

  const createForm = useForm<UserFormValues>({
    defaultValues: {
      name: "",
      password: "",
      role: "normal",
    },
  })

  const editForm = useForm<UserFormValues>({
    defaultValues: {
      name: "",
      password: "",
      role: "normal",
    },
  })

  // 检查权限（使用变量而不是提前返回，避免违反 React Hooks 规则）
  const hasPermission = canManageUsers(currentUser?.role)

  // 对于没有权限的已登录用户，直接重定向到资源管理页面
  useEffect(() => {
    if (currentUser && !hasPermission) {
      router.replace("/resources")
    }
  }, [currentUser, hasPermission, router])

  const fetchUsers = async () => {
    // 如果没有权限或用户未登录，不执行 API 调用
    if (!hasPermission || !currentUser) return
    
    try {
      setLoading(true)
      const response = await usersAPI.getUsers()
      setUsers(response.users || [])
    } catch (error) {
      console.error("Failed to fetch users:", error)
      // 如果是 401、403 或 400 错误，可能是权限问题或未认证，不显示错误提示
      if (error instanceof Error && (
        error.message.includes("401") || 
        error.message.includes("403") || 
        error.message.includes("400") ||
        error.message.includes("Unauthorized") ||
        error.message.includes("Forbidden")
      )) {
        return
      }
      const errorMessage = error instanceof APIError 
        ? getErrorMessage(error.message) 
        : error instanceof Error 
        ? error.message 
        : "未知错误"
      toast.error("获取用户列表失败", {
        description: errorMessage,
      })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    // 只有在有权限且用户已登录时才获取用户列表
    if (hasPermission && currentUser) {
      fetchUsers()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hasPermission, currentUser])

  const handleCreate = async (values: UserFormValues) => {
    // 验证密码复杂度
    const { validatePasswordComplexity } = await import("@/lib/utils")
    const passwordError = validatePasswordComplexity(values.password)
    if (passwordError) {
      toast.error(passwordError)
      return
    }

    try {
      await usersAPI.createUser({
        name: values.name,
        password: values.password,
        role: values.role,
      })
      toast.success("用户创建成功")
      setIsDialogOpen(false)
      createForm.reset()
      await fetchUsers()
    } catch (error) {
      const errorMessage = error instanceof APIError 
        ? getErrorMessage(error.message) 
        : error instanceof Error 
        ? error.message 
        : "未知错误"
      toast.error("创建用户失败", {
        description: errorMessage,
      })
    }
  }

  const handleEdit = (user: UserInfo) => {
    setEditingUser(user)
    editForm.setValue("name", user.name)
    editForm.setValue("role", user.role)
    editForm.setValue("password", "")
    setIsEditDialogOpen(true)
  }

  const handleUpdate = async (values: UserFormValues) => {
    if (!editingUser) return

    // 如果提供了新密码，验证密码复杂度
    if (values.password) {
      const { validatePasswordComplexity } = await import("@/lib/utils")
      const passwordError = validatePasswordComplexity(values.password)
      if (passwordError) {
        toast.error(passwordError)
        return
      }
    }

    try {
      const updateData: UpdateUserRequest = {}
      if (values.password) {
        updateData.password = values.password
      }
      if (values.role !== editingUser.role) {
        updateData.role = values.role
      }

      await usersAPI.updateUser(editingUser.name, updateData)
      toast.success("用户更新成功")
      setIsEditDialogOpen(false)
      setEditingUser(null)
      editForm.reset()
      await fetchUsers()
    } catch (error) {
      const errorMessage = error instanceof APIError 
        ? getErrorMessage(error.message) 
        : error instanceof Error 
        ? error.message 
        : "未知错误"
      toast.error("更新用户失败", {
        description: errorMessage,
      })
    }
  }

  const handleDeleteClick = (username: string) => {
    setUserToDelete(username)
    setDeleteDialogOpen(true)
  }

  const handleDelete = async () => {
    if (!userToDelete) return

    try {
      await usersAPI.deleteUser(userToDelete)
      toast.success("用户删除成功")
      setDeleteDialogOpen(false)
      setUserToDelete(null)
      await fetchUsers()
    } catch (error) {
      const errorMessage = error instanceof APIError 
        ? getErrorMessage(error.message) 
        : error instanceof Error 
        ? error.message 
        : "未知错误"
      toast.error("删除用户失败", {
        description: errorMessage,
      })
    }
  }

  const handleUnlock = async (username: string) => {
    try {
      await usersAPI.unlockUser(username)
      toast.success("用户解锁成功")
      await fetchUsers()
    } catch (error) {
      const errorMessage = error instanceof APIError 
        ? getErrorMessage(error.message) 
        : error instanceof Error 
        ? error.message 
        : "未知错误"
      toast.error("解锁用户失败", {
        description: errorMessage,
      })
    }
  }

  const getRoleBadge = (role: string) => {
    switch (role) {
      case "super":
        return <Badge variant="destructive">超级管理员</Badge>
      case "platform":
        return <Badge variant="default">平台管理员</Badge>
      case "normal":
        return <Badge variant="secondary">普通用户</Badge>
      default:
        return <Badge variant="secondary">{role}</Badge>
    }
  }

  // 无权限时不展示具体内容，等待重定向到资源管理页面
  if (currentUser && !hasPermission) {
    return (
      <AuthGuard>
        <div className="flex h-screen bg-background">
          <Sidebar />
          <main className="flex-1 overflow-auto" />
        </div>
      </AuthGuard>
    )
  }

  return (
    <AuthGuard>
      <div className="flex h-screen bg-background">
        <Sidebar />
        <main className="flex-1 overflow-auto">
          <div className="p-8">
            <div className="flex items-center justify-between mb-8">
              <div>
                <h1 className="text-3xl font-playfair font-bold text-foreground mb-2">用户管理</h1>
                <p className="text-muted-foreground">管理系统用户、角色和权限</p>
              </div>
              <div className="flex items-center space-x-3">
                <Button variant="outline" onClick={fetchUsers} disabled={loading}>
                  <RefreshCw className={`h-4 w-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
                  刷新
                </Button>
                <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
                  <DialogTrigger asChild>
                    <Button>
                      <Plus className="h-4 w-4 mr-2" />
                      创建用户
                    </Button>
                  </DialogTrigger>
                  <DialogContent>
                    <DialogHeader>
                      <DialogTitle>创建用户</DialogTitle>
                      <DialogDescription>创建新的系统用户</DialogDescription>
                    </DialogHeader>
                    <Form {...createForm}>
                      <form onSubmit={createForm.handleSubmit(handleCreate)} className="space-y-4">
                        <FormField
                          control={createForm.control}
                          name="name"
                          rules={{ required: "请输入用户名" }}
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>用户名</FormLabel>
                              <FormControl>
                                <Input placeholder="用户名" {...field} />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <FormField
                          control={createForm.control}
                          name="password"
                          rules={{ 
                            required: "请输入密码",
                            validate: async (value) => {
                              if (!value) return true // required 已经处理了空值
                              const { validatePasswordComplexity } = await import("@/lib/utils")
                              return validatePasswordComplexity(value) || true
                            }
                          }}
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>密码</FormLabel>
                              <FormControl>
                                <Input 
                                  type="password" 
                                  placeholder="8-16位，包含大小写字母、数字、特殊字符" 
                                  {...field} 
                                />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <FormField
                          control={createForm.control}
                          name="role"
                          rules={{ required: "请选择角色" }}
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>角色</FormLabel>
                              <Select onValueChange={field.onChange} defaultValue={field.value}>
                                <FormControl>
                                  <SelectTrigger>
                                    <SelectValue placeholder="选择角色" />
                                  </SelectTrigger>
                                </FormControl>
                                <SelectContent>
                                  <SelectItem value="normal">普通用户</SelectItem>
                                  <SelectItem value="platform">平台管理员</SelectItem>
                                  <SelectItem value="super">超级管理员</SelectItem>
                                </SelectContent>
                              </Select>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <DialogFooter>
                          <Button type="button" variant="outline" onClick={() => setIsDialogOpen(false)}>
                            取消
                          </Button>
                          <Button type="submit">创建</Button>
                        </DialogFooter>
                      </form>
                    </Form>
                  </DialogContent>
                </Dialog>
              </div>
            </div>

            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <UsersIcon className="h-5 w-5" />
                  用户列表
                </CardTitle>
                <CardDescription>管理系统中的所有用户账户</CardDescription>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>用户名</TableHead>
                      <TableHead>角色</TableHead>
                      <TableHead>状态</TableHead>
                      <TableHead>失败次数</TableHead>
                      <TableHead>锁定到期时间</TableHead>
                      <TableHead>操作</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {loading ? (
                      Array.from({ length: 5 }).map((_, i) => (
                        <TableRow key={i}>
                          <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                          <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                          <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                          <TableCell><Skeleton className="h-4 w-12" /></TableCell>
                          <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                          <TableCell><Skeleton className="h-8 w-24" /></TableCell>
                        </TableRow>
                      ))
                    ) : users.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={6} className="text-center py-8 text-muted-foreground">
                          暂无用户
                        </TableCell>
                      </TableRow>
                    ) : (
                      users.map((user) => (
                        <TableRow key={user.name}>
                          <TableCell className="font-medium">{user.name}</TableCell>
                          <TableCell>{getRoleBadge(user.role)}</TableCell>
                          <TableCell>
                            {user.locked ? (
                              <Badge variant="destructive">已锁定</Badge>
                            ) : (
                              <Badge variant="default" className="bg-green-500">正常</Badge>
                            )}
                          </TableCell>
                          <TableCell>{user.failed_count}</TableCell>
                          <TableCell className="text-sm text-muted-foreground">
                            {user.locked_until || "-"}
                          </TableCell>
                          <TableCell>
                            <div className="flex items-center gap-2">
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => handleEdit(user)}
                              >
                                <Edit className="h-4 w-4" />
                              </Button>
                              {user.locked && (
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => handleUnlock(user.name)}
                                  title="解锁用户"
                                >
                                  <Unlock className="h-4 w-4" />
                                </Button>
                              )}
                              {user.name !== currentUser?.username && (
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => handleDeleteClick(user.name)}
                              >
                                <Trash2 className="h-4 w-4" />
                              </Button>
                              )}
                            </div>
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>

            {/* 编辑用户对话框 */}
            <Dialog open={isEditDialogOpen} onOpenChange={setIsEditDialogOpen}>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>编辑用户</DialogTitle>
                  <DialogDescription>修改用户信息</DialogDescription>
                </DialogHeader>
                <Form {...editForm}>
                  <form onSubmit={editForm.handleSubmit(handleUpdate)} className="space-y-4">
                    <FormField
                      control={editForm.control}
                      name="name"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>用户名</FormLabel>
                          <FormControl>
                            <Input {...field} disabled />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={editForm.control}
                      name="password"
                      rules={{
                        validate: async (value) => {
                          if (!value) return true // 留空则不修改
                          const { validatePasswordComplexity } = await import("@/lib/utils")
                          return validatePasswordComplexity(value) || true
                        }
                      }}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>新密码（留空则不修改）</FormLabel>
                          <FormControl>
                            <Input 
                              type="password" 
                              placeholder="留空则不修改密码，否则需8-16位，包含大小写字母、数字、特殊字符" 
                              {...field} 
                            />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={editForm.control}
                      name="role"
                      rules={{ required: "请选择角色" }}
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
                              <SelectItem value="normal">普通用户</SelectItem>
                              <SelectItem value="platform">平台管理员</SelectItem>
                              <SelectItem value="super">超级管理员</SelectItem>
                            </SelectContent>
                          </Select>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <DialogFooter>
                      <Button type="button" variant="outline" onClick={() => setIsEditDialogOpen(false)}>
                        取消
                      </Button>
                      <Button type="submit">保存</Button>
                    </DialogFooter>
                  </form>
                </Form>
              </DialogContent>
            </Dialog>

            {/* 删除用户确认对话框 */}
            <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>确认删除用户</AlertDialogTitle>
                  <AlertDialogDescription>
                    确定要删除用户 <strong>"{userToDelete}"</strong> 吗？此操作不可恢复。
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel onClick={() => setUserToDelete(null)}>取消</AlertDialogCancel>
                  <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
                    删除
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        </main>
      </div>
    </AuthGuard>
  )
}
