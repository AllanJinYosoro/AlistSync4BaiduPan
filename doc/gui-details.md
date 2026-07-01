# GUI 细节

`bdp-sync.exe` 不带参数启动时会打开桌面窗口。窗口默认大小约为 `1040 x 720`，标题为 `bdp-sync`。

## Sync 标签

`Sync` 是日常使用入口。

- `Config`: 配置文件路径输入框。默认读取 `config.yaml`。
- `Refresh`: 重新加载配置文件、任务列表和配置编辑页。
- 任务下拉框: 来自 `tasks[].name`。
- `All tasks`: 勾选后禁用任务下拉框，并对所有任务执行操作。
- `Doctor`: 检查环境和配置，不做数据同步。
- `Dry run`: 预览 `Sync` 会造成的变化，不上传、不删除。
- `Update`: 上传新增或修改过的本地文件，不删除远端独有文件。
- `Sync`: 镜像同步，可能删除远端独有文件；GUI 会弹出确认框。
- `Specific`: 只处理当前任务里的一个文件或文件夹。文件只能 `Update`，文件夹可选择 `Update` 或 `Sync`。
- `Stop`: 请求取消正在运行的任务。
- `Clear`: 清空日志区。

运行期间按钮会被禁用，防止同时执行多个任务。任务完成后状态栏会显示完成时间；失败时会显示错误摘要，并把详细错误写入日志区。

日志区会整理 rclone 的动态进度，避免同一段进度反复刷屏。

## 传输前检查

`Update`、`Sync` 和 `Dry run` 会先做这些检查：

- 本地文件名是否包含百度网盘 WebDAV 可能拒绝的字符。目前会拦截全角冒号 `：`。
- 本地 0 字节文件会自动加入本次排除列表，避免上传失败或卡住。

`Specific` 选择的路径必须在当前任务的 `local` 目录内。选择文件时只支持 `Update`；选择文件夹时可以 `Update` 或 `Sync`。

## Config 标签

`Config` 里有两个子标签。

`Form` 子标签用于编辑常用配置字段：

- AList URL
- AList user
- Password env
- Server command
- Startup timeout
- Rclone remote
- Rclone config
- Transfers
- Checkers
- Retries
- Low-level retries
- Global excludes
- Task name
- Task local
- Task remote
- Task excludes

任务区域可以选择、新增或删除任务。新增或删除后需要点击 `Save` 才会写入配置文件。

`YAML` 子标签显示完整配置文本。点击 `Save YAML` 时，程序会先解析并校验内容；如果 YAML 格式错误、必填字段缺失、任务名重复或 URL 不合法，保存会失败，原文件保持不变。

## Dependencies 标签

`Dependencies` 显示 `rclone` 和 `alist` 的检测结果。检测顺序是：

1. 系统 PATH 中的对应 exe。
2. 项目本地 `.alist-sync/tools` 目录中的 exe。

按钮含义：

- `Recheck`: 重新检测工具路径。
- `Install`: 缺什么下载什么。
- `Force reinstall`: 即使已经存在也重新下载。
- `Use rclone`: 选择一个现有 `rclone.exe` 并复制到本地工具目录。
- `Use AList`: 选择一个现有 `alist.exe` 并复制到本地工具目录。

如果启动程序时发现依赖缺失，窗口会询问是否安装到 `.alist-sync/tools`。下载只会发生在用户确认后。
