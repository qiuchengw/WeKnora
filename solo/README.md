# `solo/` —— WeKnora 下游车道（Solo 知识库引擎）

本目录是 **Solo 产品线对上游 WeKnora 的唯一增量**，且**只做构建、不改码**：上游源文件**零改动**（补丁仅在显式 `--with-patches` 时可选启用），所有差异以 **构建脚本 + CI** 形式存放于此。

- 上游基线：`Tencent/WeKnora` tag **`v0.8.0`**（远端 `origin`；fork：`qiuchengw/WeKnora`，远端 `fork`，车道分支：`solo`）
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
  patches/0001-strip-stage1.patch   【可选，默认不启用】阶段 1 剥离（DuckDB / 数据分析工具 / 表格摘要服务）
  scripts/build-windows.sh    Windows/amd64 构建（默认零补丁；生成 sqlite3.h → 编译）
.github/workflows/solo-lite-windows.yml   CI：windows-latest 出活 + SHA256
```

## 构建

```bash
# 本机（Windows，Git Bash 或 msys2）：
bash solo/scripts/build-windows.sh                 # 零补丁（canonical）→ dist/WeKnora-lite.exe
bash solo/scripts/build-windows.sh --keep-symbols  # 体积画像（保留符号表）
bash solo/scripts/build-windows.sh --with-patches   # 可选：出剥离版（体积极敏场景）
```

工具链发现顺序：`MINGW_BIN` 环境变量 → `PATH` 上的 `gcc`。CI 用 msys2 `UCRT64`。

产物纪律：每个发布物记录 **上游 tag + 本车道 commit + 是否启用补丁 + 构建命令 + SHA256 + 许可清单**。

## 剥离补丁（可选，默认不启用）

`patches/0001-strip-stage1.patch` = 18 文件 / −2472 行（DuckDB + 数据分析工具/管道 + 表格摘要任务与入队点 + 悬空 `data_schema`）。
**启用会影响能力面**：`data_analysis`/`data_schema` 被删 ⇒ **问数不可用**（ADR-0019 D-16）。仅当体积/内存成为发布阻塞时用 `--with-patches`。

## 升级流程（上游出新 tag 时）——零补丁、零人工

```bash
git fetch origin --tags                    # origin = 上游 Tencent/WeKnora
git checkout -b solo-v0.9 origin/v0.9       # 基线：新 tag
git checkout solo -- solo/ .github/workflows/solo-lite-windows.yml
bash solo/scripts/build-windows.sh          # 直接构建（默认零补丁），无需刷任何补丁
```

- 默认路径**不碰源码** ⇒ 不存在补丁冲突；CI 红/绿即可判定新 tag 可用性。
- 若启用 `--with-patches`：补丁主体是整文件删除（极少冲突），少量 hunk 在调用点；`git apply --check` 失败时脚本**立即报错退出**（不静默兜底），用 `git apply --3way` 刷新补丁。
- 真要系统性瘦身：**向上游提 build tag PR**（把不可达能力（云向量库/IM/沙箱/内置 Agent 面）改为可选编译）——一次性、零本地维护。

## 已知待办（后续阶段）

- **体积极化（仅当成为阻塞）**：先试上游 build tag PR；剥离补丁只是应急开关。
- **问数（ADR-0019 D-16）**：宿主 Agent 委派引擎 `agent-chat` + 内置 “Data Analyst” 预设；已知缺口：xlsx 依赖 DuckDB `excel` 扩展（运行时联网 INSTALL），CSV 离线可用。
- 许可清单随包（`scripts/copy-licenses.sh`）与发布物装配（Solo 侧 W5）。
