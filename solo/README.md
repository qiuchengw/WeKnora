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

### 上游同步记录（2026-09-18）

- 同步到上游 `aaec920a`（**80 commit / 1115 文件**），**文本合并零冲突**；但**编译不过**：
  上游把 `internal/agent/tools.DataAnalysisInput.Sql` 改名为 `SQL`，而车道 `internal/application/service/knowledge_data_query.go`
  仍在用旧字段名 ⇒ 同一变更集内跟随上游命名修好（JSON 键仍是 `sql`，对外契约不变）。
  **教训（写进纪律）**：**文本合并不冲突 ≠ 能编译**；每次同步必须跑 `go build ./...` + 关键包 `go test`，
  不能只看 `git merge` 的退出码。
- 上游自带 17 个失败用例（`internal/handler` 2 + `internal/application/service` 15），**在 pristine 上游 worktree 上同样失败**
  （Windows 执行位 / docker / python CLI / AES key / fork 脚本 / ZIP 权限等环境相关）⇒ 与本车道无关，按指纹登记、不逐个诊断。
- 车道净增（相对上游）：15 文件 / +861 行 —— E1 passage enrichment、`enable_rerank`、问数端点、DuckDB 离线优先、`content_revision` 投影。
  上游**已自行实现**「问数工具」与 `DUCKDB_SKIP_EXTENSION_LOAD` 开关，但**没有**我们的能力端点与投影 ⇒ 车道内容仍全部独有。
- **产物事实（2026-09-18 构建，本机 WSL 原生构建）**：`kb-engine-0.8.3-linux-x64.tar.gz`（80.5 MB / `sha512 b9ca4adf…2a1bc`），
  来源 = 本车道 HEAD `d21c72a4`（ldflags 内 `CommitID=d21c72a4` 可自证），二进制自报版本 **0.8.3**（与产物名一致）。
- **升级迁移演练（本地 WSL，客户升级路径）**：旧产物（15 个迁移）在空目录建库 ⇒ `schema_migrations.version=14`；
  换新产物（24 个迁移）在**同一数据目录**启动 ⇒ 自动迁移到 `version=23, dirty=0` 且 `/health` 200。
  ⇒ 引擎升级会自动迁移客户库（`AUTO_MIGRATE` 缺省开），**发布前必须跑这条演练**；客户侧无备份动作，见 Solo 侧提案"升级前自动备份引擎库"。

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

## 本仓的 canonical 检出位置（2026-09-18 起）

**canonical = Solo 主仓的子模块 `solo/packages/kb-engine-src`**（方案 A，拍板 2026-09-18）：

- 为什么：产物必须能追溯到**确切的源码提交**（ADR-0019 D-13 的 provenance 要求）。子模块的 gitlink 把它变成
  仓库事实（`git rev-parse HEAD` = 产物里的 `CommitID`），并且**消掉了发版脚本里写死的本机绝对路径**；
- 远端约定（子模块内）：`origin` = fork（`git@github.com:qiuchengw/WeKnora.git`，推 `solo` 分支）；
  `upstream` = `https://github.com/Tencent/WeKnora.git`（只拉上游）；
- **两级提交**（引擎改动必须走完，否则主仓钉的还是旧提交）：
  ```bash
  cd <solo>/packages/kb-engine-src
  git checkout solo && git merge upstream/main      # 需要同步上游时（先 git fetch upstream --tags）
  go build ./... && go test ./internal/...          # 文本合并不冲突 ≠ 能编译（2026-09-18 实测）
  git commit … && git push origin solo              # ① 车道提交推到 fork
  cd <solo> && git add packages/kb-engine-src && git commit …   # ② 主仓提交新 gitlink
  ```
- 发版脚本（`solo/scripts/lib/release-kbengine.mjs`）缺省就用这个子模块，并在构建前**断言 gitlink 工作树干净**；
  `--src` / `SOLO_KB_ENGINE_SRC` 仍可指到别处的检出（离线预检/对照构建）。
- 原来的旁路检出（`E:\code\900bu\_thirdparty\weknora`）自此只是历史副本，不再被任何脚本使用。

工具链发现顺序：`MINGW_BIN` 环境变量 → `PATH` 上的 `gcc`。CI 用 msys2 `UCRT64`。

产物纪律：每个发布物记录 **上游基线 SHA + 本车道分支 SHA + 构建命令 + SHA256 + 许可清单**。

## 上游同步流程（上游出新 tag/推进 main 时）

```bash
git fetch origin --tags                 # origin = 上游 Tencent/WeKnora
git fetch origin '+refs/heads/main:refs/remotes/origin/main'   # 本仓 origin 只配了 tag refspec ⇒ main 要显式抓
git checkout solo
git merge origin/main                   # 冲突一次解决（这就是退役 patch 车道的主要收益）
go build ./... && go test ./internal/... # 车道自检（含新增端点/健壮性用例）
git push fork-ssh solo
```

- **必跑编译再判定同步成功**（2026-09-18 实测：80 commit 合并零冲突，却因上游字段改名编译失败）。
- 冲突解决原则：**行为对齐上游**优先；我们的能力端点保持附加式（不改变上游既有语义）。
- 每次同步后重跑约定：`go build ./...`、车道单测（`TestApplyRerank*` / `TestData*` / 投影 4 例）、`router` 包自检、产物冒烟（`EDITION=lite` 出活 + `/health`）。
- **版本口径**：产物/平台版本（x.y.z，`--engine-version`）与二进制自报版本由构建脚本**强制对齐**（`KB_ENGINE_VERSION` → 追加一条
  同符号 `-X` 覆盖上游 `get_version.sh` 的 `VERSION` 文件值），构建末尾自带 `strings | grep -qx` 自证；上游 `VERSION` 文件保持不动（避免每次同步冲突）。
- 上游接受对应 PR 后：rebase/merge 会把这些 commit 自然吞掉（becomes empty），届时从本车道删除即可。

## 已知待办（能力端点化路线）

目标形态：**宿主 Agent（唯一编排） + 引擎（无 agent 的纯能力面）**。

- 已就绪：`hybrid-search`（召回）+ `enable_rerank`（重排）；**问数端点** `POST /knowledge/:id/data-analysis` + `GET /knowledge/:id/data-schema`（DuckDB 执行与表格解析均在引擎）。
- 宿主侧：`KbEnginePort` 收口端点；工具注册走已有 `agent.mastra.tool` 机制（`kb_search` / `kb_data_schema` / `kb_data_query`）；MCP 作为后续可选薄壳。宿主 Agent 只做“问题→只读 SQL”的规划与结果解释。
- 待定：`POST /engine-tools/{tool}` 通用工具端点（E5，仅在出现第二消费方时推进）。
- 许可清单随包（`scripts/copy-licenses.sh`）与发布物装配（Solo 侧 W5）。
