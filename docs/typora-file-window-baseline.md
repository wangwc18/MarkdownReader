# Typora 1.3.8 文件与窗口基线

2026-10-16，computer-use 实际读取本机 Typora 文件与视图菜单。

- 再次读取原生“打开”对话框，文件类型为 `*.;*.mdtxt;*.mdtext;*.apib;*.rmarkdown;*.rmd;*.txt;*.text;*.md;*.markdown;*.mmd;*.mkd;*.mdwn;*.mdown;*.mdx`。只读取筛选后取消，没有修改文档。
- MarkdownReader 将该集合用于普通打开、按编码打开、文件树、文件夹搜索和应用内文档链接。无扩展名、大小写扩展名和全部 14 个带扩展名类型有集成测试；二进制类型仍不纳入文件树或全文搜索。

- 文件菜单：“删除…”位于“在侧边栏中显示”之后。只读取入口，未执行删除或确认删除，不据此推定 Typora 的删除提示和恢复策略。
- 视图菜单：“应用内窗口切换”，快捷键 Ctrl+Tab。
- MarkdownReader 此前 Ctrl+Tab 切换标签页；现改为原生应用窗口切换，标签页使用 Ctrl+PageDown。Ctrl+Shift+Tab 及窗口列表是补充入口。
- 本机实测 MarkdownReader：Ctrl+Shift+N 创建第二个进程窗口，随后在新窗口按 Ctrl+Tab，原有示例窗口成为前台。`window-before-cycle.json` 与 `window-after-cycle.json` 记录窗口句柄、标题与前台变化。空白测试窗口随后正常关闭。

## Windows 实现依据

- [SHFileOperationW](https://learn.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-shfileoperationw) 与 [SHFILEOPSTRUCTW](https://learn.microsoft.com/en-us/windows/win32/api/shellapi/ns-shellapi-shfileopstructw)：使用完整路径、双 NUL 终止列表、FOF_ALLOWUNDO，检查返回值及取消标志。程序另保留校验过的恢复副本，因此不依赖系统回收站独立保管恢复内容。
- [SetForegroundWindow](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setforegroundwindow)：切换窗口；窗口枚举限定 MarkdownReader 类与进程名。系统拒绝前台切换时报告错误，不修改系统设置。

删除行为、确认提示、恢复副本、原生窗口的完整排序规则尚未与 Typora 逐项比较，因此此项为功能已实现、部分已验证，仍存在差异。
