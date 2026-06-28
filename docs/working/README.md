# docs/working/ — 临时工作文档

**预计失效日期**: 2099-12-31

## 用途
存放审查报告、修复记录、分析文档等临时性质的文档。

## 命名规则
文件名格式: `YYYY-MM-DD-<scope>-<type>.md`
- 审查报告: `YYYY-MM-DD-<scope>-review.md`
- 修复记录: `YYYY-MM-DD-<scope>-fix-log.md`

## 生命周期
- 创建时在文件头部注明 `**预计失效日期**: YYYY-MM-DD`
- 默认失效日期 = 创建日期 + 2 个月
- 失效后由 Agent 移至 `../archive/`
