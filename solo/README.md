# `solo/` —— WeKnora 下游车道（Solo 知识库引擎）

本目录是 **Solo 产品线对上游 WeKnora 的增量**，承载构建脚本与 CI 说明；**源码改动本身以 `solo` 分支的提交承载**（见下）。

- 上游基线：`Tencent/WeKnora`（远端 `origin`；fork：`qiuchengw/WeKnora`，远端 `fork-ssh`，车道分支：`solo`）
  - **当前基线 = 上游 `main` 合并点**（`2122a75`，2026-09-12 用户在 fork 侧同步）。
  - 发布物按**具体 commit SHA** 记录：上游基线 SHA + 本车道分支 SHA + 构建命令 + SHA256（`main` 是移动目标）。
- 决策依据：`solo` 仓库 `docs/adr/0019-kb-engine-externalization.md`（D-13 构建来源纪律、D-15 车道模型、D-16 问数归属）。
- 形态：**单进程无头服务**（`cmd/server`，`EDITION=lite`），由 Solo 宿主 spawn 一个子进程，走 `127.0.0.1 + token`。

## 车道模型（2026-09-12 收敛：分支提交，不再是 patch 文件）

**产物源码 = 本仓 `solo` 分支 HEAD**（上游 `main` + 我们的改动）。

| | patch 文件车道（已退役） | **分支提交车道（现行）** |
|---|---|---|
| 我们的改动存放 | `solo/patches/*.patch`（构建时 `git apply`） | `solo` 分支上的提交（**一 commit = 一个上游 PR 候选**） |
| 上游漂移 | 每个 patch 逐个 3-way + **重新生成 patch 文件** | `git merge origin/main`，冲突在 git 里解决一次 |
| 真相数量 | 两处：patch 文件 + 工作树「设计态」脏 | 一处：分支 HEAD（`git status` 干净） |
| 构建 | 需先应用补丁（失败要显式报错兜底） | 直接构建当前分支 |

- 为何退役：patch 只是同一份分叉的另一种编码，却把「上游一动」变成「逐个重生成」的持续成本；git merge 的 3-way 能力显著强于 `git apply` 的上下文匹配。
- 保留的纪律：**一个 commit = 一个上游 PR 候选**（commit message 写明动机 + 实测证据），便于随时逐个提 PR；上游合并后对应 commit 自然成为空操作（rebase/merge 时消失）。
- 纯上游对照构建：直接 `git checkout origin/main` 构建，不再需要 `--no-patches`。

**车道当前内容（`git log origin/main..solo`）**：

| commit | 变更 | 性质 |
|---|---|---|
| `02d3ea5` | E1 passage enrichment（标题进入重排语料，修 title-blind rerank） | 能力修复（上游 PR 候选） |
| `6364086` | `hybrid-search` 支持 `enable_rerank`（含单测；重排失败回退融合序） | 能力端点（上游 PR 候选） |
| `bf2f05e` | 问数端点 `POST /knowledge/:id/data-analysis` + `GET /knowledge/:id/data-schema` | 能力端点（上游 PR 候选） |
| `2d558e4` | DuckDB 扩展安装**离线优先 + 探活门控**（无外网不再卡启动） | 健壮性（上游 PR 候选） |
| `976bb5b` | `hybrid-search` 响应**回传 `content_revision`**（引用回链四件套齐备；新增响应投影，内部 `json:"-"` 语义与存储载荷不变） | 能力修复（上游 PR 候选） |

## 为什么需要本车道（但源码改动很少）

上游 Lite 目标在 Linux 上可零改码构建，Windows 上需要**两个构建期适配**（均不改源码）：

| 事实 | 处理方式 |
|---|---|
| 上游无 `windows/amd64` 资产，`release-lite.yml` 仅手动触发 | 本车道自建（CI：windows-latest） |
| `sqlite-vec` cgo 在 `SQLITE_CORE` 下 `#include "sqlite3.h"`，而 mattn/go-sqlite3 只带 `sqlite3-binding.h` | 脚本生成**构建输入头**目录（`solo/.build-headers/`），不进源码树 |
| DuckDB 预编译静态库是 GCC/libstdc++ ABI（LLVM-MinGW 只有 libc++，链不通） | 用 **GCC 系工具链**（msys2 UCRT64 / WinLibs）——工具链选对即可，**不需删任何代码** |

> 早期曾以“脚本化剥离（删掉 DuckDB / 数据分析 / 表格摘要）”为默认。**2026-09-12 实验推翻**：仅因工具链选错。
> 实测（全新数据目录）：零补丁 **248.5MB / 2.39s / 237MB**；剥离版 202.6MB / 1.64s / 213.8MB——只省 46MB+23MB，却要每版刷代码并丢掉**问数**能力。故剥离方案退役。

## 目录

```
solo/
  README.md                    本文件（车道说明 + 同步流程）
  scripts/build-windows.sh     Windows/amd64 构建（生成头 shim → CGO 编译；直接构建当前分支）
.github/workflows/solo-lite-windows.yml   CI：windows-latest 出活 + SHA256
```

## 构建

```bash
# 本机（Windows，Git Bash 或 msys2）：
bash solo/scripts/build-windows.sh                 # 当前分支 HEAD → dist/WeKnora-lite.exe
bash solo/scripts/build-windows.sh --keep-symbols  # 体积画像（保留符号表）

# 纯上游对照（不切分支即可）：git worktree add /tmp/upstream origin/main && cd /tmp/upstream && bash <本脚本>
```

工具链发现顺序：`MINGW_BIN` 环境变量 → `PATH` 上的 `gcc`。CI 用 msys2 `UCRT64`。

产物纪律：每个发布物记录 **上游基线 SHA + 本车道分支 SHA + 构建命令 + SHA256 + 许可清单**。

## 上游同步流程（上游出新 tag/推进 main 时）

```bash
git fetch origin --tags                 # origin = 上游 Tencent/WeKnora
git checkout solo
git merge origin/main                   # 冲突一次解决（这就是退役 patch 车道的主要收益）
go build ./... && go test ./internal/... # 车道自检（含新增端点/健壮性用例）
git push fork-ssh solo
```

- 冲突解决原则：**行为对齐上游**优先；我们的能力端点保持附加式（不改变上游既有语义）。
- 每次同步后重跑约定：`go build ./...`、车道单测（`TestApplyRerank*` / `TestData*`）、`router` 包自检、产物冒烟（`EDITION=lite` 出活 + `--version`）。
- 上游接受对应 PR 后：rebase/merge 会把这些 commit 自然吞掉（becomes empty），届时从本车道删除即可。

## 已知待办（能力端点化路线）

目标形态：**宿主 Agent（唯一编排） + 引擎（无 agent 的纯能力面）**。

- 已就绪：`hybrid-search`（召回）+ `enable_rerank`（重排）；**问数端点** `POST /knowledge/:id/data-analysis` + `GET /knowledge/:id/data-schema`（DuckDB 执行与表格解析均在引擎）。
- 宿主侧：`KbEnginePort` 收口端点；工具注册走已有 `agent.mastra.tool` 机制（`kb_search` / `kb_data_schema` / `kb_data_query`）；MCP 作为后续可选薄壳。宿主 Agent 只做“问题→只读 SQL”的规划与结果解释。
- 待定：`POST /engine-tools/{tool}` 通用工具端点（E5，仅在出现第二消费方时推进）。
- 许可清单随包（`scripts/copy-licenses.sh`）与发布物装配（Solo 侧 W5）。
