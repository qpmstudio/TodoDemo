#!/usr/bin/env bash
# 仓库统一检查入口：本地 pre-push 与 PR CI 都调用这里。
# 用法: .flow/check.sh [go|frontend|all]  （默认 all）
set -euo pipefail
cd "$(dirname "$0")/.."

check_go() {
  echo "== go vet =="
  go vet ./...
  echo "== go test =="
  go test ./... -count=1
}

check_frontend() {
  cd frontend
  if [ ! -d node_modules ]; then
    echo "== npm ci =="
    npm ci
  fi
  echo "== frontend lint =="
  npm run lint
  echo "== frontend test =="
  npm run test
  echo "== frontend build =="
  npm run build
}

case "${1:-all}" in
  go) check_go ;;
  frontend) check_frontend ;;
  all) check_go; check_frontend ;;
  *) echo "未知检查项: ${1}（可用: go frontend all）" >&2; exit 2 ;;
esac
