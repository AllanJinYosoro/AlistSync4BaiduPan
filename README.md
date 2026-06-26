# AListSync4BaiduPan

A manual backup/sync CLI for sending local folders to Baidu Netdisk through AList WebDAV and rclone.

This project does not run in the background, watch files, or schedule automatic jobs. Nothing is uploaded or deleted until you run a command.

## Commands

```powershell
alist-sync init
alist-sync setup deps
alist-sync setup rclone
alist-sync doctor
alist-sync dry-run documents
alist-sync update documents
alist-sync sync documents --yes
```

- `dry-run` runs `rclone sync --dry-run --combined -` and previews what a full sync would change.
- `update` runs `rclone copy`, uploading new or changed local files without deleting remote-only files.
- `sync` runs `rclone sync`, making the remote match local contents. It can delete remote-only files, so it requires `--yes`.

## Configuration

Run `alist-sync init` to create `config.yaml`:

```yaml
alist:
  url: "http://127.0.0.1:5244"
  username: "admin"
  password_env: "ALIST_PASSWORD"

rclone:
  remote: "alist_baidu"
  config_file: ".alist-sync/rclone.conf"
  transfers: 4
  checkers: 8

tasks:
  - name: "documents"
    local: "D:/Documents"
    remote: "/百度网盘备份/Documents"
```

Set the AList password in an environment variable instead of committing it:

```powershell
$env:ALIST_PASSWORD = "your_alist_password"
```

## Setup Notes

1. Install or run AList.
2. In AList, mount Baidu Netdisk with your own Baidu account.
3. Enable WebDAV permissions for the AList user used by this CLI.
4. Run `alist-sync setup deps` to reuse or download `rclone` and `alist`.
5. Run `alist-sync setup rclone` to write `.alist-sync/rclone.conf`.
6. Run `alist-sync doctor`.

## Baidu VIP/SVIP

AList uses the Baidu account you configured in its Baidu Netdisk storage. rclone only talks to AList through WebDAV, so any membership benefit must come through AList's Baidu driver and Baidu's official/open upload behavior.

Do not use crack APIs or limit-bypass tools. This project only uses AList WebDAV plus rclone.

References:

- [AList Baidu Netdisk driver](https://alistgo.com/guide/drivers/baidu.html)
- [AList WebDAV](https://alistgo.com/guide/webdav.html)
- [rclone sync](https://rclone.org/commands/rclone_sync/)
- [rclone copy](https://rclone.org/commands/rclone_copy/)
