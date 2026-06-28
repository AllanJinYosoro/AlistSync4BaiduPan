# 命令行兼容模式

`bdp-sync.exe` 现在主要作为 Windows 桌面程序使用。只有在传入命令行参数时，它才会进入兼容的 CLI 模式。普通用户不需要使用本页内容。

## 可用命令

```powershell
bdp-sync.exe init
bdp-sync.exe setup deps
bdp-sync.exe doctor
bdp-sync.exe dry-run documents
bdp-sync.exe update documents
bdp-sync.exe sync documents
```

- `init`: 创建默认 `config.yaml` 和本地状态目录。
- `setup deps`: 检查并安装 `rclone` 和 `alist`。
- `doctor`: 检查配置、依赖、AList、WebDAV/rclone 连接和上传文件名。
- `dry-run`: 预览完整同步会改变什么。
- `update`: 上传新增或修改的本地文件，不删除远端独有文件。
- `sync`: 让远端与本地一致，可能删除远端独有文件。

`documents` 是 `config.yaml` 中的任务名。也可以使用 `--all` 处理全部任务。

## 全局配置路径

CLI 模式支持 `--config PATH` 指定配置文件：

```powershell
bdp-sync.exe doctor --config config.yaml
bdp-sync.exe update --config config.yaml documents
```

## 开发构建

开发时构建无控制台窗口的 Windows GUI exe：

```powershell
go build -ldflags "-H=windowsgui" -o bdp-sync.exe ./cmd/bdp-sync
```

exe 图标由 `assets/app-icon.ico` 生成并嵌入。修改图标后，先重新生成 Windows 资源文件：

```powershell
cd cmd/bdp-sync
windres -O coff -F pe-x86-64 -i bdp-sync.rc -o rsrc_windows_amd64.syso
cd ../..
```

然后再执行上面的 `go build`。

如果需要保留终端输出，可以另外构建 CLI 调试版本：

```powershell
go build -o bdp-sync-cli.exe ./cmd/bdp-sync
```

Fyne 打包方式：

```powershell
go install fyne.io/tools/cmd/fyne@latest
fyne package -os windows
```
