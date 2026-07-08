# TodoDemo — 5D 架构规格说明书

> 里程碑: **0.1.0**
> 创建日期: 2026-07-08
> 技术栈: React 18 + Go 1.22+ + PostgreSQL 16

---

## D1 — Core Concept Definition（核心概念定义）

### 业务实体

| 实体 | 描述 |
|---|---|
| **User** | 通过 GitHub OAuth 登录的独立用户。每个用户拥有独立的 TODO 列表 |
| **Todo** | 一条待办事项，属于某个用户。包含标题、描述、完成状态 |

### 领域边界

| 范围 | 包含 | 不包含（YAGNI） |
|---|---|---|
| **In Scope** | GitHub OAuth 登录、TODO 增删改查、完成/取消完成、软删除、分页列表 | |
| **Out of Scope** | 标签系统、优先级、截止日期、多用户协作、搜索、排序自定义、数据导出 | |

### 核心约束

- 一个用户只能看到自己的 TODO
- 软删除不展示在前端，但数据保留
- 所有 API 需经过 JWT 认证（除 OAuth 端点）

---

## D2 — Database Modeling（数据库建模）

### 表结构

#### `users`

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| `id` | `UUID` | `PK DEFAULT gen_random_uuid()` | 主键 |
| `github_id` | `BIGINT` | `UNIQUE NOT NULL` | GitHub 用户 ID |
| `github_login` | `VARCHAR(255)` | `NOT NULL` | GitHub 用户名 |
| `github_avatar_url` | `TEXT` | | 头像 URL |
| `display_name` | `VARCHAR(255)` | | 展示名（优先使用 GitHub name） |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | |
| `updated_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | |

#### `todos`

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| `id` | `UUID` | `PK DEFAULT gen_random_uuid()` | 主键 |
| `user_id` | `UUID` | `FK → users(id) NOT NULL` | 所属用户 |
| `title` | `VARCHAR(500)` | `NOT NULL` | 标题 |
| `description` | `TEXT` | | 详细描述（可选） |
| `completed` | `BOOLEAN` | `NOT NULL DEFAULT false` | 完成状态 |
| `completed_at` | `TIMESTAMPTZ` | | 完成时间戳 |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | |
| `updated_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | |
| `deleted_at` | `TIMESTAMPTZ` | | 软删除标记（NULL = 未删除） |

### 索引

```sql
CREATE INDEX idx_todos_user_active ON todos(user_id, deleted_at, created_at DESC);
CREATE INDEX idx_todos_user_completed ON todos(user_id, completed) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_users_github_id ON users(github_id);
```

### 迁移文件

采用 `golang-migrate` 格式：

```
migrations/
├── 000001_create_users.up.sql
├── 000001_create_users.down.sql
├── 000002_create_todos.up.sql
└── 000002_create_todos.down.sql
```

---

## D3 — API Design（API 设计）

### 基础信息

- Base URL: `http://localhost:8080`
- API 前缀: `/api/v1`
- 认证方式: JWT (httpOnly Cookie, SameSite=Lax)
- 内容类型: `application/json`

### 端点列表

#### Auth

| 方法 | 路径 | 认证 | 说明 |
|---|---|---|---|
| `GET` | `/auth/github/login` | 否 | 302 重定向至 GitHub OAuth |
| `GET` | `/auth/github/callback` | 否 | OAuth 回调 → 写入 JWT cookie → 302 至前端 |
| `POST` | `/api/v1/auth/logout` | 是 | 清除 JWT cookie |

#### Todos

| 方法 | 路径 | 认证 | 说明 |
|---|---|---|---|
| `GET` | `/api/v1/todos` | 是 | 分页列表（query: `completed`, `page`, `per_page`） |
| `POST` | `/api/v1/todos` | 是 | 创建 TODO |
| `PUT` | `/api/v1/todos/{id}` | 是 | 更新 TODO |
| `DELETE` | `/api/v1/todos/{id}` | 是 | 软删除 TODO |

### 请求 / 响应示例

#### 创建 TODO

```http
POST /api/v1/todos
Cookie: jwt=...
Content-Type: application/json

{
  "title": "买咖啡豆",
  "description": "哥伦比亚蕙兰，中度烘焙"
}
```

```http
201 Created

{
  "data": {
    "id": "a1b2c3d4-...",
    "title": "买咖啡豆",
    "description": "哥伦比亚蕙兰，中度烘焙",
    "completed": false,
    "completed_at": null,
    "created_at": "2026-07-08T16:00:00Z",
    "updated_at": "2026-07-08T16:00:00Z"
  },
  "error": null
}
```

#### 列表查询

```http
GET /api/v1/todos?completed=false&page=1&per_page=20
Cookie: jwt=...
```

```http
200 OK

{
  "data": [ ... ],
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 42
  },
  "error": null
}
```

### 统一错误格式

```json
{
  "data": null,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message"
  }
}
```

| 错误码 | HTTP 状态 | 场景 |
|---|---|---|
| `UNAUTHORIZED` | 401 | JWT 缺失/过期/无效 |
| `NOT_FOUND` | 404 | TODO 不存在或不属当前用户 |
| `VALIDATION_ERROR` | 400 | 请求体验证失败 |
| `INTERNAL_ERROR` | 500 | 服务器内部错误 |

### 安全设计

- JWT 有效期为 7 天
- Cookie 设置 `HttpOnly; SameSite=Lax; Path=/`
- 生产环境启用 `Secure` 标志
- GitHub OAuth state 参数 + PKCE 防 CSRF
- 所有参数化查询防 SQL 注入

---

## D4 — Risk Identification（风险识别）

| # | 风险 | 可能性 | 影响 | 缓解措施 |
|---|---|---|---|---|
| R1 | GitHub OAuth code 被截获重放 | 低 | 高 | 校验 state 参数；GitHub code 一次性使用 |
| R2 | JWT 泄露（XSS） | 低 | 高 | httpOnly cookie 防 JS 读取；React 默认转义防 XSS |
| R3 | 并发写覆盖（无乐观锁） | 中 | 中 | MVP 先不加；后续 `WHERE updated_at = ?` 乐观锁 |
| R4 | 软删除数据膨胀 | 低 | 低 | MVP 不考虑；后续加定期清理或 hard delete 后台任务 |
| R5 | PostgreSQL 连接泄漏 | 中 | 高 | 使用连接池（`pgxpool`），确保 rows 关闭 |
| R6 | 无 HTTPS 时 JWT 明文传输 | 低（开发） | 高 | 生产强制 HTTPS；开发环境 localhost 豁免 |
| R7 | 功能 creep 提前加标签/优先级 | 高 | 中 | YAGNI 第一原则：只有 title + description + completed |

---

## D5 — Frontend Presentation（前端展示）

### 路由

| 路径 | 组件 | 说明 |
|---|---|---|
| `/` | `LoginPage` | 未登录 → GitHub 登录按钮；已登录 → 重定向到 `/todos` |
| `/todos` | `TodoPage` | 主面板，未登录 → 重定向到 `/` |
| `*` | `NotFound` | 404 页面 |

### 组件树

```
<App>
├── <LoginPage>
│   └── "Login with GitHub" 按钮
└── <ProtectedRoute>
    └── <TodoPage>
        ├── <Header>
        │   ├── Logo / 标题 "TodoDemo"
        │   ├── 用户头像 (GitHub avatar)
        │   └── 登出按钮
        ├── <TodoForm>
        │   └── 输入框 + "添加" 按钮 (Enter 提交)
        ├── <TodoFilter>
        │   └── 全部 / 未完成 / 已完成 (三个 tab)
        └── <TodoList>
            └── <TodoItem> × N
                ├── <Checkbox> — 切换完成状态
                ├── <Title> — 双击进入编辑模式
                ├── <DeleteButton> — 软删除
                └── <Timestamp> — 创建时间 / 完成时间
```

### 状态管理

- 使用 `React Context + useReducer`
- 不引入 Redux / Zustand（当前复杂度不够）

### 状态与视图映射

| 状态 | 视图 |
|---|---|
| 加载中 | 居中 spinner |
| 空列表 | "还没有 TODO，创建一个吧 🎉" + 空状态插图 |
| 网络错误 | 错误提示 + 重试按钮 |
| 提交失败 | 行内 toast 提示 |

### 交互设计

- Enter 提交新 TODO，Esc 取消编辑模式
- Checkbox 切换 → 乐观更新 UI，API 失败后回滚
- 删除操作：MVP 不做确认弹窗（软删除可恢复）
- 双击条目标题 → 行内编辑

---

## 版本记录

| 版本 | 日期 | 变更说明 |
|---|---|---|
| 0.1.0 | 2026-07-08 | 初始规格发布 |
