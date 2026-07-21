# 托管浏览器 OAuth 与 Browser Agent

new-api 作为控制面统一管理 Browser Agent、公共代理、浏览器指纹和 Profile。Browser Agent 只负责在 Chromium 所在主机上启动指定运行时、维护浏览器数据目录，并执行一次 OAuth 登录。

浏览器登录、OAuth 授权码交换、Token 刷新、Codex 用量查询和渠道中继请求都会使用同一条托管代理配置。该链路采用严格失败策略：代理、GeoIP 服务或浏览器指纹验证任一不可用时，OAuth 都会终止，不会降级为直连。

Browser Agent 会在浏览器启动前通过托管代理查询出口 GeoIP，并要求指纹中的 Locale 和 Timezone 分别与出口国家的语言列表及出口时区匹配。浏览器启动后，本地预检页还会读取实际的 `navigator.language`、`navigator.languages` 和 `Intl.DateTimeFormat().resolvedOptions().timeZone`；只有实际值也匹配时才会进入 OpenAI 授权页。授权码交换前会再次通过同一代理复验 GeoIP。

上述检查能够保证受管流程不会在校验失败时继续，也能阻止启动参数、环境变量和渠道配置绕过代理。但第三方代理是否为每个连接固定同一出口，最终由代理服务决定。若代理会随机分配不同国家或时区的出口，流程会在检测到不匹配时失败；若业务要求出口 IP 本身始终相同，必须使用带粘性会话的代理账号或固定出口代理。

## 推荐部署

- new-api：部署在服务器或现有控制面环境中。
- Browser Agent：部署在自编译 Chromium 所在的机器上。本地桌面和远程桌面主机都支持。
- 公共代理：必须能同时被 Browser Agent 主机和 new-api 主机访问。
- 远程 Browser Agent 连接 new-api 时必须使用 HTTPS；仅连接 `localhost` 时允许 HTTP。

Browser Agent 不需要与 new-api 同机。对于自编译指纹 Chromium，推荐让 Browser Agent 与 Chromium 同机，而不是把 Chromium 放进 new-api 服务进程或容器中。

## 安全前置条件

托管代理 URL 会使用 AES-GCM 加密后写入数据库。启用此功能前，必须为 new-api 配置稳定且妥善保管的 `CRYPTO_SECRET` 或 `SESSION_SECRET`。如果两个变量都没有配置，new-api 会拒绝保存托管代理，避免重启后无法解密。

```bash
export CRYPTO_SECRET='replace-with-a-long-random-secret'
```

Browser Agent Token 只在创建或轮换时显示一次，数据库仅保存不可逆摘要。建议通过环境变量传入 Token，避免把 Token 放到进程命令行或脚本参数中。

## 构建 Browser Agent

在仓库根目录执行：

```bash
go build -o browser-agent ./cmd/browser-agent
```

## 启动 Browser Agent

先由 root 用户在 new-api 的“浏览器管理”页面创建 Agent，并复制一次性 Token。然后在 Chromium 主机执行：

```bash
export NEW_API_BROWSER_AGENT_SERVER='https://new-api.example.com'
export NEW_API_BROWSER_AGENT_TOKEN='nba_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'
export NEW_API_BROWSER_PROFILE_ROOT='/var/lib/new-api-browser/profiles'
export NEW_API_BROWSER_RUNTIMES='fingerprint=/opt/fingerprint-chromium/chrome'
# 可选；默认值为 https://ipapi.co/json/
export NEW_API_BROWSER_GEOIP_URL='https://ipapi.co/json/'

./browser-agent
```

也可以重复使用 `--runtime key=/absolute/path/to/chrome` 注册多个 Chromium 运行时，并可用 `--geoip-url` 覆盖 GeoIP 地址。Profile 中只保存运行时 key，可执行文件的实际路径始终只保留在 Agent 主机上。

GeoIP 地址必须使用 HTTPS；只有本机测试地址允许 HTTP。兼容端点必须返回以下字段，缺少、无效或非 2xx 响应都会使流程失败：

```json
{
  "ip": "203.0.113.10",
  "country_code": "US",
  "timezone": "America/Los_Angeles",
  "languages": "en-US,es-US"
}
```

GeoIP 请求只通过 Agent 创建的本地转发代理发出，没有直连回退。控制面只允许上报 `strict_proxy_geo_v1` 能力的新版 Agent 启动新的托管 OAuth 流程；升级 Agent 后需要重新启动，使下一次心跳上报该能力。

默认情况下，Agent 每 15 秒发送一次心跳，每 2 秒领取一次待处理 OAuth 流程。OAuth 回调使用 Agent 主机的 `127.0.0.1:1455`，该端口必须可用。一个 Agent 同一时间只执行一个 OAuth 流程。

## 控制面配置顺序

1. 创建 Browser Agent，并在 Chromium 主机启动它，确认状态为在线且已上报运行时。
2. 创建托管代理。支持 `http`、`https`、`socks5` 和 `socks5h`，可在 URL 中携带用户名和密码。
3. 创建浏览器指纹，配置 User Agent、Locale、Timezone、视口、指纹 JSON、启动参数和环境变量。Locale 必须是有效的 BCP 47 标签（例如 `en-US`），Timezone 必须是有效的 IANA 时区（例如 `America/Los_Angeles`），并且两者必须与所选代理的出口 GeoIP 一致。
4. 创建 Profile，将 Agent、运行时、托管代理和浏览器指纹绑定在一起。
5. 新建或编辑 Codex 渠道，选择 Profile 后点击“打开浏览器登录”。

OAuth Token 不会返回前端页面。登录成功后，页面只展示 email、account ID、plan 和凭证过期时间。新建渠道时，已完成的 OAuth 流程需要在页面显示的截止时间前保存；编辑已有渠道时，凭证会在 OAuth 完成后直接更新到该渠道。

## 自编译 Chromium 对接

指纹配置中的启动参数是 JSON 字符串数组，例如：

```json
[
  "--fingerprint-config={fingerprint_file}",
  "--fingerprint-timezone={timezone}",
  "--fingerprint-locale={locale}"
]
```

可使用以下模板变量：

- `{profile_dir}`
- `{fingerprint_file}`
- `{proxy_server}`
- `{authorize_url}`
- `{oauth_preflight_url}`
- `{locale}`
- `{timezone}`
- `{user_agent}`
- `{viewport_width}`
- `{viewport_height}`

`{authorize_url}` 和 `{oauth_preflight_url}` 都指向 Agent 的本地浏览器预检页。真正的 OpenAI 授权地址只会在实际语言和时区验证成功后由预检页取得。

指纹 JSON 会以权限 `0600` 写入 Profile 目录下的 `.new-api/fingerprint.json`。Browser Agent 会强制设置 Profile、代理、语言、时区、视口、WebRTC 和 QUIC 相关参数，并强制应用指纹中已配置的 User Agent；指纹模板不能覆盖这些参数、代理绕过规则或远程调试参数。外部地址全部走托管代理，仅 OAuth 所需的本机预检和回调地址允许直连回环接口。

环境变量配置是 JSON 字符串映射。出于安全考虑，Agent 会拒绝 `PATH`、`HOME`、`LD_PRELOAD`、`DYLD_*`、`NODE_OPTIONS` 等可改变程序加载行为的变量，也不允许覆盖 `TZ`、`LANG`、`LANGUAGE`、`LC_*`、`HTTP_PROXY`、`HTTPS_PROXY`、`ALL_PROXY` 和 `NO_PROXY`。这些值由 Agent 按已验证的指纹和本地转发代理统一设置。

## Profile 与运维

- 持久化 Profile 会在 Agent 主机保留 Cookie 和浏览器状态。
- 临时 Profile 会在 OAuth 流程结束后删除。
- “重置 Profile 数据”会轮换目录 key；旧目录不会由控制面远程删除，需要在 Agent 主机确认后手工清理。
- 轮换 Agent Token 会立即使旧 Token 失效。
- 更新托管代理后，后续浏览器登录、Token 请求和渠道流量会使用新配置。
- 仍被 Profile 或渠道引用的托管代理不能删除。
