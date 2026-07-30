---
name: ldap-sync
description: 从 Active Directory 同步用户到 FollowITup 数据库，支持增量更新和新用户导入，确保用户信息和组成员关系与 AD 保持一致
disable-model-invocation: true
---

# LDAP/AD 用户同步

## 前置条件

- `config.yaml` 中 LDAP 连接参数已正确配置
- 服务器能访问 AD 域控制器（端口 389 或 636）
- 绑定用户有足够的目录读取权限

## 同步流程

1. 使用绑定 DN 连接到 AD 服务器
2. 搜索 `base_dn` 范围内的所有用户（根据 `user_filter` 过滤）
3. 提取每个用户的属性：`sAMAccountName`（登录名）、`mail`（邮箱）、`displayName`（显示名）、`memberOf`（组成员关系）
4. 与本地 `users` 表对比：
   - 新增：AD 中存在但本地不存在的用户 → 插入，`auth_source='ldap'`
   - 更新：本地存在但属性变更的用户 → 更新邮箱、显示名
   - 停用：本地存在但 AD 中已删除的用户 → 设置 `is_active = false`（不删除）
5. 输出同步摘要：新增 N 人，更新 M 人，停用 K 人

## 排除规则

- 服务账号（`sAMAccountName` 以 `svc_` 或 `_svc` 开头/结尾）
- 已禁用的 AD 账号（`userAccountControl` 包含 `2`）
- 系统内置账号（`Administrator`、`Guest`、`krbtgt`）

## 同步冲突处理

- 如果本地 `users` 表的 `auth_source='local'`（管理员手动创建的），不与 AD 同步，保留本地数据
- 如果有用户从 AD 中消失又在 30 天内重新出现，恢复 `is_active` 状态而不创建新记录

## 验证步骤

同步完成后执行以下检查：
1. 统计 AD 中符合条件的用户数 vs 本地 `auth_source='ldap'` 用户数，应基本一致
2. 随机抽查 3 个用户，确认属性的完整性
3. 检查没有任何本地管理员账号（`is_admin=true, auth_source='local'`）被误停用
