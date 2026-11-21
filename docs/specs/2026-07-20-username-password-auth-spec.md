# Spec: Username/Password 注册登录

> 日期: 2026-07-20 | 版本: v0.1.0 | 分支: dev

## 概述

在现有 GitHub OAuth 登录基础上，新增邮箱 + 密码的注册/登录流程。两者共用 `users` 表和 JWT 鉴权体系，不影响现有 GitHub 登录行为。

---

## D1 — 核心概念定义

### 业务实体

- **User**：系统唯一身份标识，通过 `id` (UUID) 确定
- **Auth Provider**：用户的认证来源，由字段 NULL 判断 — `github_id IS NOT NULL` 为 GitHub 用户，`email IS NOT NULL AND github_id IS NULL` 为邮箱用户
- **同一人的两种登录方式不关联**：本期不做账号绑定，GitHub 用户和邮箱用户是独立记录

### 核心约束

- `github_id` 改为 nullable，`email` 新增唯一部分索引
- 注册即登录，成功后直接签发 JWT
- 密码使用 bcrypt 哈希

---

## D2 — 数据库建模

### Migration: `000003_add_local_auth.up.sql`

```sql
ALTER TABLE users
  ALTER COLUMN github_id DROP NOT NULL,
  ADD COLUMN email VARCHAR(255),
  ADD COLUMN password_hash VARCHAR(255);

CREATE UNIQUE INDEX idx_users_email
  ON users(email) WHERE email IS NOT NULL AND github_id IS NULL;

CREATE INDEX idx_users_email_lookup
  ON users(email) WHERE email IS NOT NULL;
```

### Migration: `000003_add_local_auth.down.sql`

```sql
DROP INDEX IF EXISTS idx_users_email_lookup;
DROP INDEX IF EXISTS idx_users_email;

ALTER TABLE users
  DROP COLUMN password_hash,
  DROP COLUMN email,
  ALTER COLUMN github_id SET NOT NULL;
```

### Model 层变更

```go
type User struct {
    ID              string    `json:"id"`
    GitHubID        *int64    `json:"-"`           // int64 → *int64 (nullable)
    GitHubLogin     string    `json:"github_login"`
    GitHubAvatarURL string    `json:"github_avatar_url"`
    Email           string    `json:"-"`           // 新增，不序列化
    DisplayName     string    `json:"display_name"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}
```

---

## D3 — API 设计

### `POST /auth/register`

注册新用户（邮箱 + 密码）。

```
Request:
{
  "email": "deven@example.com",
  "password": "hunter2abc"
}

Response 201:
{
  "data": {
    "id": "uuid",
    "github_login": "",
    "github_avatar_url": "",
    "display_name": "deven",
    "created_at": "...",
    "updated_at": "..."
  },
  "error": null
}

Errors:
409 CONFLICT       — 邮箱已被注册
422 VALIDATION     — 邮箱格式不合法 / 密码 < 8 字符
```

- 成功后 Set-Cookie `jwt`，注册即登录
- `display_name` 默认取邮箱 `@` 前部分

### `POST /auth/login`

邮箱密码登录。

```
Request:
{
  "email": "deven@example.com",
  "password": "hunter2abc"
}

Response 200: 同 register 的 User 结构

Errors:
401 UNAUTHORIZED   — 邮箱或密码错误（不区分是邮箱不存在还是密码错）
```

- 成功后 Set-Cookie `jwt`

### 现有端点不变

- `GET /auth/github/login` — GitHub OAuth 入口
- `GET /auth/github/callback` — GitHub OAuth 回调
- `GET /api/v1/auth/me` — 获取当前用户
- `POST /api/v1/auth/logout` — 登出

### 错误码扩充

```go
ErrCodeConflict = "CONFLICT"  // 409
```

---

## D4 — 风险识别

| # | 风险 | 等级 | 缓解 |
|---|------|------|------|
| 1 | 暴力破解 — login 无速率限制 | 🔴 高 | 建议后续加 per-IP rate limit（MVP 不做） |
| 2 | 用户枚举 — register 返回 409 | 🟡 中 | 注册接口天然泄露；建议加注册速率限制（MVP 不做） |
| 3 | 弱密码 — 仅 ≥ 8 字符 | 🟡 中 | MVP 可接受，后续加黑名单 |
| 4 | GitHub 与邮箱用户无法关联 | 🟢 低 | 产品决策：本期不做账号绑定 |
| 5 | password_hash 泄露 | 🟢 低 | User 模型的 password_hash 不参与 JSON 序列化 |

---

## D5 — 前端展示

### 组件结构

```
LoginPage
├── LoginRegisterTabs
│   ├── Tab: "登录"
│   │   └── LoginForm (email + password + submit)
│   ├── Tab: "注册"
│   │   └── RegisterForm (email + password + confirm_password + submit)
│   └── "Login with GitHub" 按钮 (两 Tab 下均显示)
```

### 交互细节

1. Tab 切换不清空表单，email 字段自动带过去
2. 登录/注册成功 → `checkAuth()` 刷新 → 跳转 `/todos`
3. GitHub 按钮始终显示，行为不变
4. 提交中 button disabled + spinner

### API Client 新增

```typescript
register(email: string, password: string): Promise<User>
login(email: string, password: string): Promise<User>
```

### 路由

不变。`/login` → LoginPage → 双 Tab 组件。

---

## 任务清单

- [ ] 数据库 migration: `000003_add_local_auth`
- [ ] Model 层: `User` 结构体 `GitHubID` → `*int64`，新增 `Email` 字段
- [ ] Repository: `CreateUserByEmail`、`GetUserByEmail`
- [ ] Handler: `Register`、`Login`
- [ ] Config: `GitHubClientID/Secret` 改为 optional（邮箱用户不需要）
- [ ] 路由注册: `POST /auth/register`、`POST /auth/login`
- [ ] 前端 API Client: `register()`、`login()`
- [ ] 前端 LoginPage: 双 Tab 容器 + LoginForm + RegisterForm
- [ ] 后端测试: register/login handler test
- [ ] 前端测试: LoginPage/Form 测试
