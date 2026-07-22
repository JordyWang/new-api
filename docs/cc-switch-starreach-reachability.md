# CC Switch 对 Starreach 返回 404 的调查报告

## 文档信息

- 调查日期：2026-07-21
- 调查对象：`https://starreach.xyz/v1`
- 对比对象：`https://prod-ai-gateway.timeresearch.biz:4000/v1`
- 状态：根因已确认；Responses 兼容层和裸 `/v1` 服务信息路由已在代码中实现并验收，生产环境需部署后才会生效

## 摘要

CC Switch 对 Starreach 的测试结果 `operational / success=1 / http_status=404` 不矛盾。当前 CC Switch 执行的是轻量级 HTTP 可达性探测，而不是完整 API 功能测试：它向供应商配置的 `base_url` 发起 `GET` 请求，只要收到任意 HTTP 响应，就认为目标可达。HTTP 401、404、500 或 503 都可以对应 `success=true`；只有 DNS、连接、TLS 或超时等网络级错误才会判定为不可达。

Starreach 的 Base URL 是 `https://starreach.xyz/v1`。调查时的 new-api 没有注册 `GET /v1`，因此请求进入 API 404 处理器并返回 404。CC Switch 收到了有效 HTTP 响应，所以仍将它记录为 `Reachable`。当前代码已为 `GET` 和 `HEAD /v1` 注册公开的最小服务信息路由；部署后 CC Switch 对该 Base URL 的探测将收到 200。

这说明：

- 调查时 `GET /v1` 返回 404 不影响 Codex 使用的流式 `POST /v1/responses`；当前代码已将裸 `/v1` 调整为 200。
- CC Switch 的绿色状态只能证明地址可连接，不能证明鉴权、模型、请求格式或真实推理链路可用。
- 调查时的生产版本存在 Responses API 兼容性缺口：字符串 `input` 和非流式请求会返回 400；当前代码已增加 Codex 渠道兼容层，生产环境需部署后才会生效。
- 当前 `/healthz` 和 `/readyz` 虽然返回 200，但内容是前端 HTML，并不是真实健康检查。

## 原始现象

生产节点上的 CC Switch 记录为：

```text
provider: starreach
status: operational
success: 1
message: Reachable
http_status: 404
response_time: 124 ms
```

表面上看，`success=1` 与 `http_status=404` 相互冲突；实际上二者描述的是不同层级：

- `success=1`：已建立连接并收到 HTTP 响应。
- `http_status=404`：目标服务没有为本次 `GET /v1` 注册路由。
- `operational`：请求耗时没有超过 CC Switch 的降级阈值。

## 根因分析

### 1. CC Switch 只探测 Base URL 是否可达

本地核对的 CC Switch 源码版本为 v3.16.5；该行为自 v3.16.3 起改为轻量级可达性检查。

其执行链路如下：

```text
读取 provider.base_url
  -> GET base_url
  -> 收到任意 HTTP 响应
  -> success=true，保存实际 http_status
```

源码中的关键行为：

- 使用 `GET` 请求供应商的 `base_url`。
- `reqwest` 收到任何 HTTP 状态都会返回成功结果。
- 只有网络级错误进入失败分支。
- 内置测试明确覆盖 200、401、403、404、429、500 和 503，全部应判定为可达。

参考：

- [CC Switch 可达性检查实现](https://github.com/farion1231/cc-switch/blob/v3.16.5/src-tauri/src/services/stream_check.rs#L214-L276)
- [CC Switch 对任意 HTTP 状态的测试](https://github.com/farion1231/cc-switch/blob/v3.16.5/src-tauri/src/services/stream_check.rs#L445-L455)
- [CC Switch v3.16.3 发布说明](https://github.com/farion1231/cc-switch/blob/v3.16.5/docs/release-notes/v3.16.3-en.md#lightweight-provider-health-check)

因此，不能根据 CC Switch 显示“成功”推断对比中转站的裸 `/v1` 一定返回 200；它即使返回 404，仍可能被判定为可达。

### 2. 调查时 new-api 没有注册 `GET /v1`

new-api 为 Responses API 注册的是：

```text
POST /v1/responses
POST /v1/responses/compact
```

调查时没有注册 `GET /v1`，也没有注册 `GET /v1/responses`。当前代码已增加 `GET` 和 `HEAD /v1`，`GET /v1/responses` 的 405 语义仍未实现。相关路由见 [`router/relay-router.go`](../router/relay-router.go)。

所有未匹配的 `/v1` 请求都会进入统一 API 404 分支，见 [`router/web-router.go`](../router/web-router.go#L33-L44) 和 [`controller/relay.go`](../controller/relay.go#L464-L473)。因此裸请求返回：

```http
HTTP/2 404
Content-Type: application/json; charset=utf-8

{
  "error": {
    "message": "Invalid URL (GET /v1)",
    "type": "invalid_request_error",
    "param": "",
    "code": ""
  }
}
```

修复前的完整因果链为：

```text
CC Switch GET https://starreach.xyz/v1
  -> new-api 没有 GET /v1
  -> NoRoute 返回 404 JSON
  -> CC Switch 成功收到响应头
  -> Reachable / success=true / http_status=404
```

## 修复前的真实接口验证

使用 CC Switch 已保存的 Starreach provider 配置进行了脱敏请求验证。测试没有记录或输出访问令牌。本节记录的是兼容修复前的生产基线，不能用于判断当前代码或后续部署版本的行为。

### HTTP 路由与错误语义

| 请求 | 实际结果 | 结论 |
| --- | --- | --- |
| `GET /v1` | 404 JSON | 没有裸 Base URL 路由 |
| `HEAD /v1` | 404 JSON | 没有显式 HEAD 路由 |
| `GET /v1/responses` | 404 JSON | 当前没有返回 405 |
| `POST /v1/responses`，无 token | 401 JSON | 鉴权语义正确 |
| `POST /v1/responses`，不存在的模型 | 503 JSON | 无可用渠道语义正确 |
| 未知 `/v1/*` 路径 | 404 JSON | 未知 API 路径语义正确 |

### Responses API 请求矩阵

测试模型为 `gpt-5.6-sol`，提示词要求仅回复 `OK`。

| `input` | `stream` | HTTP 状态 | 结果 |
| --- | --- | --- | --- |
| 字符串 | `true` | 400 | `Input must be a list` |
| 数组 | 省略 | 400 | `Stream must be set to true` |
| 数组 | `false` | 400 | `Stream must be set to true` |
| 数组 | `true` | 200 | 正常返回 SSE，并以 `response.completed` 结束 |

成功请求可观察到以下事件：

```text
response.created
response.in_progress
response.output_item.added
response.content_part.added
response.output_text.delta
response.output_text.done
response.output_item.done
response.completed
```

因此，调查时的生产版本可满足 Codex CLI 使用的数组输入和流式请求，但不满足完整的 OpenAI Responses API 请求契约。

## 根因与兼容层实现

new-api 的请求 DTO 将 `input` 保存为原始 JSON，并将 `stream` 定义为可选布尔值，省略时按 `false` 处理，见 [`dto/openai_request.go`](../dto/openai_request.go#L839-L869) 和 [`dto/openai_request.go`](../dto/openai_request.go#L947-L949)。

修复前，Codex 渠道适配器只会补充 `instructions`、强制 `store=false` 并删除部分字段，不会把字符串 `input` 转换成数组，也不会为非流式客户端聚合上游 SSE。随后请求被转发到 Codex 上游的 `/backend-api/codex/responses`，因此上游的两项限制直接暴露给了客户端。

实测得到的两个错误字符串不在 new-api 或 CC Switch 源码中。结合适配器的原样转发行为，可以确认它们是 Codex 上游约束经网关透传后的结果，而不是 Starreach 路由层主动实施的校验。

官方 OpenAI Responses API 则支持：

- `input` 使用字符串或结构化数组。
- 默认生成完整响应并返回单个 JSON HTTP 响应。
- 只有显式设置 `stream=true` 时才使用 SSE。

参考：

- [OpenAI Create response API](https://developers.openai.com/api/reference/resources/responses/methods/create)
- [OpenAI Streaming API responses](https://developers.openai.com/api/docs/guides/streaming-responses)

当前代码在 Codex 渠道边界增加了兼容处理：

1. 字符串 `input` 被规范化为包含 `input_text` 的用户消息数组；客户端原本提供的数组保持不变。
2. 普通 `/v1/responses` 请求转发上游时始终使用 `stream=true`，并显式请求 `text/event-stream`。
3. 流式客户端继续收到 SSE；非流式客户端由网关消费上游 SSE，从 `response.completed` 中取出完整 Response，并返回 `application/json`。
4. 聚合路径继续提取 usage、图片生成标记和内置工具信息，供计费与日志链路使用。
5. `/v1/responses/compact` 保持原有请求与响应语义，不强制切换为流式。

实现见 [`relay/channel/codex/adaptor.go`](../relay/channel/codex/adaptor.go) 和 [`relay/channel/openai/relay_responses.go`](../relay/channel/openai/relay_responses.go)。这层转换只解决 Responses API 契约差异；`GET /v1` 的状态由独立的公开服务信息路由处理。

## 本地兼容修复验收

本地 new-api 连接了一个严格模拟 Codex 的上游。该模拟上游只接受数组 `input` 和 `stream=true`，其他请求直接返回与现场相同的 400。通过本地 API Token 访问 `/v1/responses` 的结果如下：

| 客户端 `input` | 客户端 `stream` | 上游实际收到 | 下游结果 |
| --- | --- | --- | --- |
| 字符串 | 省略 | 数组、`true` | 200 JSON |
| 字符串 | `false` | 数组、`true` | 200 JSON |
| 数组 | `false` | 数组、`true` | 200 JSON |
| 字符串 | `true` | 数组、`true` | 200 SSE |
| 数组 | `true` | 数组、`true` | 200 SSE |

非流式响应保留了 `id`、`object`、`status`、`model`、`output` 和 usage，响应类型为 `application/json`；流式响应保持 `text/event-stream` 和 `response.completed` 事件。管理端 `/api/channel/test/:id` 同样成功。

这项调整与既有的 Codex 通道测试强制流式逻辑不冲突：通道测试的客户端语义本来就是流式，因此继续走 SSE 透传；普通客户端请求为非流式时，才走新增的 SSE 到 JSON 聚合路径。

## 健康检查路由现状

当前实测：

| 请求 | HTTP 状态 | Content-Type | 实际内容 |
| --- | --- | --- | --- |
| `GET /healthz` | 200 | `text/html` | 前端 SPA 首页 |
| `HEAD /healthz` | 200 | `text/html` | 前端 SPA 回退响应 |
| `GET /readyz` | 200 | `text/html` | 前端 SPA 首页 |
| `HEAD /readyz` | 200 | `text/html` | 前端 SPA 回退响应 |

这是因为 web router 会把所有不以 `/v1`、`/api` 或 `/assets` 开头的未知路径回退到前端首页。它们不是健康检查，只是恰好返回 200。监控系统如果只检查状态码，会产生假健康结果。

建议为这些路径注册显式路由，并保证它们先于 SPA fallback 生效。

## 实施状态与后续建议

### 已实现：完善 Responses API 兼容性

#### 支持字符串 `input`

Codex 渠道收到字符串输入时，会在转发上游前将其规范化为上游接受的数组形式。例如：

```json
{
  "model": "gpt-5.6-sol",
  "input": "Hello"
}
```

应等价转换为：

```json
{
  "model": "gpt-5.6-sol",
  "input": [
    {
      "role": "user",
      "content": [
        {
          "type": "input_text",
          "text": "Hello"
        }
      ]
    }
  ]
}
```

#### 支持非流式请求

Codex 上游要求 `stream=true`。为了维持客户端的标准契约，网关已经实现：

1. 对上游请求强制使用流式模式。
2. 在服务端消费并校验完整 SSE 事件流。
3. 聚合 `response.completed`、输出项、用量及错误信息。
4. 向非流式客户端返回标准 JSON Response。

实现没有把上游 SSE 直接返回给非流式客户端；否则会违反客户端请求的非流式响应契约。

### P0：增加真实健康检查

建议行为：

```http
GET  /healthz -> 200 application/json
HEAD /healthz -> 200
```

示例响应：

```json
{
  "status": "ok"
}
```

`/readyz` 应反映实例及必要依赖是否已经能够接收请求：

```http
GET  /readyz -> 200 或 503 application/json
HEAD /readyz -> 200 或 503
```

不建议用一个全局 readiness 状态表示所有模型和上游渠道都可用。new-api 支持多个模型、渠道和故障转移路径，具体上游可用性更适合通过内部监控、合成请求或受保护的渠道状态接口展示。

### 已实现：为裸 `/v1` 返回最小服务信息

当前代码已增加：

```http
GET  /v1 -> 200 application/json
HEAD /v1 -> 200
```

示例响应：

```json
{
  "status": "ok",
  "service": "new-api",
  "protocol": "openai-compatible"
}
```

响应中不应暴露访问令牌、上游渠道、账户、内部模型映射、数据库状态或基础设施信息。

注意：增加 `/healthz` 本身不会改变 CC Switch 记录的 404，因为当前 CC Switch 始终探测 provider 的 `base_url`，也就是 `/v1`。本次增加的 `GET /v1` 会在部署后将 CC Switch 记录的原始 `http_status` 改为 200。

### P2：完善 HTTP 方法语义

目标行为：

| 请求 | 目标状态 |
| --- | --- |
| `GET /v1` | 200 |
| `HEAD /v1` | 200 |
| `GET /healthz` | 200 |
| `HEAD /healthz` | 200 |
| `GET /readyz` | 200 或 503 |
| `POST /v1/responses` | 支持流式与非流式 |
| `GET /v1/responses` | 405 |
| 已存在接口缺少 token | 401 |
| 模型没有可用渠道 | 503 |
| 未知 API 路径 | 404 |

Gin 默认不会自动把已知路径上的错误方法转换为 405。实现时可以评估启用统一的 Method Not Allowed 处理，或为关键 API 增加显式方法处理器，同时验证 CORS、OPTIONS、鉴权中间件和现有 NoRoute 行为不受影响。

## 验收状态

本次 Responses 兼容修复已覆盖：

1. 字符串和数组两种 `input`。
2. 省略 `stream`、显式 `stream=false` 和 `stream=true`。
3. 非流式 SSE 聚合后的输出、usage、模型、状态和 JSON Content-Type。
4. 流式 SSE 透传及 Codex 管理端通道测试。
5. `/v1/responses/compact` 不受普通 Responses 强制流式逻辑影响。

裸 `/v1` 服务信息路由已实现，本地路由级回归测试已覆盖 `GET` 和 `HEAD`的 200、JSON Content-Type、HEAD 空响应体及公开响应不包含敏感信息。部署后仍需用 CC Switch 确认现场记录已变为 `http_status=200`。

其余网页路由优化尚未实施，后续验收仍需覆盖：

1. `GET` 和 `HEAD /healthz` 返回 200，Content-Type 为 JSON 或 HEAD 的预期空响应，不再返回 HTML。
2. readiness 正常时 `/readyz` 返回 200，必要依赖不可用时返回 503。
3. `GET /v1/responses` 返回 405，未知 `/v1/*` 仍返回 404。
4. 缺少 token 仍返回 401，无可用模型渠道仍返回 503。

## 调查限制

- 当前执行环境无法解析 SSH 别名 `sg-snowball-prod-yufan-01`，通过本地 HTTP 代理也未能完成 SSH banner 交换，因此没有独立重新读取远端 CC Switch 数据库；本文引用的远端记录来自问题现场提供的数据。
- 当前网络无法完成对对比地址 `https://prod-ai-gateway.timeresearch.biz:4000/v1` 的 TLS 握手，因此本文不推断该地址的实际 `GET /v1` 状态码。
- 尝试执行 CC Switch 的目标 Rust 单测时，本机缺少 `gobject-2.0 >= 2.70` 开发依赖，测试未完成编译。本文关于 CC Switch 的结论来自对应版本源码、发布说明、现场数据库记录以及 Starreach 的真实请求结果，不声称 CC Switch 测试套件已通过。

## 最终结论

Starreach 调查时的 404 是裸 `GET /v1` 路由缺失造成的。CC Switch 将该请求记录为成功，是因为其测试目标是 HTTP 可达性，而不是完整 API 功能。当前代码已增加裸 `/v1` 公开服务信息路由，部署后可消除 CC Switch 记录中的 404。

调查时确认的字符串输入和非流式兼容性缺口已在当前代码中修复并完成本地验收，生产行为仍以实际部署版本为准。`/healthz`、`/readyz` 和 `GET /v1/responses` 的 405 语义仍是独立后续项。
