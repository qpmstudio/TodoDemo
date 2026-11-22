# .flow

仓库级流水线配置，命令由仓库自己声明，CI 不猜语言。

- `check.sh`：统一检查入口。`bash .flow/check.sh [go|frontend|all]`（默认 all），本地与 CI 共用。
- `ci.json`：PR 门禁声明。`runner` 为 runner 标签，`checks[].name` 即 required check 名，`run` 为命令。
- `.github/workflows/ci.yml`：由 `ci_init.py` 按 `ci.json` 生成；`setup-go` / `setup-node` 为裸 self-hosted runner 手动追加。
- `deploy.json`：部署槽位（`deploy_<env>`）的 url / namespace / deployment / image_repo。
- `.github/workflows/deploy.yml`：只接受 `deploy_<env>@<yyyymmDDHHMMSS>` 标签触发。
