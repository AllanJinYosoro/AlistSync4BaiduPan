# 安装与准备

普通用户只需要使用 `bdp-sync.exe`。推荐目录结构：

```text
bdp-sync.exe
config.yaml
.env
.alist-sync/
```

`config.yaml` 写同步任务，`.env` 保存 AList WebDAV 密码，`.alist-sync` 由程序自动维护。

## 准备 AList

这个工具通过 AList 的 WebDAV 接口访问百度网盘，所以需要先在 AList 里完成百度网盘挂载。

1. 安装并启动 AList。
2. 打开 `http://127.0.0.1:5244`。
3. 登录 AList 管理界面。
4. 添加百度网盘存储，并使用你自己的百度账号授权。
5. 确认用于同步的 AList 用户有 WebDAV 权限。
6. 把该 AList 用户的密码写入 `.env` 中的 `ALIST_PASSWORD`。

`ALIST_PASSWORD` 是 AList 用户密码，不是百度网盘账号密码。

## config.yaml 字段

配置文件由三部分组成：

- `alist`: AList 地址、WebDAV 用户名、密码环境变量名，以及可选的 AList 启动命令。
- `rclone`: rclone remote 名称、配置文件位置、并发数、重试次数和全局排除规则。
- `tasks`: 一个或多个同步任务，每个任务包含任务名、本地目录、网盘目录和任务级排除规则。

常用字段说明：

- `alist.url`: AList 服务地址，默认本机地址通常是 `http://127.0.0.1:5244`。
- `alist.username`: AList 里用于 WebDAV 的用户名。
- `alist.password_env`: 保存 AList WebDAV 密码的环境变量名，默认可用 `ALIST_PASSWORD`。
- `alist.server_command`: AList 不可访问时用于启动 AList 的命令。
- `rclone.transfers`: 同时传输的文件数。网络或网盘限速明显时可以调小。
- `rclone.excludes`: 全局排除规则，所有任务都会生效。
- `tasks[].name`: 任务名，必须唯一。
- `tasks[].local`: 本地文件夹路径，例如 `D:/Documents` 或 `C:\Users\Name\Documents`。
- `tasks[].remote`: AList 中挂载出来的远端路径，例如 `/BaiduPanBackup/Documents`。
- `tasks[].excludes`: 仅对当前任务生效的排除规则。

`rclone.conf` 由程序自动生成和刷新，通常不需要手动编辑。

## Windows 手动安装 AList

从 AList release 页面下载 Windows 64 位压缩包，通常是 `alist-windows-amd64.zip`。解压到稳定目录，例如 `C:\alist`，然后启动：

```powershell
cd C:\alist
.\alist.exe server
```

如果需要设置或重置管理员密码：

```powershell
.\alist.exe admin set NEW_PASSWORD
.\alist.exe admin random
```

配置完成后，可以把 `config.yaml` 里的启动命令设为：

```yaml
alist:
  server_command: "C:/alist/alist.exe server"
```

如果使用 GUI 的 `Install` 下载本地 AList，则可以保持默认值：

```yaml
alist:
  server_command: ".alist-sync/tools/alist.exe server"
```

## Docker 安装 AList

如果你用 Docker，可以用持久化数据目录运行 AList：

```powershell
docker run -d --restart=unless-stopped `
  -v C:\alist\data:/opt/alist/data `
  -p 5244:5244 `
  -e PUID=0 -e PGID=0 -e UMASK=022 `
  --name alist xhofe/alist:latest
```

设置或重置管理员密码：

```powershell
docker exec -it alist ./alist admin set NEW_PASSWORD
docker exec -it alist ./alist admin random
```

## 百度 VIP/SVIP 说明

本项目只使用 AList WebDAV 和 rclone。上传能力取决于 AList 的百度网盘驱动、百度账号状态以及百度官方接口行为。

不要使用破解接口或限速绕过工具。

## 参考

- AList manual installation: https://alistgo.com/guide/install/manual.html
- AList Docker installation: https://alistgo.com/guide/install/docker.html
- AList Baidu Netdisk driver: https://alistgo.com/guide/drivers/baidu.html
- AList WebDAV: https://alistgo.com/guide/webdav.html
