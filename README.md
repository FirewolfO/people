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

## 部门管理

管理员可在管理前端维护多级部门树，包括上级部门、编码、名称、描述和启停状态。新增或修改普通员工时必须选择一个已启用部门，管理员账号可以不属于部门。部门名称修改后会同步到员工资料；仍有关联员工或下级部门的部门不能删除，修改上级部门时会拒绝自引用和循环层级。旧数据库中的部门文本会在首次升级启动时自动转换为受管理部门。

## 岗位管理

岗位与部门为多对多关系，同一岗位可用于多个部门，一个部门也可配置多个岗位。每名员工必须选择一个已启用且属于其部门的岗位；内置 `admin` 固定使用“系统管理员”岗位。岗位改名会同步员工资料中的岗位名称快照，仍有关联员工的岗位不能停用、删除或移除员工所在部门。

首次启动会幂等预置系统管理员、通用员工及常见 IT 公司岗位，包括管理、研发、架构、前后端、移动端、测试、DevOps、SRE、安全、数据、AI、产品、项目、设计、IT 支持、网络、人力、财务、销售、客户成功和运营等类别。升级旧数据库时，已有自由文本职务会自动映射或迁移为受管理岗位，空职务使用“通用员工”。

## Inner 员工目录

Permission 不读取 People 数据库，而是通过 Gateway Inner 使用独立 AK/SK 实时获取员工和部门目录：

- `GET /api/inner/people/directory/employees`
- `GET /api/inner/people/directory/employees/:id`
- `GET /api/inner/people/directory/departments`
- `GET /api/inner/people/directory/positions`

People 后端对应的上游路径为 `/api/v1/inner/directory/**`，只接受 `PEOPLE_INNER_ACCESS_KEY`、`PEOPLE_INNER_SECRET_KEY` 对应的 Gateway 系统签名。部门响应中的 `parentId` 表示父部门，空值表示顶级部门；岗位响应通过 `departmentIds` 表示可使用该岗位的部门。

## OAuth

People 提供 OAuth 2.0 授权码模式供内部系统登录。Permission 的员工与部门同步使用上面的 Gateway Inner 接口。默认预置 Permission、Gateway Admin、内部 Blog 和 AI Workbench OAuth 客户端：

用户进入 OAuth 授权页后必须明确点击授权，不会因已有 People 登录态而自动跳转。授权页支持临时切换其他 People 账号：切换时只校验该账号并为本次 OAuth 签发授权码，不创建或替换 People 浏览器会话，返回 People 后仍保持原登录身份。

```text
Client ID: permission-ui
Client Secret: permission-local-client-secret-change-me
Redirect URI: Permission 独立控制台和 admin-ui 的 /oauth/callback

Client ID: gateway-admin-ui
Client Secret: gateway-admin-local-client-secret-change-me
Redirect URI: Gateway 独立控制台的 /oauth/callback

Client ID: blog-ui
Client Secret: blog-local-client-secret-change-me
Redirect URI: Blog 独立控制台的 /oauth/callback

Client ID: ai-workbench-ui
Client Secret: ai-workbench-local-client-secret-change-me
Redirect URI: AI Workbench 独立工作台的 /oauth/callback
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

## Docker 与公网部署

仓库根目录的 `compose.yaml` 会启动 People 后端、Gateway-HMAC 边缘代理和 Nginx 前端。浏览器仍只访问 `/api/open/people/**`，边缘代理将请求签名后转发到后端的 `/api/v1/**`；后端不会接受未签名的业务请求。

```bash
export PEOPLE_GATEWAY_ACCESS_KEY='替换为生产 AK'
export PEOPLE_GATEWAY_SECRET_KEY='替换为至少 32 字符的生产 SK'
export PEOPLE_AI_WORKBENCH_CLIENT_SECRET='与 AI Workbench 一致'
export PEOPLE_LINKUP_CLIENT_SECRET='与 IM 服务一致'
docker compose up -d --build
curl http://127.0.0.1:18084/api/open/people/auth/csrf
```

生产源站只监听 `127.0.0.1:18084`，公网入口为 `https://people.lxvb.top`。edge 同时加入仅容器使用的 `people-services` 网络，AI Workbench 与连线通过 `http://people-edge:8082/api/open/people` 调用，不需要开放额外宿主机端口。内置 `admin` 角色在 Permission 服务暂不可达时仍保留 People 管理权限；普通员工的扩展权限继续由 Permission 判定。
