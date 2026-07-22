# Browser 自动指纹模板审计

> 审计日期：2026-07-22
>
> 适用范围：new-api 托管 Browser Agent 自动创建的固定指纹
>
> Chromium 基线：`150.0.7871.46`

## 结论

自动指纹的配置结构与 Transfigure Browser 39-patch Chromium 的实际解析器兼容，Profile 级稳定 seed、代理强制、WebRTC 限制和地区字段分离方式也是正确的。审计发现的主要问题不是字段缺失，而是审计早期模板草案混用了来源：它使用了 FP-Controlink 2020—2021 历史受控样本中的 `HD Graphics 5500 + 4 cores` 组合，却把模板命名为 Formal Chromium 150。该组合内部并不矛盾，但没有被 2026-07-18 的 Chromium 150 正式验收直接覆盖。

新建 Profile 的模板现已对齐正式验收配置：

- 模板 ID：`formal-150-linux-x86_64-20260718-v1`
- UA / Client Hints：Chromium `150.0.7871.46`、Linux x86_64
- CPU / memory：8 logical cores、8 GiB
- WebGL / WebGPU：Intel UHD Graphics 630、Gen 9、device `0x3e92`
- Screen：1920×1080、available 1920×1040、DPR 1
- 字体 family allowlist：Arial、Courier New、Times New Roman、Noto Sans、Noto Serif、Noto Color Emoji

模板升级只影响之后自动创建的 Fingerprint。已经写入数据库的 Fingerprint 不会被重新生成或静默修改，因此同一 Profile 多次打开仍保持原有身份。

这份结论只证明配置契约、运行时兼容性和 Profile 内稳定性，不承诺规避第三方风控，也不把历史研究样本解释为 2026 年真实用户分布。

## 证据优先级

| 优先级 | 证据 | 用途 |
| --- | --- | --- |
| 1 | `/android/transfigure-browser/releases/chromium-150.0.7871.46/acceptance/2026-07-18-39patch-formal/` 的正式归档 | 确定当前 Chromium 版本、正式模板值、已通过字段和已接受限制 |
| 2 | `/android/chromium-src/src/components/embedder_support/transfigure_profile_config.{h,cc}` | 确定 browser-process 对 UA、Client Hints、Accept-Language、WebRTC 和 Proxy 的实际解析行为 |
| 3 | `/android/chromium-src/src/third_party/blink/renderer/core/frame/transfigure_fingerprint_config.{h,cc}` | 确定 Blink 对 Navigator、硬件、屏幕、Canvas、Audio、WebGL、WebGPU、字体、媒体设备和存储等字段的实际解析行为 |
| 4 | `/android/transfigure-browser/profiles/examples/transfigure-fingerprint-config.example.json` | 提供正式 smoke 使用的可读配置基线 |
| 5 | `transfigure_risk_data.risk_data.active_ref_browser_fingerprints` | 只做历史受控组合的字段一致性复核，不用于生成当前人口画像 |
| 6 | 当前宿主机字体与 Chromium 二进制检查 | 确认部署前提，不代替网页可见 probe |

正式验收所用二进制为：

```text
/android/transfigure-browser/runtime/formal-workspaces/chromium-150.0.7871.46-formal-20260717/out/Acceptance-20260718T033425Z/chrome
Chromium 150.0.7871.46
sha256 3dcefb74b6d429c54d29f2bcb31e0dc172deff3bdc01018dc6850419451762b2
```

Browser Agent 的 runtime key 必须指向这个二进制或经过同一套 39-patch、版本和 probe 门禁验证的等价产物。

## 当前模板契约

### JSON 结构兼容性

两个 Chromium parser 都支持以下两种结构：

- 根级扁平字段，例如 `{"navigator": ..., "hardware": ...}`；
- 嵌套字段，例如 `{"fingerprint": {"navigator": ..., "hardware": ...}}`。

当 `fingerprint` 对象存在时，parser 会优先读取该对象。审计中发现，Agent 早期的地区覆盖逻辑会对根级扁平配置无条件新增 `fingerprint` 对象，导致原本位于根级的 CPU、GPU、屏幕等核心字段被新对象遮蔽。当前实现已改为在原有结构内注入地区字段：扁平输入保持扁平，嵌套输入保持嵌套；控制面保存时也会从对应核心对象移除 Locale、Timezone 和语言字段。两种结构都有回归测试保护。

### Browser process 与 renderer 的配置传递

审计还发现，早期 Agent 只把合并后的 JSON 写入 `.new-api/fingerprint.json`，是否传递 `--transfigure-fingerprint-config` 完全依赖 Fingerprint 自定义启动参数。手工 Fingerprint 的默认启动参数为空；自动生成记录也只有路径参数，没有 Formal launcher 使用的内联 JSON。路径足以让 browser process 读取一部分配置，却不能证明 sandbox 中的 Blink renderer 能读取同一文件，因此核心 Canvas、Audio、WebGL、Navigator 等字段存在未生效风险。

当前 Agent 已与 Formal launcher 的传递契约对齐。它读取同一份 `0600` 配置快照，使用标准 Base64 编码，并强制同时设置：

- `--transfigure-fingerprint-config=<path>`
- `--transfigure-fingerprint-config-json=<base64>`
- `TRANSFIGURE_FINGERPRINT_CONFIG=<path>`
- `TRANSFIGURE_FINGERPRINT_CONFIG_JSON=<base64>`

Fingerprint 自定义参数和环境变量不能覆盖这些值。数据库中已有的两个 Transfigure 启动参数会被 Agent 忽略并替换为受控值，避免升级后使旧 Profile 无法打开；新自动 Fingerprint 的自定义启动参数为空数组。回归测试覆盖空参数、旧参数替换、受控环境变量以及从落盘文件生成内联快照的完整启动链路。

### 固定且跨启动复用

以下字段在 Fingerprint 创建时确定，之后直接从数据库复用：

| 类别 | 固定字段 |
| --- | --- |
| 浏览器身份 | UA、Chromium full version、Client Hints brands/platform/architecture/bitness |
| Navigator | platform、vendor、product、appVersion、touch points、cookie、online、PDF viewer、webdriver |
| 硬件与屏幕 | concurrency、device memory、screen/available area、color depth、DPR |
| 图形与音频 | WebGL vendor/renderer/extensions、WebGPU adapter metadata、Canvas/WebGL/Audio seed |
| 设备与存储 | media device IDs、battery、storage estimate、network information |

Canvas、WebGL、Audio 和 media device ID 在创建时使用 Profile 的初始 `data_key` 分域派生，随后作为 Fingerprint payload 持久化。Chromium 最终把字符串 noise seed 归一为 32-bit seed；同一 Profile 的值稳定，不同 Profile 使用不同输入。“重置 Profile 数据”只轮换浏览器存储目录 key，不会重算或改变已经保存的 Fingerprint。

### 每次启动动态注入

下列字段不属于固定指纹：

- 出口 IP
- 国家或地区
- Locale
- `Accept-Language` / `navigator.languages`
- IANA Timezone

Browser Agent 每次启动都必须先通过同一托管代理获取 GeoIP，再把语言和时区覆盖到临时 `.new-api/fingerprint.json`。浏览器预检会读取实际 `navigator.language`、`navigator.languages` 和 Intl timezone；不一致时终止流程。核心 Fingerprint 数据库记录不保存这些地区字段。

IP 不参与 CPU、GPU、Canvas、Audio、屏幕或 media device seed 的生成。更换代理只改变本次地区覆盖和网络出口，不应改变固定设备身份。

### 代理与泄漏面

Agent 强制追加以下运行约束，Fingerprint 的自定义参数不能覆盖：

- `--proxy-server`
- `--disable-quic`
- `--force-webrtc-ip-handling-policy=disable_non_proxied_udp`
- 只允许 OAuth 本地预检和回调使用 loopback bypass
- `HTTP_PROXY`、`HTTPS_PROXY`、`ALL_PROXY` 与 Chromium 使用同一托管代理

39-patch 正式验收覆盖 candidate event/local SDP、`RTCStatsReport`、`RTCIceTransport` 和 detached iframe 生命周期；保护模式下非 relay 地址计数为 0。

## 与 FP-Controlink 历史数据的复核

`transfigure-risk-data` 当前活跃发布包含 FP-Controlink 的 1,148 个受控 Profile。数据库快照和项目文档都明确说明它是 2020—2021 历史实验数据，不代表 2026 年市场份额或真实人口分布。

2026-07-22 对活跃视图进行只读聚合得到：

| 字段 | 历史受控结果 |
| --- | ---: |
| Ubuntu 20.04 | 1,062 / 1,148 |
| 1920×1080 | 1,008 / 1,148 |
| hardware concurrency = 4 | 950 / 1,148 |
| `navigator.platform = Linux x86_64` | 1,058 / 1,148 |
| WebDriver = true / false / unknown | 845 / 288 / 15 |

统计口径使用数据库中的小写值 `os_name = 'ubuntu'` 和 `os_version = '20.04'`。历史 Chrome + Ubuntu 记录中，`1920×1080 + 4 cores + Google Inc. + ANGLE Intel HD Graphics 5500` 是真实出现过的受控组合。这说明审计早期模板草案的 CPU/GPU/screen 字段并非互相冲突，但不能证明它适合 Chromium 150，也不能证明它是当前常见设备。尤其是大多数历史记录带有 `webdriver=true`，这是实验采集环境特征，不能作为当前模板的生成分布。

因此风险数据在本项目中的用途限定为：

- 检查字段是否存在明显跨平台矛盾；
- 复核 parser 对历史字段名和值域的兼容性；
- 设计防御性一致性规则和测试输入。

禁止把这些记录用于抽样生成“更像真人”的指纹、推断当前地区人口画像或优化第三方反滥用规避。

## 已确认正确的设计

1. UA、Client Hints、`navigator.platform` 和 `navigator.appVersion` 使用同一个 Chromium 150 / Linux x86_64 基线。
2. model 中的 viewport 与 payload 中的 screen 同为 1920×1080，Agent 同时设置窗口尺寸，避免只改 JS screen。
3. 自动生成只在创建时执行一次；Fingerprint 和 Profile 在一个数据库事务内创建。
4. 扰动 seed 和 media IDs 在创建时由初始 `data_key` 分域派生并持久化，后续重置浏览器存储不会改变 Fingerprint。
5. 固定模板不绑定地区；语言和时区由代理 GeoIP 动态覆盖。
6. WebRTC、QUIC、系统代理环境变量和 Chromium proxy 参数由 Agent 强制管理，模板不能绕过。
7. `navigator.webdriver=false` 与正式验收配置一致；没有复制历史受控数据中偏高的 automation 信号。
8. Agent 强制把同一配置快照以 path 和 inline JSON 同时传给 browser process 与 renderer，旧记录不能覆盖受控值。

## 仍然存在的边界

### 1. Runtime 只按 key 广告，尚无版本证明

控制面当前只知道 Agent 上报的 runtime key，不知道该 key 对应二进制的版本和 SHA-256。自动模板因此依赖运维保证 runtime 映射正确。如果同一 key 被改指向普通 Chrome、其他 Chromium 版本或不同 patch stack，服务端仍会生成 Chromium 150 模板。

建议后续让 Agent 心跳上报每个 runtime 的 `version`、`sha256` 和 capability set，控制面只在匹配模板声明时开放自动生成。

### 2. new-api 尚未运行完整 fingerprint probe

Transfigure Browser 正式门禁对同一 Profile 连续 3 次验证了 73 个稳定字段和 64 个 required-match 字段。new-api Browser Agent 当前只执行代理出口、语言和时区预检，没有采集 UA/CH、screen、hardware、Canvas、WebGL、Audio、fonts、media、storage 等完整报告。

因此“配置已生成”不能等价为“所有网页可见字段已验证”。建议把 fingerprint probe 作为 Agent 的可选验收任务，保存结构化结果和模板版本，不把原始敏感网络地址写入日志。

### 3. 字体 family allowlist 不会安装字体

当前 Chromium patch 只按 family 过滤可见字体；new-api Agent 没有接入 Transfigure Browser post-formal 的 FontBundle materializer。正式验收环境中 6 个声明 family 只有 Noto Color Emoji 被 Local Font Access 实际枚举，其余缺失被正式报告记录为允许的已知限制。

所以模板中的 fonts 表示“最多允许暴露这些 family”，不是“宿主机一定存在这些字体文件”。需要精确字体文件一致性时，必须接入受控 FontBundle，不能只增加 JSON 名称。

### 4. Media device 目前主要控制身份元数据

固定模板能稳定覆盖 `kind/deviceId/groupId/label`，但 2026-07-18 正式 release 没有生成虚拟音视频内容流。new-api Agent 也未接入 post-formal `virtual_media`。依赖真实 `getUserMedia()` 内容的业务仍可能受宿主机硬件和权限影响。

### 5. WebGPU 与 Network Information 有运行环境限制

正式验收允许以下字段缺失：

- Headless 场景中的 WebGPU surface 和 driver；
- `navigator.connection.type`；
- `navigator.connection.downlinkMax`。

模板配置这些值不代表目标环境一定暴露对应 API。常规 OAuth 使用 headed Chromium，但验收和监控仍应区分 `match`、`missing` 与 `environment_limit`。

### 6. 自动模板是固定设备族，不是人口采样器

所有自动 Profile 共享同一个 Chromium/OS/CPU/GPU/screen 基线，只有 profile ID、Canvas/WebGL/Audio seed 和 media IDs 按 Profile 区分。这是为了可解释、可重复和可验收，不应扩展为从历史数据库随机拼接字段。

## 发布与运维规则

1. 模板内容变化时必须修改模板 ID，并新增精确回归断言。
2. 不批量重写已有 Fingerprint；需要新模板时创建新 Profile 或显式迁移。
3. runtime 只能指向已核对版本的 Formal Chromium，不能回退到系统 Chrome。
4. 核心 Fingerprint 禁止保存 locale、timezone、language、country 或出口 IP。
5. Profile 启动必须使用已绑定的托管代理，不允许直连回退；Profile 本身可以不绑定渠道并独立打开。
6. Agent 必须强制传递 path 与 inline JSON 两种指纹配置，并拒绝 Fingerprint 自定义参数或环境覆盖。
7. 生产上线前至少核对 Chromium `--version` 和 SHA-256；版本变化后重新执行 Transfigure fingerprint probe。
8. 风险数据只用于防御性一致性和兼容性分析，不进入自动模板随机生成逻辑。

## 最低验证清单

代码级门禁：

```bash
go test ./service ./cmd/browser-agent -count=1
go test ./... -count=1
```

运行时身份：

```bash
/android/transfigure-browser/runtime/formal-workspaces/chromium-150.0.7871.46-formal-20260717/out/Acceptance-20260718T033425Z/chrome --version
sha256sum /android/transfigure-browser/runtime/formal-workspaces/chromium-150.0.7871.46-formal-20260717/out/Acceptance-20260718T033425Z/chrome
```

网页可见验收应使用 `/android/transfigure-browser/tools/fingerprint-probe/`，至少覆盖正式报告中的 64 个 required-match 字段、同 Profile 重启稳定性、不同 Profile seed 差异、WebRTC 非 relay 地址为 0，以及地区覆盖与代理 GeoIP 一致。
