// 用户角色类型
export type UserRole = "normal" | "platform" | "super"

// 权限检查工具函数

/**
 * 检查用户是否有指定角色或更高权限
 */
export function hasRole(userRole: string | undefined, requiredRole: UserRole): boolean {
  if (!userRole) {
    return false
  }

  switch (requiredRole) {
    case "super":
      // 只有超级管理员可以访问
      return userRole === "super"
    case "platform":
      // 平台管理员和超级管理员可以访问
      return userRole === "platform" || userRole === "super"
    case "normal":
      // 所有用户都可以访问
      return true
    default:
      return false
  }
}

/**
 * 检查用户是否可以访问用户管理功能
 */
export function canManageUsers(userRole: string | undefined): boolean {
  return hasRole(userRole, "super")
}

/**
 * 检查用户是否可以访问日志审计功能
 */
export function canAccessAudit(userRole: string | undefined): boolean {
  return hasRole(userRole, "platform")
}

/**
 * 检查用户是否可以修改资源节点
 */
export function canModifyResources(userRole: string | undefined): boolean {
  return hasRole(userRole, "platform")
}

/**
 * 获取用户角色显示名称
 */
export function getRoleDisplayName(role: string | undefined): string {
  switch (role) {
    case "super":
      return "超级管理员"
    case "platform":
      return "平台管理员"
    case "normal":
      return "普通用户"
    default:
      return "普通用户"
  }
}
