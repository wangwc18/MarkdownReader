# Typora 1.3.8 自动编号基线（2026-10-23）

computer-use实际操作偏好设置→Markdown→自动添加序号。原设置关闭。临时开启后正常关闭空白窗口，使用完整进程视图核验Typora进程Count=0，再启动并打开examples/公式编号验证.md。读取稳定截图和可访问性结构后，恢复关闭，完整退出核验Count=0，再次启动并打开同一文件取得关闭状态。

| 样本 | 自动编号开启 | 自动编号关闭 |
| --- | --- | --- |
| 行内q=1 | 不编号 | 不编号 |
| 定义之前ref{last} | 4 | ??? |
| 首式a+b=c、label{first} | (1) | 无编号 |
| 手动tag{X}、label{manual} | (X)，不消耗自动序号 | (X) |
| notag | 无编号 | 无编号 |
| align两行 | (2)、(3) | 无编号 |
| 末式z=0、label{last} | (4) | 无编号 |
| 文末ref{first}、eqref{manual}、ref{last} | 1、(X)、4 | ???、(X)、??? |

原始结构见typora-numbering-on-tree.txt和typora-numbering-off-tree.txt；截图中编号位于公式右侧。关闭状态是恢复后的当前设置，换行选项维持原来的开启、physics维持关闭。程序自带授权提示按用户既有说明点“不是现在”，未修改授权机制。

样本只打开观察，没有编辑或保存。SHA256：1D8D1BE42338EE17A240BBF0A81AA98F931744C5FC5750C2BFE30A39A8F77ACB，271字节。该文件不是用户个人文档。

本轮完成静态编号和引用基线，不证明Typora插入/删除/撤销、跨文档、重复标签、编号偏移和刷新行为已全部对齐。后续MarkdownReader已实现文档级排版，固定样本、编辑删除撤销与导出29项检查通过，原生点击3项检查通过，见math-numbering-smoke-result.json和math-numbering-cua-result.json。这些产品检查与先前math-numbering-research.md中的库诊断分开记录；完整交互与视觉对齐仍待验收。
