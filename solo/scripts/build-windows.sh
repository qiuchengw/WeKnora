#!/usr/bin/env bash
# Solo 版 WeKnora Lite（cmd/server，Windows/amd64）构建脚本。
#
# 流程：应用 solo/patches/*.patch（剥离 Lite 不可达能力）→ 生成 sqlite3.h 构建输入头
#       → CGO 编译（EDITION=lite + sqlite_fts5 + 静态链接 MinGW 运行时）。
#
# 用法（在仓库根执行）：
#   bash solo/scripts/build-windows.sh [--out NAME] [--keep-symbols] [--no-patches]
#
# 工具链：优先 $MINGW_BIN（目录含 gcc.exe），否则要求 PATH 上有 gcc（CI 用 msys2 UCRT64）。
# 设计纪律：源码树零手改；补丁不能应用也不能回滚时**立即失败**，不做静默兜底。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"      # repo/solo
SRC="$(cd "$ROOT/.." && pwd)"                  # repo
OUT_NAME="WeKnora-lite.exe"
KEEP_SYMBOLS=0
APPLY_PATCHES=1

while [ $# -gt 0 ]; do
  case "$1" in
    --out) OUT_NAME="$2"; shift 2 ;;
    --keep-symbols) KEEP_SYMBOLS=1; shift ;;
    --no-patches) APPLY_PATCHES=0; shift ;;
    *) echo "未知参数：$1" >&2; exit 2 ;;
  esac
done

[ -n "${MINGW_BIN:-}" ] && export PATH="$MINGW_BIN:$PATH"
command -v gcc >/dev/null || { echo "错误：PATH 上没有 gcc（设置 MINGW_BIN 或安装 mingw-w64 UCRT）" >&2; exit 1; }
command -v go  >/dev/null || { echo "错误：PATH 上没有 go" >&2; exit 1; }

export CGO_ENABLED=1
export CC="${CC:-gcc}"
export CXX="${CXX:-g++}"
export EDITION=lite                 # 与上游 CI 一致；ldflags 注入 internal/handler.Edition
export GOPROXY="${GOPROXY:-https://proxy.golang.org,direct}"

# 1) 剥离补丁：能应用则应用；已应用则跳过；否则显式失败（上游已变，需刷新补丁）。
if [ "$APPLY_PATCHES" = 1 ]; then
  for p in "$ROOT"/patches/*.patch; do
    [ -e "$p" ] || continue
    name="$(basename "$p")"
    if git -C "$SRC" apply --check "$p" 2>/dev/null; then
      git -C "$SRC" apply "$p"
      echo ">> patch applied: $name"
    elif git -C "$SRC" apply --check -R "$p" 2>/dev/null; then
      echo ">> patch already applied: $name"
    else
      echo "错误：$name 既不能应用也不处于已应用状态——上游代码已变化，请刷新补丁（git apply --3way）" >&2
      exit 1
    fi
  done
fi

# 2) 构建输入头：sqlite-vec 的 cgo 绑定在 SQLITE_CORE 下 `#include "sqlite3.h"`，
#    而 mattn/go-sqlite3 只带 sqlite3-binding.h。把同一份 amalgamation 头复制为 sqlite3.h（不改源码树）。
SHIM_DIR="${SHIM_DIR:-$ROOT/.build-headers}"
mkdir -p "$SHIM_DIR"
MATTN_DIR="$(ls -d "$(go env GOMODCACHE)"/github.com/mattn/go-sqlite3@* 2>/dev/null | head -1)"
MATTN_DIR="${MATTN_DIR:-$(ls -d "$(go env GOMODCACHE | sed 's|\\|/|g; s|^\([A-Za-z]\):|/\L\1|')"/github.com/mattn/go-sqlite3@* 2>/dev/null | head -1)}"
[ -n "$MATTN_DIR" ] || { echo "错误：找不到 mattn/go-sqlite3 模块目录（先跑 go mod download）" >&2; exit 1; }
cp -f "$MATTN_DIR/sqlite3-binding.h" "$SHIM_DIR/sqlite3.h"
cp -f "$MATTN_DIR/sqlite3ext.h" "$SHIM_DIR/sqlite3ext.h"
# cgo 的 -I 需要 MSYS 能认的路径；msys2/Git Bash 下用 cygpath -m 更稳。
SHIM_INC="$SHIM_DIR"
command -v cygpath >/dev/null && SHIM_INC="$(cygpath -m "$SHIM_DIR")"
export CGO_CFLAGS="-Wno-deprecated-declarations -I$SHIM_INC"
export CGO_CXXFLAGS="$CGO_CFLAGS"

# 3) 编译
cd "$SRC"
eval "$("$SRC/scripts/get_version.sh" env)"
STRIP_FLAGS="-w -s"
[ "$KEEP_SYMBOLS" = 1 ] && STRIP_FLAGS="-w"
LDFLAGS="$STRIP_FLAGS -extldflags=-static $("$SRC/scripts/get_version.sh" ldflags)"

echo ">> build: $OUT_NAME  EDITION=$EDITION  VERSION=${VERSION:-unknown}  CC=$CC  ldflags='$STRIP_FLAGS'"
mkdir -p dist
go build -tags "sqlite_fts5" -ldflags="$LDFLAGS" -o "dist/$OUT_NAME" ./cmd/server
echo ">> done: $(ls -lh "dist/$OUT_NAME" | awk '{print $5, $9}')"
