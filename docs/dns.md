# DNS

Each provisioned domain gets a zone file at `/var/lib/panel/dns/zones/<name>.zone` and default A/MX/SPF/DMARC records. PowerDNS is configured by the installer to serve bind-format zones from that directory, with the HTTP API bound to loopback only (`127.0.0.1:8081`).

Set `PANEL_PDNS_URL` and `PANEL_PDNS_API_KEY` on the worker to PATCH the live API after the zone file is written. Without those variables the file backend is still the source of truth for the agent.

DNSSEC uses the PowerDNS bind backend database at `/var/lib/panel/dns/bind-dnssec.sqlite3` (`bind-dnssec-db` in `pdns.conf`). Enabling DNSSEC on a zone queues `dns.dnssec`; the privileged agent runs `pdnsutil secure-zone` / `disable-dnssec` and `export-zone-ds`. The Account Portal and `GET /dns/zones/{id}/ds` show the DS records to paste at the registrar. The HTTP cryptokeys API is not used — it does not work for bind-format zones.
