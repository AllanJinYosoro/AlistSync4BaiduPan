# 项目结构

源码按职责拆分：

- `cmd/bdp-sync`: exe 入口。无参数时启动 GUI，有参数时进入 CLI 兼容模式。
- `internal/gui`: Fyne 桌面界面、按钮状态、日志缓冲和 GUI 参数组装。
- `internal/config`: YAML 解析、默认值、校验、保存、`.env` 读取和任务选择。
- `internal/deps`: `rclone` / `alist` 检测、下载和本地工具目录管理。
- `internal/alist`: AList 可达性检测、启动命令解析、后台启动和等待服务就绪。
- `internal/rclone`: rclone WebDAV 配置生成和传输参数构建。
- `internal/runner`: CLI 兼容命令、doctor、setup、transfer 流程。
- `internal/filename`: 百度网盘上传文件名预检查。
- `internal/proc`: Windows 进程启动与隐藏控制台相关封装。

运行时状态不放在源码包里：

- `.alist-sync/tools`: GUI 下载或用户导入的 `rclone.exe` / `alist.exe`。
- `.alist-sync/rclone.conf`: 程序自动维护的 rclone WebDAV 配置。
- `data/`: AList 运行数据目录。
- `.env`: 本地密码变量文件，不应提交。