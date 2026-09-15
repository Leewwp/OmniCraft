## 变更说明

<!-- 本次 PR 做了什么、为什么。关联 issue 用 `Closes #N` / `Fixes #N`。 -->

## 变更类型

- [ ] 缺陷修复
- [ ] 新功能
- [ ] 重构（无行为变化）
- [ ] 文档
- [ ] CI / 工程化
- [ ] 其他：

## 自查清单

- [ ] `bash scripts/verify-project.sh` 通过（涉及前端契约另跑 `--full`）
- [ ] 后端：`go test ./...` / `go vet ./...` / `go build ./...` 通过
- [ ] 前端：`npm run build` / `npm run lint` 通过
- [ ] 新增 UI 字符串已走 `next-intl`（zh/en 同步）
- [ ] 修改了 `config.go` / `migrations/` / `routes.go` 已运行
      `cd tools/doc-validator && go run . --fix`
- [ ] 修改了 `backend/config.yaml` 已运行 `go test -count=1 ./config/...`
- [ ] 不包含真实凭证、密钥、个人敏感信息（含提交中的新增文件）
- [ ] UI 变更已附浏览器验证截图

## 风险与回滚

<!-- 上线风险、数据迁移影响、回滚方式（无则写「无」）。 -->
