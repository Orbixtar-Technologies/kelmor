# DNS

Each provisioned domain gets a zone file at `/var/lib/panel/dns/zones/<name>.zone` and default A/MX/SPF/DMARC records. PowerDNS is configured by the installer to serve bind-format zones from that directory, with the HTTP API bound to loopback only (`127.0.0.1:8081`).

Set `PANEL_PDNS_URL` and `PANEL_PDNS_API_KEY` on the worker to PATCH the live API after the zone file is written. Without those variables the file backend is still the source of truth for the agent.
