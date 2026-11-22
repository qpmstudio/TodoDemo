#!/usr/bin/env bash
# PR diff 卫生检查：防止缓存/依赖目录、超大二进制、巨量文件混进改动。
# 由 .github/workflows/ci.yml 的 diff-hygiene job 调用，不进入 .flow/ci.json
# 的 required checks（只在 PR 里报红，不锁合并）。
set -uo pipefail

MAX_FILES=300
MAX_BINARY_BYTES=$((1024 * 1024))
BASE_REF="${GITHUB_BASE_REF:-main}"

fail=0
note() { printf '%s\n' "$*"; }
bad() { printf '❌ %s\n' "$*"; fail=1; }

if ! git rev-parse --verify --quiet "origin/${BASE_REF}" >/dev/null; then
  git fetch --no-tags --depth=1 origin "${BASE_REF}" >/dev/null 2>&1 || true
fi
if ! git rev-parse --verify --quiet "origin/${BASE_REF}" >/dev/null; then
  note "⚠️  找不到 origin/${BASE_REF}，跳过 diff 卫生检查"
  exit 0
fi

mapfile -t files < <(git diff --name-only --diff-filter=ACMR "origin/${BASE_REF}...HEAD")
count=${#files[@]}
note "改动文件数：${count}（上限 ${MAX_FILES}）"
if [ "$count" -gt "$MAX_FILES" ]; then
  bad "改动文件过多：${count} > ${MAX_FILES}"
fi

blocked_re='(^|/)\.cache/|\.gocache(/|$)|(^|/)node_modules/|(^|/)vendor/|(^|/)dist/'
for f in "${files[@]}"; do
  if [[ "$f" =~ $blocked_re ]]; then
    bad "改动里出现缓存/依赖目录文件：${f}"
  fi
done

for f in "${files[@]}"; do
  [ -f "$f" ] || continue
  size=$(wc -c <"$f" 2>/dev/null || echo 0)
  if [ "$size" -le "$MAX_BINARY_BYTES" ]; then continue; fi
  if ! grep -Iq . "$f" 2>/dev/null; then
    bad "超大二进制文件：${f}（${size} bytes > ${MAX_BINARY_BYTES}）"
  fi
done

if [ "$fail" -ne 0 ]; then
  note ""
  note "请把缓存/依赖/构建产物移出提交（.gitignore），大二进制改用外部存储。"
  exit 1
fi
note "✅ diff 卫生检查通过"
