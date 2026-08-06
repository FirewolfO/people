# People 开发约束

## 系统边界

- `backend/` 是基于 Go、CloudWeGo Hertz、GORM 和 SQLite 的员工目录、独立登录与 OAuth 服务。
- `frontend/` 是基于 Vue 3、TypeScript、Vite 和 Element Plus 的员工管理前端。
- People 与 Sign-in 账号体系完全独立，不提供注册接口。浏览器、内部服务和其他系统都必须通过 Gateway Open 的 `/api/open/people/**` 路由访问 People 业务 API，不得直连后端。
- 后端必须校验 Gateway 上游签名；除 `/health` 外，不提供可绕过 Gateway 的业务入口。

## 安全与账号

- 系统初始化管理员账号固定为 `admin`，初始密码为 `admin`；生产部署后应立即修改。
- 管理员创建员工时不设置初始密码。员工在 `mustChangePassword=true` 阶段首次及后续登录均不校验密码，但只能访问当前会话、退出和修改密码接口；成功设置密码后才恢复标准密码校验和其他业务能力。
- 密码必须使用可靠算法哈希，Session Cookie 必须为 HttpOnly，写请求必须校验 CSRF。密码、OAuth Client Secret、会话令牌和 Gateway SK 不得写入日志。
- OAuth 授权码必须单次使用、短时有效，并严格校验 client、redirect URI 和 state；Client Secret 只允许由服务端持有。

## 工作流

- API、业务规则和持久化分别放在 `backend/internal/api`、`backend/internal/service` 与 `backend/internal/store`，不要把业务判断放入 handler。
- 前端请求统一封装，API 基础路径通过环境变量配置；页面不得直连 People 后端地址。
- 后端修改后运行 `gofmt` 和 `go test ./...`；前端修改后运行 `npm run typecheck` 和 `npm run build`。
- 本目录是独立 Git 仓库，验证后必须提交并推送当前分支。
