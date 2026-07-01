# bdp-sync.exe

`bdp-sync.exe` 是一个 Windows 桌面同步工具，用 AList WebDAV 和 rclone 把本地文件夹备份到百度网盘。普通使用不需要打开终端：把 `bdp-sync.exe`、`config.yaml` 和 `.env` 放在同一目录，双击 exe 即可打开窗口。

![bdp-sync.exe 主界面](doc/images/bdp-sync-main.png)

## exe 功能

### 同步任务

主界面的 `Sync` 标签用于执行已配置的任务：

- `Config` 选择要读取的配置文件，默认是当前目录下的 `config.yaml`。
- 任务下拉框选择 `tasks[].name` 中的一个任务；勾选 `All tasks` 时会依次处理全部任务。
- `Doctor` 只做检查：配置格式、依赖工具、AList 地址、WebDAV 凭据、rclone remote、以及本地文件名是否包含百度网盘不支持上传的字符。
- `Dry run` 预览完整同步会发生什么，不上传、不删除。
- `Update` 上传新增或变更的本地文件，不删除网盘端独有文件。
- `Sync` 让网盘端与本地目录保持一致，可能删除网盘端独有文件；点击后会先弹确认框。
- 下方日志区显示本次执行输出，`Clear` 只清空日志显示。

### 配置编辑

`Config` 标签提供两种编辑方式：

- `Form` 适合修改常用字段，例如 AList 地址、用户名、密码环境变量名、rclone 并发数、全局排除规则。
- `YAML` 可以直接编辑完整 `config.yaml`，包括所有任务列表。

点击保存时，程序会先校验 YAML 和必填字段；校验失败时不会覆盖配置文件。

### 依赖管理

`Dependencies` 标签用于管理 `rclone` 和 `alist`：

- `Recheck` 重新检测 PATH 和 `.alist-sync/tools` 中的工具。
- `Install` 下载缺失的 `rclone.exe` 和 `alist.exe` 到 `.alist-sync/tools`。
- `Force reinstall` 重新下载并覆盖本地工具。
- `Use rclone` / `Use AList` 可以选择已有 exe，并复制到 `.alist-sync/tools`。

程序启动时也会检查依赖；如果缺少 `rclone` 或 `alist`，会询问是否下载到本地工具目录。

## config.yaml 设置

最小配置由三部分组成：`alist`、`rclone`、`tasks`。

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
  retries: 2
  low_level_retries: 20
  excludes:
    - "**/.venv/**"
    - "**/__pycache__/**"
    - "**/.git/**"

tasks:
  - name: "documents"
    local: "D:/Documents"
    remote: "/BaiduPanBackup/Documents"
    excludes:
      - "private/**"
```

### alist

- `url`: AList 服务地址。默认本机地址是 `http://127.0.0.1:5244`。
- `username`: AList 里用于 WebDAV 的用户名。
- `password_env`: 存放 AList WebDAV 密码的环境变量名。它不是百度网盘密码。
- `server_command`: 可选。执行 `Doctor`、`Dry run`、`Update`、`Sync` 时，如果 `url` 不可访问，程序会用这个命令启动 AList。
- `startup_timeout_seconds`: 启动 AList 后等待服务可访问的最长秒数。

把密码放到同目录的 `.env` 文件中更方便：

```text
ALIST_PASSWORD=your_alist_webdav_password
```

如果你把 `password_env` 改成别的名字，`.env` 里的变量名也要同步修改。

### rclone

- `remote`: 程序写入 rclone 配置时使用的 remote 名称，通常保持 `alist_baidu` 即可。
- `config_file`: 程序维护的 rclone 配置文件路径，默认是 `.alist-sync/rclone.conf`。
- `transfers`: 同时传输的文件数。网络或网盘限速明显时可以调小。
- `checkers`: 并发检查数量。
- `retries`: 普通失败重试次数，默认 `2`。
- `low_level_retries`: 底层网络/API 失败重试次数，默认 `20`。
- `excludes`: 全局排除规则，所有任务都会生效。

`rclone.conf` 由程序自动生成和刷新，通常不需要手动编辑。

### tasks

每个任务表示一个本地目录到百度网盘目录的同步关系：

- `name`: 窗口里显示的任务名，必须唯一。
- `local`: 本地文件夹路径。Windows 路径可以写成 `D:/Documents`，也可以写成 `C:\\Users\\Name\\Documents`。
- `remote`: AList 中挂载出来的远端路径，例如 `/BaiduPanBackup/Documents`。
- `excludes`: 仅对当前任务生效的排除规则，会追加到全局 `rclone.excludes` 后面。

`Update` 和 `Sync` 都会先检查任务中的本地文件名；如果发现百度网盘不支持上传的字符，会中止并在日志里列出问题。

## 更多文档

- [GUI 细节](doc/gui-details.md)
- [安装与准备](doc/installation.md)
- [命令行兼容模式](doc/commands.md)
- [项目结构](doc/project-layout.md)