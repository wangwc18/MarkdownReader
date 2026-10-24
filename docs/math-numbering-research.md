# 公式编号管线诊断（2026-10-23）

本文件记录实现前的固定样本与实际MathJax3.1.2诊断，诊断本身不计为产品功能通过。后续产品已接入文档级排版和自动编号开关，固定样本29项与原生点击3项检查结果另见math-numbering-smoke-result.json、math-numbering-cua-result.json。

`scripts/math-numbering-research.js`只在空白、干净、无公式的测试窗口运行。使用相同TeX，比较逐公式tex2svgPromise与整容器typesetPromise，结果写入math-numbering-research-result.json；最后清理临时容器并恢复配置，随后退出测试程序。

| 案例 | 逐公式转换 | 整文档排版 |
| --- | --- | --- |
| 定义之前的ref{last} | ??? | 4 |
| 首式label{first} | (1) | (1) |
| 手动tag{X} | (X)，不占自动序号 | 同左 |
| notag | 无编号 | 同左 |
| align两行 | (2)、(3) | 同左 |
| 最后一式label{last} | (4) | 同左 |
| 文末引用 | 1、(X)、4 | 同左 |

仅继续转换新公式会得到(5)，说明若直接为现有增量节点渲染打开tags=all，计数会沿用旧状态；不能将该方式视为编辑后正确重排。整文档重建并删除首式后，前向引用变3，align为(1)/(2)，末式为(3)，悬空的first引用为???。

下一步需要文档级数学事务：同一文档按顺序一次排版、重新建立编号与标签表、处理前向引用，再提交到当前仍有效的预览节点。应隔离编辑/导出的渲染状态，排队避免共享MathJax并发；必须实测插入/删除/撤销、跨文档、模式切换、导出和旧节点失效。纯库能力不能代替这些产品验收。

实现依据：
- https://docs.mathjax.org/en/v3.1/web/typeset.html
- https://docs.mathjax.org/en/v3.0/input/tex/eqnumbers.html
