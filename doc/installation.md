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