# Application deployment

Director's Applications tab lets you deploy an existing Node.js or Python application to a hosting account. The agent configures a systemd service unit and checks that it activates — no source generation, no package installation, no file changes in the account.

## Upload-first workflow

The agent never writes to the account's document root. Upload code before pressing **Detect and deploy**:

1. Upload your code to the website's document root (SSH/SFTP, File Manager, or git).
2. Open the account's **Services → Applications** tab in Director.
3. Select the target website (only enabled Node.js or Python websites appear).
4. Optionally expand **Advanced options** to supply an explicit start command.
5. Click **Detect and deploy**. The operation is queued as a Job.

## How detection works

When no start command is provided the agent probes the document root through the managed filesystem (symlinks cannot escape the account):

| Priority | Node.js | Python |
|----------|---------|--------|
| 1 | `package.json` with a non-empty `scripts.start` entry → `/usr/bin/npm start` | `app.py` present → `/usr/bin/python3 app.py` |
| 2 | `server.js` present → `/usr/bin/node server.js` | — |

If none of the expected files are found the job fails with a `requirements:` message describing what is missing. Retry after uploading the missing file.

A custom start command must begin with an absolute executable path (e.g. `/usr/bin/gunicorn`). Relative paths and systemd specifiers (`%`) are rejected.

## SOCKET_PATH requirement

Your application **must** listen on the Unix socket path provided in the `SOCKET_PATH` environment variable:

```
SOCKET_PATH=/run/panel/apps/<website-id>.sock
```

The Nginx proxy connects to this socket. If your application binds a TCP port instead the proxy cannot reach it. This increment does not verify socket connectivity — it only checks `systemctl is-active`.

## Unit naming convention

The systemd service unit is written to:

```
/etc/systemd/system/panel-app-<website-id>.service
```

The unit runs as the hosting account user, is placed in the account's systemd slice, and restarts on failure with a limit of 3 starts per 60 seconds (`StartLimitBurst=3`, `StartLimitIntervalSec=60`, `RestartSec=5`).

## Checking status and logs

After deploy, track progress in the **Jobs** panel. The job status reflects what the agent observed:

- `running` — `systemctl is-active` confirmed the service is active.
- `configured` — sandbox run; execution was not verified.
- `failed` — the service did not become active. Check the journal:

```
journalctl -u panel-app-<website-id>.service -n 100
```

Job errors include an actionable `requirements:` or `deployment:` prefix. Correct the cause and retry the job.

## Future roadmap

The following are not implemented and remain future work:

- **Dependency installation** — `npm install`, `pip install`, or equivalent; tenant-run scripts need a sandboxed executor distinct from the host module installer.
- **Build steps** — TypeScript compilation, asset bundling, static generation.
- **Environment secrets** — encrypted per-deployment secret storage and injection.
- **Private repository credentials** — git clone with SSH or token auth.
- **Runtime version selection** — enforcement of the runtime version metadata already stored on the website record.
- **Rollback** — snapshotting the previous unit and restoring on demand.
- **Health probes** — HTTP readiness checks before switching traffic.
- **TCP port adaptation** — automatic reverse-proxy configuration for apps that cannot use a Unix socket.
