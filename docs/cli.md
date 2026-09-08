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
panel-cli account import ./tenant.hpm-account.json moved moved.test
panel-cli account migrate <account-id> climig climig.test
panel-cli account sftp-password <account-id> 'SftpPass!2026'
panel-cli firewall apply
panel-cli mailbox create <account-id> <mail-domain-id> info 'MailboxPass!2026'
panel-cli db create <account-id> shop mariadb
panel-cli domain create <account-id> python.example.test python
panel-cli website create <account-id> <domain-id> node
panel-cli file write <account-id> /public_html/index.php '<?php echo "ok";'
panel-cli file list <account-id> /public_html
panel-cli backup restore <account-id> <backup-id>
panel-cli audit
panel-cli cert request <account-id> livehost.test
panel-cli reseller create 'Northwind' nwind 'ResellerPass!2026'
```
