package store

import "fmt"

func (m *Memory) ImportAccountWithJob(imported *AccountImport, job *Job) (*Job, error) {
	return m.importAccountWithJob(imported, job, nil)
}

func (m *Memory) ImportAccountWithJobAndAudit(imported *AccountImport, job *Job, audit AuditEvent) (*Job, error) {
	return m.importAccountWithJob(imported, job, &audit)
}

func (m *Memory) importAccountWithJob(imported *AccountImport, job *Job, audit *AuditEvent) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if imported == nil || job == nil {
		return nil, fmt.Errorf("import and job are required")
	}
	account := &imported.Account
	if account.ID == "" || account.OwnerUserID == "" || m.Users[account.OwnerUserID] == nil {
		return nil, fmt.Errorf("import account owner is missing")
	}
	if m.Packages[account.PackageID] == nil {
		return nil, fmt.Errorf("package %q not found", account.PackageID)
	}
	if m.Accounts[account.ID] != nil {
		return nil, fmt.Errorf("account %q already exists", account.ID)
	}
	for _, existing := range m.Accounts {
		if existing.Username == account.Username {
			return nil, fmt.Errorf("account username %q already exists", account.Username)
		}
	}
	if job.IdempotencyKey != "" {
		for _, existing := range m.Jobs {
			if existing.IdempotencyKey == job.IdempotencyKey {
				cp := *existing
				return &cp, nil
			}
		}
	}

	domains := make(map[string]Domain, len(imported.Domains))
	for _, domain := range imported.Domains {
		if domain.ID == "" || domain.AccountID != account.ID || m.Domains[domain.ID] != nil {
			return nil, fmt.Errorf("invalid imported domain %q", domain.ID)
		}
		for _, existing := range m.Domains {
			if domain.ASCII != "" && existing.ASCII == domain.ASCII {
				return nil, fmt.Errorf("domain %q already exists", domain.ASCII)
			}
		}
		if _, duplicate := domains[domain.ID]; duplicate {
			return nil, fmt.Errorf("duplicate imported domain %q", domain.ID)
		}
		domains[domain.ID] = domain
	}
	websites := make(map[string]Website, len(imported.Websites))
	for _, website := range imported.Websites {
		domain, ok := domains[website.DomainID]
		if website.ID == "" || website.AccountID != account.ID || !ok || domain.AccountID != account.ID ||
			m.Websites[website.ID] != nil {
			return nil, fmt.Errorf("invalid imported website %q", website.ID)
		}
		if _, duplicate := websites[website.ID]; duplicate {
			return nil, fmt.Errorf("duplicate imported website %q", website.ID)
		}
		websites[website.ID] = website
	}
	for _, application := range imported.Applications {
		website, ok := websites[application.WebsiteID]
		if application.ID == "" || application.AccountID != account.ID || !ok ||
			website.AccountID != account.ID || m.Apps[application.ID] != nil {
			return nil, fmt.Errorf("invalid imported application %q", application.ID)
		}
	}
	for _, database := range imported.Databases {
		if database.ID == "" || database.AccountID != account.ID || m.DBs[database.ID] != nil {
			return nil, fmt.Errorf("invalid imported database %q", database.ID)
		}
	}
	for _, user := range imported.DatabaseUsers {
		if user.ID == "" || user.AccountID != account.ID || m.DBUsers[user.ID] != nil {
			return nil, fmt.Errorf("invalid imported database user %q", user.ID)
		}
	}
	mailDomains := make(map[string]MailDomain, len(imported.MailDomains))
	for _, domain := range imported.MailDomains {
		ownedDomain, ok := domains[domain.DomainID]
		if domain.ID == "" || domain.AccountID != account.ID || !ok ||
			ownedDomain.AccountID != account.ID || m.MailDom[domain.ID] != nil {
			return nil, fmt.Errorf("invalid imported mail domain %q", domain.ID)
		}
		mailDomains[domain.ID] = domain
	}
	for _, mailbox := range imported.Mailboxes {
		domain, ok := mailDomains[mailbox.DomainID]
		if mailbox.ID == "" || mailbox.AccountID != account.ID || !ok ||
			domain.AccountID != account.ID || m.Mailboxes[mailbox.ID] != nil {
			return nil, fmt.Errorf("invalid imported mailbox %q", mailbox.ID)
		}
	}
	for _, alias := range imported.Aliases {
		domain, ok := mailDomains[alias.DomainID]
		if alias.ID == "" || alias.AccountID != account.ID || !ok ||
			domain.AccountID != account.ID || m.Aliases[alias.ID] != nil {
			return nil, fmt.Errorf("invalid imported mail alias %q", alias.ID)
		}
	}
	zones := make(map[string]DNSZone, len(imported.Zones))
	for _, zone := range imported.Zones {
		domain, ok := domains[zone.DomainID]
		if zone.ID == "" || zone.AccountID != account.ID || !ok ||
			domain.AccountID != account.ID || m.Zones[zone.ID] != nil {
			return nil, fmt.Errorf("invalid imported DNS zone %q", zone.ID)
		}
		zones[zone.ID] = zone
	}
	for _, record := range imported.Records {
		if record.ID == "" || m.Records[record.ID] != nil {
			return nil, fmt.Errorf("invalid imported DNS record %q", record.ID)
		}
		if _, ok := zones[record.ZoneID]; !ok {
			return nil, fmt.Errorf("DNS record %q references missing zone", record.ID)
		}
	}
	for _, cron := range imported.Crons {
		if cron.ID == "" || cron.AccountID != account.ID || m.Crons[cron.ID] != nil {
			return nil, fmt.Errorf("invalid imported cron %q", cron.ID)
		}
	}
	for _, key := range imported.SSH {
		if key.ID == "" || key.AccountID != account.ID || m.SSHKeys[key.ID] != nil {
			return nil, fmt.Errorf("invalid imported SSH key %q", key.ID)
		}
	}
	for _, ftp := range imported.FTP {
		if ftp.ID == "" || ftp.AccountID != account.ID || m.FTPs[ftp.ID] != nil {
			return nil, fmt.Errorf("invalid imported FTP account %q", ftp.ID)
		}
		for _, existing := range m.FTPs {
			if existing.Username == ftp.Username {
				return nil, fmt.Errorf("FTP username %q already exists", ftp.Username)
			}
		}
	}
	if job.TargetRevision == 0 {
		job.TargetRevision = account.DesiredRevision
	}
	normalizeJob(job)
	if err := m.validateNewJobLocked(job); err != nil {
		return nil, err
	}
	if account.LinuxUID < 20000 {
		account.LinuxUID = m.NextUID
		account.LinuxGID = m.NextUID
		m.NextUID++
	}

	accountCopy := *account
	m.Accounts[account.ID] = &accountCopy
	for id, value := range domains {
		copy := value
		m.Domains[id] = &copy
	}
	for id, value := range websites {
		copy := value
		m.Websites[id] = &copy
	}
	for i := range imported.Applications {
		value := imported.Applications[i]
		m.Apps[value.ID] = &value
	}
	for i := range imported.Databases {
		value := imported.Databases[i]
		m.DBs[value.ID] = &value
	}
	for i := range imported.DatabaseUsers {
		value := imported.DatabaseUsers[i]
		m.DBUsers[value.ID] = &value
	}
	for id, value := range mailDomains {
		copy := value
		m.MailDom[id] = &copy
	}
	for i := range imported.Mailboxes {
		value := imported.Mailboxes[i]
		m.Mailboxes[value.ID] = &value
	}
	for i := range imported.Aliases {
		value := imported.Aliases[i]
		m.Aliases[value.ID] = &value
	}
	for id, value := range zones {
		copy := value
		m.Zones[id] = &copy
	}
	for i := range imported.Records {
		value := imported.Records[i]
		m.Records[value.ID] = &value
	}
	for i := range imported.Crons {
		value := imported.Crons[i]
		m.Crons[value.ID] = &value
	}
	for i := range imported.SSH {
		value := imported.SSH[i]
		m.SSHKeys[value.ID] = &value
	}
	for i := range imported.FTP {
		value := imported.FTP[i]
		m.FTPs[value.ID] = &value
	}
	jobCopy := *job
	m.Jobs[job.ID] = &jobCopy
	if audit != nil {
		normalizeAudit(audit, job)
		auditCopy := *audit
		m.Audit = append(m.Audit, &auditCopy)
	}
	return &jobCopy, nil
}
