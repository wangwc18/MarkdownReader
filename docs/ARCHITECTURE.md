# 工程结构

根目录的 `main.go` 只负责嵌入前端资源并启动桌面应用。Windows 图标资源仍由构建脚本生成在根目录，供链接器使用；不提交生成的 `.syso` 文件。

`internal/desktop/` 是应用内部的桌面后端包：`runtime_windows.go` 负责 Wails 生命周期与运行时，`app.go` 定义桌面桥接和文档状态，其余文件按文件操作、编码、图片、导入导出、设置和拼写等职责命名。包不对外作为通用库发布。

Go 测试与被测实现放在同一个包目录，方便验证内部状态和文件保护逻辑。暂不为目录美观把相互依赖的文档状态强行拆成多个包；后续形成稳定接口的领域服务再单独提取。

`frontend/src/` 管理界面与编辑交互，Wails 后端命名空间为 `window.go.desktop.App`。`scripts/` 负责资源准备、构建与打包，`third_party/` 保留本地修改依赖，`docs/` 和 `examples/` 保存说明与公开样例。

仍从根目录执行 `go test ./...` 和 `scripts/build.ps1`。源码打包脚本会包含 `internal/`，恢复构建的操作不变。
