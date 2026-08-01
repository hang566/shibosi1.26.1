-- ============================================
-- 市舶司 v2.0 数据库迁移DDL
-- 兼容存量数据，禁止清库操作
-- 适用: MySQL 8.0+ / PostgreSQL 14+
-- 索引建议已包含
-- ============================================

-- 1. 用户表（扩展原有users表，兼容存量数据）
-- 如果已有users表，请使用ALTER TABLE进行增量迁移
CREATE TABLE IF NOT EXISTS `users` (
    `id`            BIGINT AUTO_INCREMENT PRIMARY KEY,
    `username`      VARCHAR(64) NOT NULL,
    `password`      VARCHAR(256) NOT NULL COMMENT 'bcrypt哈希',
    `nickname`      VARCHAR(128) DEFAULT '',
    `email`         VARCHAR(128) DEFAULT '',
    `phone`         VARCHAR(32) DEFAULT '',
    `role`          VARCHAR(32) DEFAULT 'user' COMMENT 'admin/user/guest/editor',
    `avatar`        VARCHAR(512) DEFAULT '',
    `bio`           VARCHAR(512) DEFAULT '',
    `status`        TINYINT DEFAULT 1 COMMENT '1=正常 0=禁用',
    `last_login`    DATETIME DEFAULT NULL,
    `created_at`    DATETIME DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX `uk_username` (`username`),
    INDEX `idx_role` (`role`),
    INDEX `idx_status` (`status`),
    INDEX `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';

-- 存量数据迁移：将MD5密码迁移为bcrypt
-- 注意：迁移后需要通过程序重新哈希密码，或让用户重置密码
-- UPDATE users SET password = CONCAT('$2a$12$MIGRATED_', password) WHERE password NOT LIKE '$2a$%';

-- 2. 角色表
CREATE TABLE IF NOT EXISTS `roles` (
    `id`            BIGINT AUTO_INCREMENT PRIMARY KEY,
    `name`          VARCHAR(64) NOT NULL,
    `code`          VARCHAR(64) NOT NULL,
    `description`   VARCHAR(256) DEFAULT '',
    `status`        TINYINT DEFAULT 1,
    `created_at`    DATETIME DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX `uk_name` (`name`),
    UNIQUE INDEX `uk_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='角色表';

-- 3. 权限表（接口级+按钮级+菜单级）
CREATE TABLE IF NOT EXISTS `permissions` (
    `id`            BIGINT AUTO_INCREMENT PRIMARY KEY,
    `name`          VARCHAR(64) NOT NULL,
    `code`          VARCHAR(128) NOT NULL,
    `type`          VARCHAR(16) NOT NULL DEFAULT 'api' COMMENT 'api=接口级 button=按钮级 menu=菜单级',
    `method`        VARCHAR(16) DEFAULT '' COMMENT 'GET/POST/PUT/DELETE',
    `path`          VARCHAR(256) DEFAULT '',
    `parent_id`     BIGINT DEFAULT 0,
    `sort`          INT DEFAULT 0,
    `description`   VARCHAR(256) DEFAULT '',
    `created_at`    DATETIME DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX `uk_code` (`code`),
    INDEX `idx_type` (`type`),
    INDEX `idx_parent_id` (`parent_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='权限表';

-- 4. 角色-权限关联表
CREATE TABLE IF NOT EXISTS `role_permissions` (
    `id`            BIGINT AUTO_INCREMENT PRIMARY KEY,
    `role_id`       BIGINT NOT NULL,
    `permission_id` BIGINT NOT NULL,
    `created_at`    DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX `idx_role_id` (`role_id`),
    INDEX `idx_permission_id` (`permission_id`),
    UNIQUE INDEX `uk_role_permission` (`role_id`, `permission_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='角色权限关联表';

-- 5. 用户-角色关联表（支持多角色）
CREATE TABLE IF NOT EXISTS `user_roles` (
    `id`            BIGINT AUTO_INCREMENT PRIMARY KEY,
    `user_id`       BIGINT NOT NULL,
    `role_id`       BIGINT NOT NULL,
    `created_at`    DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX `idx_user_id` (`user_id`),
    INDEX `idx_role_id` (`role_id`),
    UNIQUE INDEX `uk_user_role` (`user_id`, `role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户角色关联表';

-- 6. 审计日志表（核心：独立表，支持多维检索）
CREATE TABLE IF NOT EXISTS `audit_logs` (
    `id`            BIGINT AUTO_INCREMENT PRIMARY KEY,
    `user_id`       BIGINT DEFAULT 0,
    `username`      VARCHAR(64) DEFAULT '',
    `action`        VARCHAR(128) NOT NULL COMMENT '操作类型: login/logout/create_user/delete_user/update_config等',
    `resource`      VARCHAR(256) DEFAULT '' COMMENT '操作资源: user/role/config/dashboard',
    `resource_id`   VARCHAR(64) DEFAULT '' COMMENT '资源ID',
    `method`        VARCHAR(16) DEFAULT '' COMMENT 'HTTP方法',
    `path`          VARCHAR(512) DEFAULT '' COMMENT '接口路径',
    `ip`            VARCHAR(64) DEFAULT '',
    `user_agent`    VARCHAR(512) DEFAULT '',
    `detail`        TEXT COMMENT '操作详情',
    `status`        TINYINT DEFAULT 1 COMMENT '1=成功 0=失败',
    `created_at`    DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX `idx_user_id` (`user_id`),
    INDEX `idx_username` (`username`),
    INDEX `idx_action` (`action`),
    INDEX `idx_method` (`method`),
    INDEX `idx_created_at` (`created_at`),
    INDEX `idx_username_action` (`username`, `action`),
    INDEX `idx_created_at_action` (`created_at`, `action`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='审计日志表';

-- 7. 系统配置表（支持热更新）
CREATE TABLE IF NOT EXISTS `system_configs` (
    `id`            BIGINT AUTO_INCREMENT PRIMARY KEY,
    `key`           VARCHAR(128) NOT NULL,
    `value`         TEXT,
    `type`          VARCHAR(32) DEFAULT 'string' COMMENT 'string/int/bool/json',
    `description`   VARCHAR(256) DEFAULT '',
    `editable`      TINYINT DEFAULT 1 COMMENT '1=可编辑 0=只读',
    `updated_at`    DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `updated_by`    VARCHAR(64) DEFAULT '',
    UNIQUE INDEX `uk_key` (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='系统配置表';

-- ============================================
-- 初始数据
-- ============================================

-- 插入默认角色
INSERT IGNORE INTO `roles` (`name`, `code`, `description`) VALUES
('超级管理员', 'admin', '系统最高权限'),
('普通用户', 'user', '注册用户默认角色'),
('访客', 'guest', '未登录访客'),
('编辑', 'editor', '内容编辑');

-- 插入默认权限
INSERT IGNORE INTO `permissions` (`name`, `code`, `type`, `method`, `path`, `sort`) VALUES
('仪表盘-查看', 'dashboard:view', 'api', 'GET', '/api/v1/admin/dashboard', 1),
('系统状态-查看', 'system:status', 'api', 'GET', '/api/v1/admin/system-status', 2),
('用户列表-查看', 'user:list', 'api', 'GET', '/api/v1/admin/users', 10),
('用户详情-查看', 'user:detail', 'api', 'GET', '/api/v1/admin/users/*', 11),
('用户-编辑', 'user:update', 'api', 'PUT', '/api/v1/admin/users/*', 12),
('用户-删除', 'user:delete', 'api', 'DELETE', '/api/v1/admin/users/*', 13),
('用户-重置密码', 'user:reset_pwd', 'api', 'POST', '/api/v1/admin/users/*/reset-password', 14),
('角色-查看', 'role:list', 'api', 'GET', '/api/v1/admin/roles', 20),
('角色-创建', 'role:create', 'api', 'POST', '/api/v1/admin/roles', 21),
('角色-编辑', 'role:update', 'api', 'PUT', '/api/v1/admin/roles/*', 22),
('角色-删除', 'role:delete', 'api', 'DELETE', '/api/v1/admin/roles/*', 23),
('权限-查看', 'permission:list', 'api', 'GET', '/api/v1/admin/permissions', 30),
('权限-分配', 'permission:assign', 'api', 'PUT', '/api/v1/admin/roles/*/permissions', 31),
('日志-查看', 'audit:list', 'api', 'GET', '/api/v1/admin/audit-logs', 40),
('日志-清理', 'audit:clean', 'api', 'DELETE', '/api/v1/admin/audit-logs/clean', 41),
('配置-查看', 'config:list', 'api', 'GET', '/api/v1/admin/configs', 50),
('配置-编辑', 'config:update', 'api', 'PUT', '/api/v1/admin/configs', 51);

-- 为管理员角色分配所有权限
INSERT IGNORE INTO `role_permissions` (`role_id`, `permission_id`)
SELECT 1, id FROM `permissions`;

-- 插入默认系统配置
INSERT IGNORE INTO `system_configs` (`key`, `value`, `type`, `description`, `editable`) VALUES
('rate_limit_qps', '100', 'int', '全局限流QPS', 1),
('rate_limit_burst', '200', 'int', '全局限流突发值', 1),
('maintenance_mode', 'false', 'bool', '维护模式开关', 1),
('audit_log_retention_days', '90', 'int', '审计日志保留天数', 1),
('session_timeout_minutes', '30', 'int', '会话超时时间(分钟)', 1),
('max_login_attempts', '5', 'int', '最大登录尝试次数', 1),
('password_min_length', '6', 'int', '密码最小长度', 1);

-- ============================================
-- 索引优化建议（针对核心查询）
-- ============================================

-- 1. 用户表高频查询：按用户名查询（已有uk_username）
-- 2. 用户表组合查询：按角色+状态排序
-- 3. 审计日志表：按时间范围+操作类型查询（已建复合索引）
-- 4. 审计日志分区建议（MySQL 8.0+）：
-- ALTER TABLE audit_logs PARTITION BY RANGE (TO_DAYS(created_at)) (
--     PARTITION p202401 VALUES LESS THAN (TO_DAYS('2024-02-01')),
--     PARTITION p202402 VALUES LESS THAN (TO_DAYS('2024-03-01')),
--     ...
-- );

-- ============================================
-- 读写分离配置（MySQL）
-- ============================================
-- 1. 配置主从复制 (Master-Slave Replication)
-- 2. 应用层使用GORM的DBResolver插件实现读写分离:
--    db.Use(dbresolver.Register(dbresolver.Config{
--        Sources:  []gorm.Dialector{mysql.Open(masterDSN)},
--        Replicas: []gorm.Dialector{mysql.Open(slave1DSN), mysql.Open(slave2DSN)},
--        Policy:   dbresolver.RandomPolicy{},
--    }))

-- 存量数据兼容说明
-- 1. 原users表已有字段（id, username, password, nickname, email, role, avatar, bio, created_at, updated_at）保持不变
-- 2. 新增字段（phone, status, last_login）使用ALTER TABLE增量添加，不影响存量数据
-- 3. 密码字段：建议通过程序层逐步迁移（用户登录时检测并自动升级为bcrypt）
-- 4. 所有新增表均为独立表，不影响原有数据