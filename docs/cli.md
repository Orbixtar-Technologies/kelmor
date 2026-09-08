# CLI

`panel-cli` talks to the control API. It never applies host configuration itself.

```bash
export PANEL_API=http://127.0.0.1:18080
eval $(panel-cli login admin 'ChangeMeOnce!2026')
panel-cli account list
panel-cli account create tenant1 tenant1.example <package-id> 'TenantPass!2026'
panel-cli jobs list
panel-cli backup create <account-id>
panel-cli account export <account-id>
panel-cli audit
```
