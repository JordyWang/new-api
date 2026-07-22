# Codex 系统 token 与 CC Switch 统计差异调查

调查日期：2026-07-21（Asia/Shanghai）
调查范围：本机 Codex 会话状态、Codex rollout JSONL、CC Switch SQLite 与统计实现；未修改业务代码或数据库。

## 结论

1. 系统显示的 `51,054,480` tokens 是真实的累计“模型处理量”，不是一次查询读取了五千多万新内容，也没有发现系统侧重复记账。该值与目标会话 rollout 最后一条 `total_token_usage.total_tokens` 完全一致。
2. 高数值的直接原因是：该会话产生了 **370 次非零模型调用**，每次调用都会重新携带当时的完整上下文。平均每次输入 `137,581` tokens，长上下文被反复处理后累计到五千多万。
3. `50,905,150` 个输入 tokens 中，`49,665,536` 是缓存输入，占输入量 **97.565%**。真正的非缓存输入只有 `1,239,614`，另有输出 `149,330`。因此“五千万”主要表示缓存上下文被反复读取，不等于五千万新 token，也不能直接按非缓存输入价格理解成本。
4. CC Switch 当前数据库对同一会话记录为 `49,408,333` tokens，覆盖系统值的 **96.776%**，并不是 `2,000 万+`。因此“CC Switch 只有 2,000 万+”不是该会话结束后的最终数据库值，更符合查看时会话仍在运行、同步未完成，或界面存在应用/Provider/模型/时间过滤的瞬时结果。
5. 仍然确认存在一个较小的 CC Switch 同步偏差：当前比系统少 `1,646,147` tokens（**3.224%**）。逐条比对发现 370 条记录中有 50 条与最终 rollout 的相同序号不一致；目标会话期间又发生了 9 次中止、2 次回滚和 3 次上下文压缩。CC Switch 使用行偏移增量同步，并以会话内事件序号作为不可覆盖的主键，且没有针对 Codex 回滚事件做重建，这能够解释最终留下的错位记录。
6. `new-api` 不参与上述本地 token 累计。项目中的 CC Switch 功能只生成 `ccswitch://v1/import` 链接；Codex 渠道“用量”接口读取的是 ChatGPT WHAM 限流窗口，也不是本地会话 token 统计。

## 一、核对对象

本次定位到系统中 `5kw+` 对应的会话：

```text
session_id: 019f8230-0bab-7210-9a18-d80d71bc6661
开始时间: 2026-07-21 09:00:14
结束时间: 2026-07-21 20:46:15
模型: gpt-5.6-sol
reasoning effort: high
```

权威数据源如下：

| 数据源 | 用途 | 结果 |
|---|---|---:|
| `~/.codex/state_5.sqlite` 的 `threads.tokens_used` | Codex 系统会话累计值 | 51,054,480 |
| 目标 rollout 最后一条 `total_token_usage` | 上游逐次累计 token 的原始证据 | 51,054,480 |
| `~/.cc-switch/cc-switch.db` 的同 session 汇总 | CC Switch 已落库值 | 49,408,333 |

系统数据库与 rollout 完全相等，证明系统显示值来自上游 usage 累计，不是 UI 自行估算或重复汇总。

## 二、51,054,480 是怎么形成的

### 2.1 精确拆分

目标 rollout 最终累计值：

| 项目 | Tokens | 占比/说明 |
|---|---:|---|
| 输入 tokens | 50,905,150 | 总量的 99.71% |
| 其中缓存输入 | 49,665,536 | 输入量的 97.565% |
| 非缓存输入 | 1,239,614 | 输入量的 2.435% |
| 输出 tokens | 149,330 | 总量的 0.29% |
| 系统累计总量 | 51,054,480 | 输入 + 输出 |

这里的“缓存”只代表模型服务可以按缓存读取方式处理和计价；每次请求的 usage 仍会把缓存输入计入已处理 tokens，所以系统总数不会因为缓存命中而只增加新内容。

### 2.2 调用次数放大长上下文

会话中共有 390 条 `token_count` 事件，其中 20 条为零增量，实际有 **370 次非零模型调用**：

```text
平均每次输入: 137,581 tokens
平均每次输出:     403 tokens
平均每次总量: 137,985 tokens
```

按单次输入规模分布：

| 单次输入范围 | 调用数 | 贡献总 tokens |
|---|---:|---:|
| 0–2 万 | 2 | 38,987 |
| 2–5 万 | 51 | 1,864,807 |
| 5–10 万 | 86 | 6,642,227 |
| 10–15 万 | 47 | 6,143,820 |
| 15–20 万 | 100 | 17,926,423 |
| 20–25 万 | 84 | 18,438,216 |

其中 184 次调用的输入超过 15 万，合计贡献约 3,636 万 tokens，占总量约 71%。所以核心公式是：

```text
累计处理量 ≈ 每次完整上下文长度 × 模型调用次数
```

不是“执行一条查询就消耗五千万”，而是每次工具返回后都要再次调用模型；模型再次看到系统指令、会话历史、工具定义、之前的工具结果及当前状态。

### 2.3 会话为什么产生 370 次模型调用

从 rollout 只统计事件类型、不读取业务内容，得到：

| 行为 | 次数/规模 |
|---|---:|
| `exec` 工具调用 | 322 次 |
| 其他 function call | 36 次 |
| `exec` 工具输出文本 | 2,090,530 字符 |
| 用户/自动续跑消息 | 24 次 |
| turn context | 22 次 |
| 上下文压缩 | 3 次 |
| turn 中止 | 9 次 |
| thread 回滚 | 2 次 |
| 子代理启动 | 8 次 |

工具调用本身不是主要 token；真正的放大器是每个工具调用后都会触发下一次模型推理，而当时上下文经常已达到 15–24 万 tokens。仅 `exec` 工具就返回了 2,090,530 字符，这些输出也持续进入上下文，进一步抬高后续每次调用的输入长度。

三次上下文压缩降低了单次上下文，但不会抹掉之前已经发生的 usage；所以系统累计值只增不减。

## 三、系统和 CC Switch 的统计口径是否不同

对 Codex 来说，两者最终展示口径本应基本相同。

### 系统口径

```text
total_tokens = input_tokens + output_tokens
```

其中 `input_tokens` 包含 `cached_input_tokens`。

### CC Switch 口径

CC Switch 先把 Codex 输入拆成：

```text
fresh_input = input_tokens - cache_read_tokens
```

首页“真实消耗 Tokens”再计算：

```text
real_total = fresh_input + cache_read + cache_creation + output
```

Codex 没有独立 cache creation 数字，所以代入后仍等于：

```text
real_total = input_tokens + output_tokens
```

因此 `5,000 万` 对 `2,000 万` 不是缓存口径本身造成的。缓存会改变“新输入”“缓存命中率”和费用，但 CC Switch 的“真实消耗 Tokens”仍把缓存读取加回总量。

相关实现证据：

- CC Switch 从 Codex rollout 的累计 usage 计算 delta：`src-tauri/src/services/session_usage_codex.rs` 第 107–140、348–395 行。
- Codex/Gemini 的 fresh input 会减去缓存读取：`src-tauri/src/services/sql_helpers.rs` 第 19–46 行。
- 首页 real total 明确包含 fresh input、output、cache creation、cache read：`src-tauri/src/services/usage_stats.rs` 第 26–32 行和 `src/components/usage/UsageHero.tsx` 第 196–203 行。

## 四、CC Switch 显示偏低的调查

### 4.1 当前最终库值

同一 session 在 CC Switch 的 370 条记录汇总为：

| 项目 | Tokens |
|---|---:|
| 非缓存输入 | 1,024,964 |
| 缓存读取 | 48,235,008 |
| 输出 | 148,361 |
| CC Switch real total | 49,408,333 |
| 系统 total | 51,054,480 |
| 差额 | 1,646,147（3.224%） |

所以当前证据不能复现“CC Switch 最终只有 2,000 万+”。如果当时界面确实显示 2,000 万，需用截图中的时间、筛选项和截取时刻才能一比一还原；现有数据库证明它不是最终稳定值。

### 4.2 已确认的 3.224% 同步偏差

CC Switch 每 60 秒扫描一次所有 Codex session 文件。同步器保存 `last_line_offset`，从 `total_token_usage` 的前后累计值算 delta，并使用：

```text
codex_session:<session_id>:<event_index>
```

作为 `INSERT OR IGNORE` 的 request id。

对最终 rollout 重新计算 370 个非零 delta，再与 CC Switch 370 行逐序号比较：

```text
事件数:       370 vs 370
不一致事件:    50
总量净差: 1,646,147
```

目标会话期间有 9 次 turn abort、2 次 thread rollback 和 3 次 context compaction。同步实现只识别 `session_meta`、`turn_context` 和 `token_count`，没有处理 rollback；已经按事件序号插入的行也不会被新内容覆盖。会话文件在中止/回滚后的事件序列变化，会留下旧序号数据或阻止同序号新值更新。这与实际发现的 50 条错位记录吻合。

该问题只能解释当前约 3.2% 的偏差，不能单独解释结束后仍差 3,000 万。`2,000 万+` 更可能是以下瞬时条件之一：

1. 查看时会话仍在运行或 CC Switch 还未完成下一轮同步；同步周期是 60 秒，界面还存在查询刷新周期。
2. 界面选中了 Codex、特定 Provider、特定模型或自定义时间范围，而系统会话数字是整条 thread 的全生命周期累计。
3. 查看的是某一阶段的快照。该会话在 10:37 左右首次超过 2,000 万，在 20:40 的后续运行中才超过 5,000 万。
4. 中止/回滚造成增量库中的旧序号记录与最终 rollout 不一致；这是已证实的小幅误差来源。

## 五、new-api 是否造成重复统计

没有发现 new-api 在这条链路中重复计算本机 Codex usage：

- `web/default/src/features/keys/components/dialogs/cc-switch-dialog.tsx` 第 69–88、135–145 行仅构造并打开 Provider 导入链接。
- `controller/codex_usage.go` 调用 `service.FetchCodexWhamUsage`。
- `service/codex_wham_usage.go` 第 38 行访问 `/backend-api/wham/usage`，返回的是 Codex 套餐限流窗口/百分比，不读取 `~/.codex` 或 `~/.cc-switch`。

本次差异发生在 Codex 本地会话 usage 与 CC Switch 的本地 session 增量同步之间，与 new-api 的配额、计费、日志聚合无关。

## 六、如何降低后续消耗

无需改代码即可明显降低同类会话的 processed token：

1. 长任务按阶段新开 thread，避免在 15–24 万上下文上继续做数百次工具循环。
2. 把机械查询合并成一次受限输出的命令；优先聚合、计数和精确筛选，避免反复返回大段原文。
3. 为 `rg`、日志和数据库查询设置更小的结果范围，仅在定位后扩大；本会话工具输出累计超过 209 万字符。
4. 机械巡检或批处理减少逐步“查一次—推理一次”的循环，先由脚本完成聚合，再让模型分析汇总结果。
5. 非必要阶段不要持续使用高 reasoning effort；高推理更适合关键判断，不适合每个机械步骤。
6. 判断成本时同时看“非缓存输入、缓存读取、输出和实际费用”，不要只看包含缓存的 processed token 总数。

## 七、复核命令

以下命令均为只读。执行时应替换目标 session id；不要输出会话正文或凭证字段。

```bash
# Codex 系统累计
sqlite3 -readonly ~/.codex/state_5.sqlite \
  "SELECT id, tokens_used FROM threads WHERE id='<session_id>';"

# CC Switch 同一会话拆分
sqlite3 -readonly ~/.cc-switch/cc-switch.db \
  "SELECT COUNT(*),
          SUM(input_tokens-cache_read_tokens) AS fresh_input,
          SUM(cache_read_tokens) AS cache_read,
          SUM(output_tokens) AS output,
          SUM(input_tokens+output_tokens) AS real_total
     FROM proxy_request_logs
    WHERE session_id='<session_id>';"

# CC Switch 同步进度
sqlite3 -readonly ~/.cc-switch/cc-switch.db \
  "SELECT file_path, last_line_offset,
          datetime(last_synced_at,'unixepoch','localtime')
     FROM session_log_sync
    WHERE file_path LIKE '%<session_id>%';"
```

## 最终判断

这次 `5kw+` 的首要原因不是 token 计算错误，而是一个持续近 12 小时、包含 370 次模型调用的长上下文代理会话。**系统总量大，但 97.565% 是缓存输入；真正新增输入约 124 万。**

CC Switch 与系统的最终口径本应一致。当前数据库已记录 4,940.8 万，说明“2kw+”是过程中的不完整或带过滤快照；另外确实存在 164.6 万、约 3.2% 的回滚/增量同步偏差。二者需要分开看，不能用 CC Switch 当时的 2,000 万界面值否定系统的 5,105 万累计 usage。
