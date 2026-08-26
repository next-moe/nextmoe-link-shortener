# nextmoe-link-shortener

NextMoe 生态的共享短链接服务：Go API + Nuxt 控制台。生态站点（kungal / moyu /
letmoe …）通过 S2S API 生成短链；管理员通过生态 OIDC 登录控制台管理短链。

## 形态

标准生态站形（对齐 `kun-softmoe`）：

| 目录 | 栈 | 职责 |
|---|---|---|
| `apps/api` | Go 1.26 · Fiber v3 · Huma v2 · GORM | OIDC RP + BFF、短链 CRUD、`/s/{alias}` 跳转、S2S API、访问统计 |
| `apps/web` | Nuxt 4 · `@kungal/ui-nuxt` · Tailwind v4 | 管理控制台（登录 / 短链管理 / 统计 / API key 管理） |

- 数据库：Postgres `kun_shortlink`（GORM AutoMigrate，随 API 启动）
- 会话：Redis（`shortlink_session` httpOnly cookie，token 永不到浏览器）
- 身份：NextMoe IdP（OIDC discovery + PKCE + JWKS 验签），控制台仅限
  `SHORTLINK_ADMIN_ROLES`（默认 `admin`）角色
- S2S：Bearer API key（`slk_` 前缀，SHA-256 存储，创建时仅显示一次）

## 本地开发

```bash
pnpm install
pnpm dev                      # 缺 .env 时从 .env.example 复制；拉起 Postgres :7846
                              # + Redis :7847；然后 api :7845 + web :7844
```

`pnpm dev` 会自己起 backing services。若只想起数据库：

```bash
pnpm dev:services             # docker compose --env-file .env -f docker/compose.dev.yml up -d --wait
```

本地登录走 NextMoe IdP（infra oauth `:9277`）。先确保 infra 的 `pnpm dev` 在跑，
再注册本站的 dev OAuth client（幂等）：

```bash
pnpm oidc:register            # 写入 kun_galgame_infra.oauth_clients
```

`.env.example` 已带公开的本地凭证（`shortlink-dev` / `dev-secret-shortlink-dev`）。
无 IdP 时把 `SHORTLINK_OIDC_ISSUER` / `CLIENT_ID` / `CLIENT_SECRET` 留空，API
接受 `Authorization: Dev <user_id> [roles]` 后门（如 `Dev 1 admin`）。

## S2S API（生态站点接入）

在控制台「API Keys」页为站点创建一个 key，然后：

```
POST /s2s/links
Authorization: Bearer slk_...
{ "destination_url": "https://...", "alias": "", "description": "kungal share" }
→ { "id": 1, "alias": "aB3kZ9", "short_url": "https://<host>/s/aB3kZ9", "reused": false }
```

- `alias` 留空 = 随机 6 位（无易混淆字符）；自定义需 4-32 位 `[A-Za-z0-9_-]`
- 相同 `destination_url` 的活跃随机短链默认复用（`reuse=false` 关闭）
- `GET /s2s/links/{alias}` 查询单条短链

批量拉取按日统计（结算粒度）：

```
POST /s2s/stats/daily
Authorization: Bearer slk_...
{ "aliases": ["aB3kZ9"], "from": "2026-09-01", "to": "2026-09-30" }
→ { "stats": { "aB3kZ9": [ { "date": "2026-09-01", "total": 12, "uniques": 9 } ] } }
```

- `from` / `to` 是 **JST（Asia/Tokyo）日历日**，闭区间，跨度 ≤ 92 天；无流量的日不出现
- 去重口径：一个访问指纹 `sha256(ip + "\n" + user_agent)` 在同一短链的同一 JST 日只计一次 unique
- `aliases` 一批 1-500 个；越界或超限 → 422
- 未知 alias 返回空数组而不是 404 —— 单个未知不得让整批失败

完整契约见 `apps/api/openapi/openapi.yaml`（code-first，`pnpm gen` 再生）。

## CI

| workflow | 触发 | 内容 |
|---|---|---|
| `.github/workflows/ci.yml` | push `main` / PR | `api`（go build·vet·test）、`web`（eslint）、`spec`（openapi + TS 类型零漂移）——等价于本地 `pnpm verify` |
| `.github/workflows/build.yml` | push `main` / 手动 | 按路径过滤构建镜像 → 推 GHCR → 打 Dokploy redeploy webhook |

`build.yml` 只构建输入变了的那个镜像（`apps/api/**` → api，`apps/web/**` + 根清单 →
web），只改文档的 push 一个镜像都不构建。Actions → Run workflow 可以指定
`scope=api|web` 强制单独构建。

- `ghcr.io/next-moe/shortlink-api` — distroless，约 40 MB，`/s` 跳转 + API
- `ghcr.io/next-moe/shortlink-web` — Nitro node-server，约 390 MB

> web 镜像的代理目标（`/api/**`、`/s/**` → `http://shortlink-api:7845`）是
> **构建期烘进去的**（Nitro route rules 在 `nuxt build` 时定死），所以它不是面板里的
> 运行时变量；改动需要改 `build.yml` 的 `API_PROXY_TARGET` 并重建。对应
> `docker-compose.prod.yml` 里 api 的网络别名 `shortlink-api`，这个别名必须保持稳定。
> API 镜像相反：所有 `SHORTLINK_*` 都是启动时读环境变量，一个镜像跑任何环境。

## 部署（Dokploy）

### 首次部署前

1. **宿主机 Postgres 建库建角色**（生态惯例：一个 Postgres，每个域一个库）：
   ```sql
   CREATE ROLE shortlink LOGIN PASSWORD '...';
   CREATE DATABASE kun_shortlink OWNER shortlink;
   ```
   表结构由 API 启动时 GORM AutoMigrate 建，无需单独的 migrate 步骤。
2. **在 infra 注册生产 OAuth client**（confidential，带 secret），
   `redirect_uri` 填 `https://<域名>/auth/callback`。
3. **GHCR 包可见性**：首次推送生成的 package 默认私有。要么在 GitHub →
   Packages → Package settings 改成 public，要么在 Dokploy 里配一个 registry
   凭据（用户名 = GitHub 账号，密码 = 带 `read:packages` 的 PAT）。
4. Dokploy 新建 Compose 应用，compose 文件用仓库根的 **`docker-compose.prod.yml`**：
   web 绑域名（`expose: 3000`，Traefik 内部路由），api 和 redis 不对外暴露。
   Postgres 不在这个文件里 —— DSN 是面板变量，指宿主机实例或另一个 Dokploy 应用
   都行；表由 API 启动时 AutoMigrate 建，没有单独的 migrate 服务。

### Dokploy 面板要填的环境变量

| 变量 | 必填 | 值 |
|---|---|---|
| `SHORTLINK_DB_DSN` | ✅ | `postgres://shortlink:<密码>@host.docker.internal:5432/kun_shortlink?sslmode=disable` |
| `SHORTLINK_PUBLIC_BASE_URL` | ✅ | 站点对外 origin，如 `https://s.kungal.com`（短链 `<base>/s/<alias>` 由它拼） |
| `SHORTLINK_OIDC_ISSUER` | ✅ | NextMoe IdP 根，如 `https://oauth.kungal.com`（端点走 discovery，不要硬编码） |
| `SHORTLINK_OIDC_CLIENT_ID` | ✅ | 上面注册的 client id |
| `SHORTLINK_OIDC_CLIENT_SECRET` | ✅ | 对应 secret |
| `SHORTLINK_OIDC_REDIRECT_URI` | ✅ | `https://<域名>/auth/callback`，必须与 IdP 侧登记的完全一致 |
| `SHORTLINK_ADMIN_ROLES` | ⭕ | 默认 `admin`；逗号分隔，与 JWT roles 取交集才放进控制台 |

`SHORTLINK_MODE=prod`、`SHORTLINK_HOST`、`SHORTLINK_PORT`、
`SHORTLINK_REDIS_ADDR=redis:6379` 已经写死在 compose 里，不用填。**prod 模式下
OIDC 四件套和 Redis 缺一个进程就拒绝启动**（`internal/config`），不会带着半截认证跑起来。

### GitHub secrets

| secret | 必填 | 用途 |
|---|---|---|
| `DOKPLOY_WEBHOOK_SHORTLINK` | ⭕ | Dokploy 应用的 redeploy webhook URL。不配也不会让 workflow 失败：镜像照推 GHCR，只是不自动触发重新部署 |

GHCR 推送用内置的 `GITHUB_TOKEN`（`packages: write`），无需额外 secret。
面板里把 `:latest` 换成 `:<sha>` 可以精确回滚（infra 惯例）。

## License

AGPL-3.0（见 `LICENSE`）。
