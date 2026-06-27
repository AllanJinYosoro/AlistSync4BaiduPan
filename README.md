# bdp-sync

A backup/sync CLI for sending local folders to Baidu Netdisk through AList WebDAV and rclone.

This project does not run in the background, watch files, or schedule automatic jobs. Nothing is uploaded or deleted until you run a command.

## Windows GUI

Run `bdp-sync.exe` without command-line arguments to open the desktop window. The GUI reads `config.yaml`, lets you choose one configured task or all tasks, and shows command output in the log area.

The window has three tabs:

- `Sync` keeps the existing task selector and `Doctor`, `Dry run`, `Update`, and `Sync` actions.
- `Config` lets you edit common `config.yaml` fields through a form or edit the full YAML directly. Saves are validated before the file is overwritten.
- `Dependencies` shows the detected `rclone` and `alist` paths, can install missing tools into `.alist-sync/tools`, and can copy a manually selected executable into the local tools directory.

On startup, the GUI checks for `rclone` and `alist`. If either dependency is missing, it asks before downloading anything.

Build a local Windows GUI executable during development:

```powershell
go build -ldflags "-H=windowsgui" -o bdp-sync.exe ./cmd/bdp-sync
```

That build does not open a visible `cmd`/PowerShell window when launched from Explorer, and closing the terminal that built or launched it does not close the GUI. If you need a console-oriented CLI build with normal terminal output, build a separate executable:

```powershell
go build -o bdp-sync-cli.exe ./cmd/bdp-sync
```

For a packaged Fyne app, install the Fyne CLI and package for Windows:

```powershell
go install fyne.io/tools/cmd/fyne@latest
fyne package -os windows
```

## Project Layout

Source code is organized by responsibility:

- `cmd/bdp-sync` contains the executable entrypoint.
- `internal/config` owns YAML parsing, defaults, validation, and saving.
- `internal/deps`, `internal/alist`, `internal/rclone`, and `internal/filename` own reusable tool, service, transfer, and preflight checks.
- `internal/runner` implements the CLI command flow.
- `internal/gui` implements the Fyne desktop UI.

Runtime state stays outside source packages: `.alist-sync/` stores local tools and rclone config, while `data/` is AList runtime data.

## Commands

Passing any command-line argument keeps the CLI behavior:

```powershell
bdp-sync init
bdp-sync setup deps
bdp-sync doctor
bdp-sync dry-run documents
bdp-sync update documents
bdp-sync sync documents
```
- `init` creates a default `config.yaml` template in the project directory, so you can run through the setup flow before syncing.
- `setup deps` checks and installs dependencies needed by this toolchain (including `rclone` and `alist` helpers).
- `doctor` validates your current environment and config, verifying credentials and remote connectivity before any data operation.
- `dry-run` runs `rclone sync --dry-run --combined -` and previews what a full sync would change.
- `update` runs `rclone copy`, uploading new or changed local files without deleting remote-only files.
- `sync` runs `rclone sync`, making the remote match local contents. It can delete remote-only files.

`documents` is the task name from `tasks[].name` in `config.yaml`. You can use another task name from your config, or pass `--all` to run every configured task.

## Configuration

Run `bdp-sync init` to create `config.yaml`:

```yaml
alist:
  url: "http://127.0.0.1:5244"
  username: "admin"
  password_env: "ALIST_PASSWORD"
  server_command: ".alist-sync/tools/alist.exe server"
  startup_timeout_seconds: 30

rclone:
  remote: "alist_baidu"
  config_file: ".alist-sync/rclone.conf"
  transfers: 4
  checkers: 8
  excludes:
    - "**/.git/**"

tasks:
  - name: "documents"
    local: "D:/Documents"
    remote: "/BaiduPanBackup/Documents"
    excludes:
      - "private/**"
```

Set the AList password in an environment variable instead of committing it:

```powershell
$env:ALIST_PASSWORD = "your_alist_password"
```

Or put it in the local `.env` file:

```text
ALIST_PASSWORD=your_alist_password
```

`password_env` names the environment variable that stores the AList WebDAV user's password. It is not your Baidu Netdisk password. The `admin` AList user can still be used. Data commands automatically load `.env` when present and use this password to write or refresh the rclone WebDAV credential.

`server_command` is optional. When `dry-run`, `sync`, or `update` starts, the CLI checks `alist.url`; if it is unreachable and `server_command` is configured, it starts AList with that command and waits up to `startup_timeout_seconds` for the service to become reachable. `setup deps` only installs/reuses dependencies and never starts AList.

`rclone.excludes` applies to every task. `tasks[].excludes` applies only to that task, and is appended to the global excludes when building rclone `--exclude` filters.

## Setup Notes

1. Run `bdp-sync setup deps` to reuse or download `rclone` and `alist`.
2. In AList, mount Baidu Netdisk with your own Baidu account.
3. Enable WebDAV permissions for the AList user used by this CLI.
4. Set the environment variable named by `alist.password_env`.
5. Run `bdp-sync doctor`.
6. Run `bdp-sync update documents` or `bdp-sync sync documents`.

The `.alist-sync/rclone.conf` file is maintained automatically by `dry-run`, `update`, and `sync`.

## Installing AList

This CLI needs a configured AList data directory and storage. `bdp-sync setup deps` can download an `alist` binary, but it does not configure your AList storage and does not start AList during dependency setup.

### Windows manual install

1. Download the latest Windows asset from [AList releases](https://github.com/AlistGo/alist/releases/latest), usually `alist-windows-amd64.zip` for a 64-bit Windows PC.
2. Unzip it to a stable folder such as `C:\alist`.
3. Start AList:

```powershell
cd C:\alist
.\alist.exe server
```

4. Open `http://127.0.0.1:5244` and log in as `admin`.
5. For AList v3.25.0 and newer, set or regenerate the admin password if needed:

```powershell
.\alist.exe admin set NEW_PASSWORD
.\alist.exe admin random
```

After AList is configured once, set `alist.server_command` to the command that starts it, for example:

```yaml
alist:
  server_command: "C:/alist/alist.exe server"
```

If you rely on the binary downloaded by `setup deps`, use:

```yaml
alist:
  server_command: ".alist-sync/tools/alist.exe server"
```

### Docker install

If you use Docker, run AList with a persistent data volume:

```powershell
docker run -d --restart=unless-stopped `
  -v C:\alist\data:/opt/alist/data `
  -p 5244:5244 `
  -e PUID=0 -e PGID=0 -e UMASK=022 `
  --name alist xhofe/alist:latest
```

Then set or regenerate the admin password:

```powershell
docker exec -it alist ./alist admin set NEW_PASSWORD
docker exec -it alist ./alist admin random
```

After AList is running, add Baidu Netdisk as a storage in the AList web UI, create or choose an AList user with WebDAV access, and copy that user's password into the `ALIST_PASSWORD` environment variable used by this project.

## Baidu VIP/SVIP

AList uses the Baidu account you configured in its Baidu Netdisk storage. rclone only talks to AList through WebDAV, so any membership benefit must come through AList's Baidu driver and Baidu's official/open upload behavior.

Do not use crack APIs or limit-bypass tools. This project only uses AList WebDAV plus rclone.

References:

- [AList manual installation](https://alistgo.com/guide/install/manual.html)
- [AList Docker installation](https://alistgo.com/guide/install/docker.html)
- [AList Baidu Netdisk driver](https://alistgo.com/guide/drivers/baidu.html)
- [AList WebDAV](https://alistgo.com/guide/webdav.html)
- [rclone sync](https://rclone.org/commands/rclone_sync/)
- [rclone copy](https://rclone.org/commands/rclone_copy/)



