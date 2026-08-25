# kungal-link-shortener

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

无本地 IdP 时（dev 模式），API 接受 `Authorization: Dev <user_id> [roles]`
后门（如 `Dev 1 admin`），控制台登录不可用但 API 可直接调试。需要完整 OIDC
登录时，把 `SHORTLINK_OIDC_*` 填进 `.env` 后重启 API。

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

完整契约见 `apps/api/openapi/openapi.yaml`（code-first，`pnpm gen` 再生）。

## 部署

CI（`.github/workflows/build.yml`）按路径过滤构建镜像推 GHCR：

- `ghcr.io/kungal/shortlink-api`（distroless，`/s` 跳转 + API）
- `ghcr.io/kungal/shortlink-web`（Nitro node-server）

Dokploy 参照 `docker/compose.dokploy.yml`：web 容器对外，`/api/**` 与 `/s/**`
由 Nitro 代理到 api 容器；Postgres 用宿主机实例，Redis 随 compose。
`SHORTLINK_OIDC_*` 等 env 在 Dokploy 面板注入。

## License

AGPL-3.0（见 `LICENSE`）。
