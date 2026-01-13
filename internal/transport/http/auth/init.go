package auth

import (
	"context"
	"fmt"

	"github.com/9triver/iarnet/internal/config"
	userrepo "github.com/9triver/iarnet/internal/infra/repository/auth"
	"github.com/sirupsen/logrus"
)

// InitSuperAdmin 初始化超级管理员
// 如果数据库为空且配置中有超级管理员配置，则创建超级管理员账户
// 支持从旧的 users 配置迁移（向后兼容）
func InitSuperAdmin(cfg *config.Config, userRepo userrepo.UserRepo) error {
	ctx := context.Background()

	// 检查数据库中是否已有用户
	count, err := userRepo.Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	logrus.Infof("Checking super admin initialization: database has %d users", count)

	// 如果数据库中已有用户，检查是否已有配置的超级管理员
	if count > 0 {
		// 检查是否已有配置的超级管理员
		if cfg.SuperAdmin != nil && cfg.SuperAdmin.Name != "" {
			existingUser, err := userRepo.GetByName(ctx, cfg.SuperAdmin.Name)
			if err == nil && existingUser != nil {
				// 超级管理员已存在，不需要初始化
				logrus.Infof("Super admin %s already exists in database, skipping initialization", cfg.SuperAdmin.Name)
				return nil
			}
			// 如果配置的超级管理员不存在，但数据库中有其他用户
			// 为了确保系统可用，仍然创建配置的超级管理员（如果数据库为空或只有非超级管理员用户）
			allUsers, err := userRepo.GetAll(ctx)
			hasSuperAdmin := false
			if err == nil {
				for _, u := range allUsers {
					if u.Role == config.RoleSuperAdmin || (u.Role == "" && u.Name == "admin") {
						hasSuperAdmin = true
						break
					}
				}
			}
			if !hasSuperAdmin {
				// 数据库中没有超级管理员，创建配置的超级管理员
				logrus.Warnf("No super admin found in database, creating super admin %s from config", cfg.SuperAdmin.Name)
				// 继续执行创建逻辑
			} else {
				logrus.Warnf("Super admin %s not found in database, but database has other super admin users. Skipping initialization.", cfg.SuperAdmin.Name)
				return nil
			}
		} else {
			// 如果数据库中有用户但没有配置的超级管理员，也不初始化（避免覆盖现有用户）
			logrus.Infof("Database has %d users but no super_admin config, skipping super admin initialization", count)
			return nil
		}
	}

	// 优先使用新的 super_admin 配置
	var superAdminName, superAdminPassword string
	if cfg.SuperAdmin != nil && cfg.SuperAdmin.Name != "" {
		superAdminName = cfg.SuperAdmin.Name
		superAdminPassword = cfg.SuperAdmin.Password
	} else if len(cfg.Users) > 0 {
		// 向后兼容：从旧的 users 配置中获取第一个用户作为超级管理员
		// 如果第一个用户名为 "admin"，则使用它；否则使用第一个用户
		for _, user := range cfg.Users {
			if user.Name == "admin" || superAdminName == "" {
				superAdminName = user.Name
				superAdminPassword = user.Password
				if user.Name == "admin" {
					break
				}
			}
		}
		logrus.Warnf("Using legacy 'users' configuration. Found user '%s', migrating to database. Please update config to use 'super_admin' instead.", superAdminName)
	}

	if superAdminName == "" {
		logrus.Warn("No super admin configured and database is empty. Please create a super admin through the API.")
		return nil
	}

	// 再次检查用户是否已存在（防止并发创建）
	existingUser, err := userRepo.GetByName(ctx, superAdminName)
	if err == nil && existingUser != nil {
		logrus.Infof("Super admin %s already exists in database, skipping initialization", superAdminName)
		return nil
	}

	// 创建超级管理员
	userDAO := &userrepo.UserDAO{
		ID:       superAdminName,
		Name:     superAdminName,
		Password: superAdminPassword,
		Role:     config.RoleSuperAdmin,
	}

	if err := userRepo.Create(ctx, userDAO); err != nil {
		return fmt.Errorf("failed to create super admin: %w", err)
	}

	logrus.Infof("Super admin initialized successfully: %s (password: %s, role: %s)", superAdminName, "***", string(config.RoleSuperAdmin))
	return nil
}
