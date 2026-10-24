# 备份与恢复

当前为 0.1.0 Preview。此次整理没有推送 GitHub；本地 Git 提交会随工程目录一起删除。只有已推送远端，或已验证工程目录外的备份，才能从相应位置恢复。

## 删除工程前

- 保存工程目录外的 Git bundle、源码 ZIP、依赖原始压缩包及 SHA256 清单。
- `data/` 内的配置、个人词典、草稿恢复数据以及自己的文档，应另行私下备份，不要提交到公开仓库。
- `archive/`、`downloads/`、`dist/`、`build/bin/` 和 `node_modules/` 被 Git 忽略；历史原始测试记录也不属于公开源码。删除工程会删除这些本地内容。
- 本次公开留痕见 [交接记录](docs/HANDOFF.md) 和 [隐私检查](docs/PRIVACY_REVIEW.md)。项目自身许可证尚待确定，第三方许可见 `licenses/`。

## 获取源码

日后实际推送 GitHub 后，将下面占位地址替换为真实仓库：

```powershell
git clone https://github.com/OWNER/REPO.git MarkdownReader
cd MarkdownReader
```

未推送时，从工程外备份恢复：

```powershell
git bundle verify D:\MarkdownReader-recovery\MarkdownReader.bundle
git clone D:\MarkdownReader-recovery\MarkdownReader.bundle MarkdownReader
cd MarkdownReader
```

源码 ZIP 也可直接解压构建，但不包含 Git 历史。

## 环境与构建

使用 Windows x64、Git、Go 1.26.1、Node.js 24 和 npm。推荐在普通用户可写的目录工作。

```powershell
.\scripts\setup-deps.ps1
.\scripts\build.ps1 -Package
```

构建脚本安装锁定的 npm 依赖、准备本地资源、运行检查并构建；输出程序在 `build/bin/`，ZIP 在 `dist/`。详细参数见脚本及 README。

WebView2 固定版本的旧下载地址日后可能失效；优先保留这两份原始文件。本次 URL 探测遇到 DNS 限制，未验证远端长期可用性。

| 文件 | SHA256 |
| --- | --- |
| webview2-155.0.4283.45-x64.cab | EC533BF1B7354D288703CF821A2D640D0130108224F93E330BB9C09FAF3F8010 |
| pandoc-3.12.1.zip | 6EF431CB20B2C24A0371FC19200DC49D452BE54B1B5C9239F3462841EF53E587 |

将文件放入 `downloads/`，或指定工程外缓存：

```powershell
.\scripts\setup-deps.ps1 -DependencyCache 'D:\MarkdownReader-dependencies' -Offline
.\scripts\build.ps1 -Package
```

`-Offline` 仅针对上表依赖；首次获取 Go 模块、npm 包仍需要网络，除非另有完整缓存。版本和哈希不能随意替换。下载地址在 `scripts/setup-deps.ps1`，编辑器依赖在 `frontend/package-lock.json`，Go 依赖在 `go.mod`、`go.sum`。

## 恢复后验证

先让构建脚本的自动检查通过，再简单验证打开、编辑、保存、源码切换、图片及导出即可；当前暂缓大规模 Typora 对齐测试。Go 的文件替换测试需要正常 Windows 文件权限，受限沙箱的访问拒绝不应直接视为产品故障。不同机器构建 EXE 的字节哈希不保证一致。

完整无开发环境、无系统 WebView2、断网、普通用户权限的 Windows 10/11 验证仍待完成，不能把本机预览版验证等同于全部环境验收。
