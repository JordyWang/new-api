# Browser 自动指纹模板审计

> 审计日期：2026-07-22
>
> 适用范围：new-api 托管 Browser Agent 自动创建的固定指纹
>
> Chromium 基线：`150.0.7871.46`

## 结论

`v1` 模板能够证明 Transfigure Browser 的字段覆盖补丁生效，但不足以作为生产设备模板。根因是它直接复用了 Formal release smoke 的“全字段覆盖”验收夹具；该夹具的目标是一次覆盖尽可能多的补丁，不是模拟常见浏览器。

自动创建的 Fingerprint 已升级为：

- 模板 ID：`formal-150-desktop-20260722-v4-{windows-x86_64|macos-x86_64}`
- 固定核心：Chromium 150 的 Windows 11 或 macOS reduced UA、Client Hints、Navigator 平台字段，以及 1920×1080 屏幕、8 cores / 8 GiB、WebGL seed、WebRTC 策略；
- Chromium 原生表面：Canvas、Audio、GPU 身份与能力、字体、Battery、Storage、Network Information、媒体设备、在线状态、Cookie/DNT 和 PDF 插件状态；
- 每次启动覆盖：代理出口、Locale、语言和 IANA Timezone。

平台由创建时的随机 `data_key` 通过 HMAC 稳定选择：80% Windows 11、20% macOS，自动生成路径不产生 Linux 身份。选择结果和其他固定字段一起持久化，重新打开或重置 Profile 数据都不会换平台。

这个边界保证 new-api 只固定能够完整控制且需要跨启动稳定的字段。动态字段、权限敏感字段以及补丁只能局部覆盖的硬件字段由浏览器返回真实值，避免出现“字段匹配验收夹具，但字段之间互相矛盾”的情况。

模板升级只影响之后自动创建的 Fingerprint。已经写入数据库的 Fingerprint 不会被重新生成或静默修改，因此既有 Profile 多次打开仍保持原身份。

这份结论只覆盖协议兼容、字段内部一致性、运行时可验证性和 Profile 内稳定性，不承诺规避第三方风控，也不使用历史样本拼装所谓真人画像。

## 证据范围与优先级

| 优先级 | 证据 | 用途 |
| --- | --- | --- |
| 1 | 指定 Formal Chromium 二进制的无配置与配置后 headed runtime probe | 对比网页真实可见值，验证实际启动参数影响 |
| 2 | `/android/chromium-src/src/components/embedder_support/` 与 Blink 对应源码 | 确定 UA/CH、Battery、GPU、字体和媒体字段的真实控制边界 |
| 3 | `/android/transfigure-browser/releases/chromium-150.0.7871.46/acceptance/2026-07-18-39patch-formal/` | 确定补丁版本、正式门禁、稳定字段和已知环境限制 |
| 4 | W3C Battery Status、Media Capture and Streams 与 UA Client Hints 规范 | 检查跨字段语义、权限和 origin 隔离要求 |
| 5 | 当前 Agent 主机的字体、GPU、CPU、屏幕和浏览器默认行为 | 检查部署环境中的真实一致性，不外推为人口分布 |
| 6 | `transfigure-risk-data` 历史受控样本 | 只做历史字段兼容和明显跨平台矛盾复核 |

审计没有把 risk-data 当作唯一或最高优先级来源。Formal 验收配置本身也只证明“补丁能按配置返回值”，不能证明该组合常见。

正式二进制：

```text
/android/transfigure-browser/runtime/formal-workspaces/chromium-150.0.7871.46-formal-20260717/out/Acceptance-20260718T033425Z/chrome
Chromium 150.0.7871.46
sha256 3dcefb74b6d429c54d29f2bcb31e0dc172deff3bdc01018dc6850419451762b2
```

## 运行时对照结果

审计在同一个 headed X11 环境中分别启动了未配置的 Formal Chromium、`v1` Formal 配置和当前配置。探针使用 `/android/transfigure-browser/tools/fingerprint-probe/`，页面和 Header 捕获都只访问本机 loopback。

| 表面 | 未配置 Chromium 150 | `v1` / Formal 夹具 | `v4` 决策 |
| --- | --- | --- | --- |
| Legacy UA | `Chrome/150.0.0.0` | `Chrome/150.0.7871.46` | 使用 Chromium 默认的 reduced UA |
| 低熵 UA-CH | `Not;A=Brand 8` + `Chromium 150` | 只有 `Chromium 150` | 与该二进制默认 brand list 一致 |
| 高熵 UA-CH | x86/64，full `150.0.7871.46` | 配置本身可匹配 | 保留完整版本并移除冲突的 CLI UA 参数 |
| Battery | `true / 0 / Infinity / 1` | `true / 0 / 36000 / 1` | 原生；不固定动态状态 |
| Storage | 新 Profile 实测 10 GiB / 0 | 固定 20 GiB / 100 MiB | 原生；随 origin 数据真实变化 |
| Network | 实测值会随连接变化，例如 1.3 Mbps / 100 ms | 固定 10 Mbps / 50 ms | 原生；不伪装代理实际链路 |
| Media devices | 无权限时只有空 label/ID 的输出设备 | 三个带固定 label/ID 的虚拟设备 | 原生权限和 per-origin ID 规则 |
| Local Font Access | 当前主机实测 142 个 family | allowlist 后只剩 `Noto Color Emoji` | 原生；未接 FontBundle 前不声明不存在的字体 |
| WebGL identity | 当前主机 NVIDIA RTX 3070 Ti | 声明 Intel UHD 630 | 原生 GPU identity/extensions/limits，只保留 readback seed |
| WebGL limits | 原生 `MAX_TEXTURE_SIZE=32768` 等 | 声明 Intel 后仍是同一组宿主 limits | 原生，避免局部伪装 |
| WebGPU | 当前运行环境不可用 | 配置 Intel metadata 仍不可见 | 不写无从验证的 metadata |
| WebRTC non-relay | 未保护时可见 host candidate | Formal 保护为 0 | 五个探针读取面均为 0 |
| Canvas | 原生输出 | seed 修改 readback | 原生；BrowserScan 会把 seed 输出判为 Canvas Tampering |
| Audio | 原生输出 | seed 修改 OfflineAudioContext | 原生；BrowserScan 会把 seed 输出判为 Audio exception |
| 平台身份 | Linux UA / `Linux x86_64` / CH `Linux` | 配置字段可覆盖 | Windows：`Win32` / CH `Windows 19.0.0`；macOS：`MacIntel` / CH `macOS 15.7.6` |

`1920×1080 / DPR 1 / 8 cores / 8 GiB` 是固定、可解释的桌面基线，不是统计抽样结果。屏幕 payload 与 Agent 的 `--window-size` 同步。Windows 和 macOS 两套 UA、Navigator 与 Client Hints 分别保持内部一致，并都使用 x86/64 桌面架构。

在指定 Formal Chromium、同一 Clash 日本出口和同一 headed X11 环境中进行的平台 A/B 验证：Windows 配置在 HTTP Header、DevTools、`navigator.userAgent`、`navigator.platform=Win32`、CH `Windows/19.0.0/x86/64` 上全部匹配，BrowserScan 显示 `Windows 11`、真实性 `100%`；macOS 配置对应字段全部匹配，BrowserScan 显示 `Mac OS 15.7.6`、真实性 `100%`。两者均没有 Audio exception 或 Canvas Tampering。该结果是指定版本和检测站在审计时点的观测，不是长期绕过第三方检测的承诺。

## 已修复的实际启动参数冲突

审计按 new-api 的真实参数组合启动 `v2` 时发现：如果 Agent 同时传入 Transfigure 配置和 Chromium `--user-agent`，低熵 UA-CH 仍存在，但高熵字段会变为空字符串，`fullVersionList` 也会变为空数组。

根因可在 Chromium 150 的 `components/embedder_support/user_agent_utils.cc` 中直接确认：`GetUserAgentMetadata()` 检测到命令行自定义 UA 后会在填充高熵字段之前提前返回。Transfigure 的高熵覆盖位于这条返回路径之后，因此无法生效。

Agent 现在把数据库中的 `Fingerprint.UserAgent` 写入同一份受控 Fingerprint JSON，并且不再传 `--user-agent`。UA 和 Client Hints 由 Transfigure browser-process 配置一起应用。运行时复测结果为：

```text
User-Agent: Chrome/150.0.0.0
Sec-CH-UA: "Not;A=Brand";v="8", "Chromium";v="150"
architecture: x86
bitness: 64
uaFullVersion: 150.0.7871.46
fullVersionList: Not;A=Brand 8.0.0.0, Chromium 150.0.7871.46
```

自定义 Fingerprint 仍可只填写数据库的 User Agent 字段；Agent 会在落盘快照中注入该值。服务端自定义启动参数继续禁止 `--user-agent`，避免重新引入两个 UA 来源。

## 为什么不再固定部分字段

### Battery

W3C Battery Status 规定：电池正在充电、无法报告剩余放电时间或没有电池时，`dischargingTime` 必须为正 Infinity。Chromium Linux 的默认 `BatteryStatus` 和无电池单元测试也是 `charging=true`、`chargingTime=0`、`dischargingTime=Infinity`、`level=1`。

`v1` 的 `charging=true` 与 `dischargingTime=36000` 语义冲突。当前 JSON parser 又不能表达 Infinity，因此 `v2` 不覆盖 Battery。

### Media devices

Media Capture and Streams 规范要求可识别用户的 `deviceId` 对其他 origin 不可猜测，并随 origin 存储清理而轮换；`groupId` 需要按 document 生成。普通 Chromium 还会根据权限控制 label 和可见设备信息。

Formal 补丁会直接返回配置中的固定 ID、group 和 label，不执行这些 origin/权限语义。`v2` 删除固定设备列表，让持久化 Chromium Profile 自己管理设备标识和权限。

### GPU 与字体

当前补丁可覆盖 WebGL vendor/renderer、extension allowlist 和 readback noise，但不会覆盖全部 WebGL limits、shader precision、驱动行为或性能。实测将宿主 NVIDIA 标成 Intel 后，`MAX_TEXTURE_SIZE`、viewport limits 等仍与未配置宿主完全相同。`v4` 因此只保留 WebGL noise seed，不伪造 GPU 型号。

字体 allowlist 只会过滤现有字体，不会安装字体。Formal 配置声明的 Arial、Courier New、Times New Roman、Noto Sans 和 Noto Serif 在正式报告的 Local Font Access 中都不存在，只枚举到 Noto Color Emoji。接入带 manifest 和文件哈希的 FontBundle 前，`v2` 使用 Agent 主机原生字体。

### Storage、Network 与 Navigator 动态状态

Storage usage 是 origin 数据量，Network Information 是当前连接估计，`navigator.onLine`、Cookie、DNT 和 PDF 状态也可能随用户设置或环境变化。把这些值写成所有 Profile 相同的常量不会提高身份一致性，反而会制造与真实状态冲突，所以 `v2` 不覆盖它们。

## `v4` 固定与动态契约

### 创建时固定并持久化

| 类别 | 字段 |
| --- | --- |
| 浏览器身份 | 稳定选择的 Windows/macOS reduced UA、Chromium full version、GREASE + Chromium brands、platform/platformVersion/architecture/bitness |
| Navigator | appName/appCodeName/appVersion、platform、vendor/product、touch points |
| 屏幕与硬件 | 1920×1080、available 1920×1040、DPR 1、8 cores、8 GiB |
| 差异 seed | WebGL readback |
| 安全策略 | `webdriver=false`、`disable_non_proxied_udp` |

Seed 由创建时的随机 `data_key` 通过分域 HMAC-SHA256 派生，payload 随 Fingerprint 持久化。“重置 Profile 数据”只轮换浏览器目录 key，不会重算 Fingerprint。

### 由持久化 Chromium Profile 和绑定 Agent 提供

- Canvas、OfflineAudioContext；
- GPU vendor/renderer/extensions/limits、WebGPU；
- 字体及 Local Font Access；
- Battery、Storage、Network Information；
- 媒体设备、permission、per-origin ID；
- online、Cookie、DNT、PDF viewer 等运行时状态。

Profile 固定绑定 Agent 和 runtime，因此在 Agent 硬件、字体包、浏览器版本不变时，这些原生表面也会自然保持稳定。运维升级可能改变它们，这属于 runtime 变更，不能伪装成数据库 Fingerprint 永久不变。

### 每次启动按代理注入

- 出口 IP、国家或地区；
- Locale、`Accept-Language`、`navigator.languages`；
- IANA Timezone。

IP 不参与 CPU、WebGL、屏幕或其他固定字段的生成。更换代理只改变本次地区覆盖和网络出口。

## 与历史 risk-data 的关系

`transfigure-risk-data` 当前活跃发布包含 FP-Controlink 2020—2021 的 1,148 个受控 Profile。它能证明某些历史组合确实出现过，也能帮助检查字段名和值域，但不能代表 2026 年市场份额、普通用户分布或当前 Chromium 默认值。

因此它只用于：

- 检查明显跨平台矛盾；
- 复核历史字段解析兼容；
- 设计防御性一致性测试。

它不进入自动模板随机生成逻辑，也不用于拼接“更像真人”的设备画像。

## 仍然存在的边界

### Runtime 尚无版本证明

控制面当前只收到 runtime key，没有对应二进制的 `version`、SHA-256 和 capability manifest。如果 Agent 将同一个 key 改指向其他 Chrome，控制面仍会生成 Chromium 150 模板。后续应让心跳上报每个 runtime 的版本、哈希和 capability set，再由控制面匹配模板。

### 尚未把完整 probe 内建到 Agent

Formal 门禁验证了 64 个 required-match 字段和 73 个稳定字段。new-api Agent 当前只强制代理，并预检语言和时区；完整 fingerprint probe 仍是发布/运维验收工具，而不是每次 OAuth 的前置步骤。

### WebGL noise seed 的运行时有效空间为 32 bit

new-api 保存的是 256-bit HMAC 字符串，但 Chromium parser 最终用 FNV-1a 压缩成单个 32-bit WebGL seed。生日碰撞概率约为：1,000 个 Profile 时 0.0116%，10,000 个时 1.16%，65,536 个时 39.35%。

常规小规模部署可接受；如果单集群接近一万个自动 Profile，应增加 seed 碰撞索引，或升级 runtime 的 seed 表示，不应假定 256-bit 输入等于 256-bit 有效扰动空间。

### 原生表面依赖 Agent 稳定

GPU 驱动、字体包、显示器和 Chromium 版本变化会改变原生表面。Profile 不应在没有显式迁移和重新验收的情况下换 Agent 或 runtime。

当前 Formal Chromium 二进制实际运行在 Linux Agent 上。`v4` 已验证 UA、Navigator 和 Client Hints 的 Windows/macOS 覆盖，但原生 GPU、字体、媒体栈和其他未覆盖表面仍来自该 Agent。BrowserScan 在本次 A/B 中为两套模板给出 100%，不代表所有检测站都会忽略底层运行时差异；需要更强的整机 OS 一致性时，应分别部署原生 Windows/macOS Agent 和对应 runtime，而不是继续增加不完整的字段伪装。

## 发布与验证规则

1. 模板内容变化必须修改模板 ID；不批量重写既有 Fingerprint。
2. runtime 只能指向已核对版本的 Formal Chromium，不能回退到系统 Chrome。
3. 固定 Fingerprint 禁止保存 locale、timezone、language、country 或出口 IP。
4. Agent 必须强制代理、WebRTC 策略、path + inline 配置，并禁止 CLI `--user-agent`。
5. 生产部署前核对 Chromium 版本、SHA-256 和完整 fingerprint probe。
6. 风险数据只用于防御性一致性分析，不参与模板采样。

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

网页验收至少覆盖 UA/CH、screen、hardware、Canvas/WebGL/Audio 稳定性、Canvas/Audio 未出现人工修改告警、不同 Profile 的 WebGL seed 差异、五个 WebRTC 读取面非 relay 地址为 0，以及语言/时区与代理 GeoIP 一致。
