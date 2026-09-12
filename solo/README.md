# `solo/` —— WeKnora 下游车道（Solo 知识库引擎）

本目录是 **Solo 产品线对上游 WeKnora 的唯一增量**。上游仓库/分支**只读**：我们不推上游分支、不改上游源文件；
所有差异以 **补丁 + 构建脚本 + CI** 的形式存放于此，构建时自动应用（`git apply`）。

- 上游基线：`Tencent/WeKnora` tag **`v0.8.0`**（远端 `origin`；fork：`qiuchengw/WeKnora`，远端 `fork`，车道分支：`solo`）
- 决策依据：`solo` 仓库 `docs/adr/0019-kb-engine-externalization.md`（D-13 构建来源纪律、D-15 下游车道）
- 形态：**单进程无头服务**（`cmd/server`，`EDITION=lite`），由 Solo 宿主 spawn 一个子进程，走 `127.0.0.1 + token`。

## 为什么需要本车道

上游 Lite 目标在 Linux 上可零改码构建；Windows 上**零改码不可行**：

| 事实 | 后果 |
|---|---|
| 上游无 `windows/amd64` 资产，`release-lite.yml` 仅手动触发 | 必须自建 |
| `sqlite-vec` cgo 在 `SQLITE_CORE` 下 `#include "sqlite3.h"`，而 mattn/go-sqlite3 只带 `sqlite3-binding.h` | 需构建输入头（脚本生成，不进源码树） |
| DuckDB 预编译静态库是 GCC/libstdc++ ABI | 必须用 GCC 系工具链（msys2 UCRT64 / WinLibs） |

加之 Lite 形态下大量能力**不可达**（云向量库、IM 通道、沙箱、内置 Agent、Wails 桌面壳），
保留它们只带来体积与链接负担。故采取**脚本化剥离**：不叠过渡层、不上 feature flag，直接删除不可达能力（SSOT 纪律）。

## 目录

```
solo/
  README.md                   本文件（车道说明 + 升级流程）
  patches/0001-strip-stage1.patch   阶段 1 剥离（DuckDB / 数据分析工具 / 表格摘要服务）
  scripts/build-windows.sh    Windows/amd64 构建（自动应用补丁 → 生成 sqlite3.h → 编译）
.github/workflows/solo-lite-windows.yml   CI：windows-latest 出活 + SHA256
```

## 构建

```bash
# 本机（Windows，Git Bash 或 msys2）：
bash solo/scripts/build-windows.sh                 # 产物 dist/WeKnora-lite.exe
bash solo/scripts/build-windows.sh --out Foo.exe --keep-symbols   # 体积画像用（保留符号表）
```

工具链发现顺序：`MINGW_BIN` 环境变量 → `PATH` 上的 `gcc`。CI 用 msys2 `UCRT64`。

产物纪律：每个发布物记录 **上游 tag + 本车道 commit + 补丁清单 + 构建命令 + SHA256 + 许可清单**。

## 剥离清单（阶段 1，2026-09-12）

删除（15 文件 / −2259 行 / +2 行）：

| 移除项 | 原作用 |
|---|---|
| `internal/agent/tools/data_analysis*.go` + `chat_pipeline/data_analysis.go` | 内置 Agent 的「数据分析」工具：CSV/Excel 载入内存 DuckDB，把问题转 SQL 只读执行；以及问答链路上的表格分析编排 |
| `internal/application/service/extract.go` 两个任务（表格摘要） | 后台任务：为表格类附件生成 LLM 摘要并索引，供语义检索命中 |
| `cmd/download/duckdb/` | 运维工具：运行时联网 INSTALL DuckDB 扩展（spatial/excel） |
| `container.go` / `agent_service.go` / 两个 router / 两处入队调用点 | 上述能力的装配、工具注册、任务注册与触发 |

**保留**：`ToolDataSchema`（表格元信息工具，不依赖 DuckDB）、解析/分块/嵌入/检索主链。

## 升级流程（上游出新 tag 时）

```bash
git fetch origin --tags                    # origin = 上游 Tencent/WeKnora
git checkout -b solo-v0.9 origin/v0.9       # 基线：新 tag
git checkout solo -- solo/ .github/workflows/solo-lite-windows.yml
bash solo/scripts/build-windows.sh          # 补丁自动应用；冲突则显式报错
```

- 补丁主体是**整文件删除**（几乎不会冲突）；少量改动集中在几处调用点（容器装配、工具/任务注册、入队调用）。
- 若 `git apply --check` 失败，脚本**立即报错退出**（不允许静默兜底）：用 `git apply --3way` 刷新补丁，或调整基线 tag。
- 长期更优解：向上游提 PR 把这些能力改为 build tag（可选编译）；在此之前本车道是 canonical 构建路径。

## 已知待办（后续阶段）

- **阶段 2 剥离**（W1/W5 之间）：云向量库驱动（milvus / weaviate / qdrant / elasticsearch / pgvector）、IM 通道（lark/slack 等）、沙箱（e2b）、内置 Agent 与 MCP/CLI、Wails 桌面壳、`swaggo` Swagger UI 资源等 —— 目标：二进制体积与常驻内存显著下降。
- 许可清单随包（`scripts/copy-licenses.sh`）与发布物装配（Solo 侧 W5）。
