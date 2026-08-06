# People

People 是企业内部员工信息系统，账号体系与 Sign-in 完全隔离，不提供公开注册能力。

- `backend/`：Go + CloudWeGo Hertz 后端，默认监听 `8085`。
- `frontend/`：Vue 3 管理前端，默认监听 `5177`。
- 业务入口：`http://localhost:8082/api/open/people/**`，所有前后端和 OAuth 交互都必须经过 Gateway Open；People 后端会拒绝未携带合法 Gateway 上游签名的直连业务请求。

## 默认管理员

首次启动会幂等创建系统管理员：

```text
用户名：admin
密码：admin
```

生产环境首次登录后应立即修改默认密码。

## 员工首次登录

管理员创建员工时不设置初始密码。只要员工仍处于 `mustChangePassword=true` 状态，登录就不校验输入密码，并强制跳转到修改密码页面；此时除查看当前会话、退出和修改密码外的业务请求都会被拒绝。成功设置新密码后，该标志才会清除，后续登录严格校验新密码。

## OAuth

People 提供 OAuth 2.0 授权码模式供内部系统登录，并提供客户端凭证模式供可信后端同步员工目录。默认预置 Permission 客户端：

```text
Client ID: permission-ui
Client Secret: permission-local-client-secret-change-me
Redirect URI: http://localhost:5173/oauth/callback
```

生产环境必须通过环境变量替换 Client Secret，并配置准确的回调地址。

## 本地启动

先启动 Gateway，再分别启动后端和前端：

```bash
cd backend
cp .env.example .env
go run ./cmd/server

cd ../frontend
npm install
npm run dev
```

打开 `http://localhost:5177`。前端会把 `/api/open/people` 请求代理到 Gateway 运行时 `http://127.0.0.1:8082`。

## 验证

```bash
cd backend
gofmt -w ./cmd ./internal
go test ./...

cd ../frontend
npm run typecheck
npm run build
```
