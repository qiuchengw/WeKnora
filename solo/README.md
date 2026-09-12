# `solo/` —— WeKnora 下游车道（Solo 知识库引擎）

本目录是 **Solo 产品线对上游 WeKnora 的唯一增量**，且**只做构建、不改码**：上游源文件**零改动**（补丁仅在显式 `--with-patches` 时可选启用），所有差异以 **构建脚本 + CI** 形式存放于此。

- 上游基线：`Tencent/WeKnora`（远端 `origin`；fork：`qiuchengw/WeKnora`，远端 `fork`，车道分支：`solo`）
  - **当前基线 = 上游 `main` 合并点**（`2122a75`，2026-09-12 用户在 fork 侧同步；本地车道合并为 `d3a0113`）。历史基线 `v0.8.0`（tag）已升级。
  - 发布物仍按**具体 commit SHA**记录（main 是移动目标；升级后重跑补丁验证与 golden）
- 决策依据：`solo` 仓库 `docs/adr/0019-kb-engine-externalization.md`（D-13 构建来源纪律、**D-15 零补丁 canonical**、D-16 问数归属）
- 形态：**单进程无头服务**（`cmd/server`，`EDITION=lite`），由 Solo 宿主 spawn 一个子进程，走 `127.0.0.1 + token`。

## 为什么需要本车道（但不需要改码）

上游 Lite 目标在 Linux 上可零改码构建，Windows 上需要**两个构建期适配**（均不改源码）：

| 事实 | 处理方式 |
|---|---|
| 上游无 `windows/amd64` 资产，`release-lite.yml` 仅手动触发 | 本车道自建（CI：windows-latest） |
| `sqlite-vec` cgo 在 `SQLITE_CORE` 下 `#include "sqlite3.h"`，而 mattn/go-sqlite3 只带 `sqlite3-binding.h` | 脚本生成**构建输入头**目录（`solo/.build-headers/`），不进源码树 |
| DuckDB 预编译静态库是 GCC/libstdc++ ABI（LLVM-MinGW 只有 libc++，链不通） | 用 **GCC 系工具链**（msys2 UCRT64 / WinLibs）——工具链选对即可，**不需删任何代码** |

> 早期曾以“脚本化剥离（补丁删掉 DuckDB / 数据分析 / 表格摘要）”为默认。**2026-09-12 实验推翻**：仅因工具链选错。
> 实测（全新数据目录）：零补丁 **248.5MB / 2.39s / 237MB**；剥离版 202.6MB / 1.64s / 213.8MB——只省 46MB+23MB，却要每版刷补丁并丢掉**问数**能力。故降级为可选。

## 目录

```
solo/
  README.md                   本文件（车道说明 + 升级流程）
  patches/0001-rerank-in-hybrid-search.patch   【默认启用】能力补丁：hybrid-search 支持 enable_rerank
  patches/0002-data-analysis-endpoints.patch  【默认启用】能力补丁：问数执行 + 表格元信息端点（无 agent）
  patches-optional/0001-strip-stage1.patch     【--with-strip 才启用】体积补丁（DuckDB/数据分析/表格摘要）
  scripts/build-windows.sh    Windows/amd64 构建（应用补丁 → 生成 sqlite3.h → 编译）
.github/workflows/solo-lite-windows.yml   CI：windows-latest 出活 + SHA256
```

## 构建

```bash
# 本机（Windows，Git Bash 或 msys2）：
bash solo/scripts/build-windows.sh                 # 上游 tag + patches/ → dist/WeKnora-lite.exe
bash solo/scripts/build-windows.sh --no-patches     # 纯上游原版（对照/验证用）
bash solo/scripts/build-windows.sh --with-strip     # 额外剥离能力（体积应急）
bash solo/scripts/build-windows.sh --keep-symbols  # 体积画像（保留符号表）
```

工具链发现顺序：`MINGW_BIN` 环境变量 → `PATH` 上的 `gcc`。CI 用 msys2 `UCRT64`。

产物纪律：每个发布物记录 **上游 tag + 本车道 commit + 启用补丁清单 + 构建命令 + SHA256 + 许可清单**。

## 补丁策略（能力补丁默认启用 / 体积补丁可选）

**边界铁律（ADR-0019 D-16）**：引擎已具备的能力一律由引擎提供，宿主**不自建**。当上游只把能力藏在「内置 Agent」里、未暴露成独立端点时，我们的动作是**改造引擎**（能力端点化），而不是在宿主重写一份。

- `patches/`（默认启用）：**能力补丁**——附加式、每项对应一个上游 PR/Issue，上游合并后即删。当前：
  - `0001-rerank-in-hybrid-search.patch`：`SearchParams.enable_rerank` + `HybridSearch` 在融合后调用 tenant 配置的 rerank 模型（含单测 5 例；失败回退融合顺序）。
  - `0002-data-analysis-endpoints.patch`：`POST /knowledge/:id/data-analysis`（只读 SQL 执行）与 `GET /knowledge/:id/data-schema`（表元信息），复用内置 Agent 的 `data_analysis`/`data_schema` 工具实现；含 handler 单测 4 例 + 路由能力断言更新。
- `patches-optional/`（`--with-strip` 才启用）：**体积补丁**——删除 Lite 不可达能力（DuckDB 数据分析工具/表格摘要任务/悬空 `data_schema`）。仅当体积/内存成为发布阻塞时用；与能力无关（剥离后问数、rerank 走引擎新端点，不受影响）。

## 升级流程（上游出新 tag 时）

```bash
git fetch origin --tags                    # origin = 上游 Tencent/WeKnora
git checkout -b solo-v0.9 origin/v0.9       # 基线：新 tag
git checkout solo -- solo/ .github/workflows/solo-lite-windows.yml
bash solo/scripts/build-windows.sh          # 自动应用 patches/；冲突则显式报错退出
```

- `patches/` 是**附加式小补丁**（当前 12 文件 / ~+570 行），每项都有对应上游 PR；上游合并后补丁退役 ⇒ 冲突面随时间收敛。
- 若 `git apply --check` 失败，脚本**立即报错退出**（不静默兜底）：用 `git apply --3way` 解决冲突后**重新生成补丁**（注意：生成的 patch 必须是 **LF** 行尾——用 PowerShell `Set-Content` 可能写成 CRLF 导致应用失败，建议 `git diff ... > patch`）。
- 基线升级后必须：两补丁在干净基线可依次应用 + `go build ./...` + 本车道单测（`TestApplyRerank*` / `TestData*`）+ `router` 包自检 + 产物冒烟。2026-09-12 从 `v0.8.0` 升到上游 `main` 时已完成上述全流程（0001 直接可用；0002 因 `router` 两文件漂移重新生成）。
- 长期零维护路径：上游接受能力端点 PR（工具端点化 + `enable_rerank`）后，`patches/` 清空。

## 已知待办（能力端点化路线）

目标形态：**宿主 Agent（唯一编排） + 引擎（无 agent 的纯能力面）**。

- 已就绪：`hybrid-search`（召回）+ `enable_rerank`（重排）；**问数端点** `POST /knowledge/:id/data-analysis` + `GET /knowledge/:id/data-schema`（E2/E3，DuckDB 执行与表格解析均在引擎）。
- 宿主侧：`KbEnginePort` 收口端点；工具注册走已有 `agent.mastra.tool` 机制（`kb_search` / `kb_data_schema` / `kb_data_query`）；MCP 作为后续可选薄壳。宿主 Agent 只做“问题→只读 SQL”的规划与结果解释。
- 待定：`POST /engine-tools/{tool}` 通用工具端点（E5，仅在出现第二消费方时推进）。
- 许可清单随包（`scripts/copy-licenses.sh`）与发布物装配（Solo 侧 W5）。
