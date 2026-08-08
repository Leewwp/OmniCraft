# media/t83 阻塞解除与后续 Agent 执行指南

**创建日期**: 2026-08-09
**预计失效日期**: 2026-10-09
**来源会话**: `019fe23c-f1a1-79e1-809d-7e7bed3a8ce5`
**目标**: 解除 `media/t83`（GitHub #83）的真实合同阻塞，并产出可审查、可验证、可交接的 heavy 单提交；本文不授权合并、推送或开启生产 Agent。

## 1. 先读结论

该会话最后仍报告阻塞是合理的：自动化门全部通过，只能证明现有断言通过，不能证明断言覆盖了权威合同。

`media/t83` 还缺两条可观察合同：

1. **私有 OSS 交付合同未闭合**：新媒体封面必须持久化相对 OSS key，并在读取时生成短期签名 URL；当前实现只向 `content_items.cover_image_url` 写 delivery domain URL。未配置 `oss.domain` 时写入空字符串，数据库也没有可供后续签名的封面 key。
2. **poster 尺寸不是服务端事实**：当前发布 DTO 的 `poster_width/poster_height` 由客户端提交，`VerifyUploadedObject` 只核对对象大小和 MIME，随后服务直接把客户端尺寸写入 `cover_width/cover_height`。客户端可提交与真实 poster 不同的宽高。

因此“Go test/vet/build 全绿”与“#83 可完成”并不等价。解除阻塞必须让下面第 4 节的两个红测试先失败、实现后转绿，并通过第 7 节的真实私有 OSS smoke。

## 2. 当前事实快照

开始工作前重新执行只读命令核对；若输出不同，以现场状态为准并更新 `progress.txt`。

```bash
git worktree list --porcelain
git -C /Users/pp/Desktop/file/code/project/OmniCraft-wt-83 status --short
git -C /Users/pp/Desktop/file/code/project/OmniCraft-wt-83 diff --stat
git -C /Users/pp/Desktop/file/code/project/OmniCraft-wt-83 branch --show-current
gh issue view 83 --json number,title,state,labels,body,comments,url
```

2026-08-09 的已知状态：

- `main` 位于 `be6bf89`。
- 主 worktree 位于 `web/agent-t6`，领先 `main` 4 个提交且有未提交修复；不得把它当成干净 `main`。
- `media/t83` 位于 `f82b271`，领先 `main` 1 个 heavy 提交，另有 9 个文件的未提交审查修复。
- GitHub #83 已关闭，验收项均被勾选，标签仍为 `ready-for-agent`；这与本地未闭合合同和 dirty worktree 不一致。tracker 状态不是完成证据。
- `media/t82`、`web/t66`、`web/t67`、`web/t70` 也保留了审查期未提交补丁。处理 #83 时不得清理、覆盖、合并或顺带提交这些 worktree。

已通过且无需无理由重复调查的门：主 worktree 与 t83 的 Go tests/vet/build、主前端与 t82 的单测/类型检查/build、`git diff --check`、gofmt、doc-validator。实现本指南的新增改动后必须重新运行相关门。

## 3. 权威与边界

按 `AGENTS.md` 的冲突优先级执行：

- `architecture.md` §2.2：OSS 文件路径持久化为相对 key，域名/桶切换不改数据库。
- `docs/superpowers/specs/2026-08-08-omnicraft-media-experience-design.md`：后端消费并核验 poster grant，再派生持久 cover URL/OSS key 与尺寸；禁止客户端任意 cover URL/key。
- GitHub #83：heavy，独立 worktree、单一提交、先失败测试、两阶段审查。
- GitHub #84：前端负责采集并提交媒体/首帧尺寸及 poster grant；它依赖 #83。客户端尺寸可服务交互与预校验，不能成为 poster 持久尺寸的唯一信任源。

推荐的最小兼容策略：

- 在 `063_content_media_metadata.sql` 增加可空 `cover_oss_key TEXT`。该迁移尚未合入 `main`，应在同一 heavy 迁移内一次收口，不新占迁移编号。
- `ContentItem` 增加内部 `CoverOSSKey` 字段，API 默认不直接暴露 key；保留 `cover_image_url` 仅兼容历史外部 URL。
- 新 image/video 发布只持久化封面相对 key、服务端尺寸；响应时优先用 key 生成短期签名 URL，旧行无 key 时才回退历史 `cover_image_url`。
- 签名 URL 只存在于响应对象，不写数据库、不写日志、不进入生命周期可能超过签名 TTL 的缓存值。

若实现者认为上述策略与生产代码发生新冲突，停止编码，按文档治理规则记录 issue；不得回退为未签名 bucket endpoint、把完整签名 URL长期写库，或用“已配置 CDN domain”绕开私有桶分支。

## 4. 先建立两个紧的红测试

在 `/Users/pp/Desktop/file/code/project/OmniCraft-wt-83` 内工作。先读现有 `content_media_gallery_test.go`、upload grant/OSS 测试和内容 handler/router 测试，复用既有 seam。

### 4.1 相对 key + 读取签名

新增测试必须证明：

- 在 `oss.domain` 为空的私有桶配置下发布 image 或 video，数据库仍保存非空相对 `cover_oss_key`；数据库不包含 `?Expires=`、`Signature=`、bucket endpoint 或 CDN domain。
- API 返回的 `cover_image_url` 是由该 key 在读取时解析出的可用短期 URL。
- 缓存命中后仍重新解析，不会返回已过期的旧签名 URL。
- legacy 行只有 `cover_image_url`、没有 key 时仍按原行为读取。

测试应注入确定性 signer，不访问公网；让 signer 返回带调用序号的 URL，以断言缓存后会重新签名。这个测试在当前代码上必须失败，保存失败输出到 `progress.txt` 后才能实现。

### 4.2 poster 服务端尺寸派生

将对象核验 seam 改为能返回服务端对象事实，例如：

```go
type UploadedObjectFacts struct {
    ContentLength int64
    ContentType   string
    Width         int
    Height        int
}
```

新增测试：客户端提交 `poster_width=1`、`poster_height=1`，fake OSS verifier 返回真实 `640x480`；发布后数据库必须保存 `640x480`。再覆盖无法读取尺寸、非正尺寸或 MIME/格式不一致时：

- 发布返回受控错误，不暴露原始 OSS 错误；
- 内容和附件均不写库；
- 已消费 poster/media grants 被恢复；
- 不产生空 cover key 或半完成记录。

这个测试在当前代码上必须失败。不要把断言写成“接受客户端 1x1”，也不要只单测一个脱离发布事务的尺寸 helper。

建议用 `/tmp` 下的独立缓存避免沙箱把构建缓存误判为代码失败：

```bash
cd /Users/pp/Desktop/file/code/project/OmniCraft-wt-83/backend
GOCACHE=/tmp/omnicraft-t83-gocache go test ./internal/service -run 'TestPublish.*(Cover|Poster)' -count=1
```

## 5. 最小实现路径

### 5.1 持久化与响应分层

1. 更新 `063_content_media_metadata.sql`、`internal/model/content.go` 及 migration fixture/幂等测试，加入 nullable `cover_oss_key`；legacy 行不回填。
2. image 封面使用 `sort_order=0` 附件 grant 的 `OSSKey`；video 封面使用已核验 poster grant 的 `OSSKey`。新媒体路径不再调用 `PersistentObjectURL` 生成持久字段。
3. 建立单一 cover resolver/signer seam：有 key时生成 GET 签名 URL，无 key时返回 legacy `cover_image_url`。
4. 在 HTTP 响应边界解析列表、详情、推荐、收藏集/系列等实际暴露 `cover_image_url` 的路径。缓存只保存原始 key/legacy URL；对缓存结果复制后解析，避免修改共享缓存对象。
5. signer 使用 `oss.download_url_ttl_sec` 对应配置；任何签名 query、AccessKey、grant ID 均不得记录到日志。

逐个用 `rg -n "CoverImageURL|cover_image_url" backend/internal` 枚举消费者，完成标准是每个公开响应路径都有明确处理或有测试证明不需要处理。

### 5.2 服务端读取 poster 尺寸

1. 扩展 `internal/pkg/aliyun/oss.go` 的窄接口，让服务端可读取图片元信息。优先使用 OSS `image/info` 或只读取解码所需头部；不要把整张大图无上限读入内存。
2. 支持仓库允许上传的全部 poster 图片格式。若新增解码依赖，先确认 Go 1.22+ 兼容并执行 `go mod tidy`；不要静默把已允许格式降级成运行时不可发布。
3. `VerifyUploadedObject` 同时核对 grant 的 key、长度、MIME，并返回实际宽高。发布服务只使用返回事实写 `cover_width/cover_height`。
4. `poster_width/poster_height` 可暂留 DTO 兼容 #84，但只作客户端声明/交叉校验；它们不得覆盖服务端事实。若声明与事实不一致，推荐拒绝并返回统一 `MEDIA_SET_INVALID`，便于发现损坏或伪造输入。
5. 保持 grant 消费、对象核验、内容/附件写入、失败恢复的现有事务语义；对外错误继续走安全错误信封。

## 6. 本地与项目验证

按 heavy 车道完成红—绿—重构，并依次执行：

```bash
cd /Users/pp/Desktop/file/code/project/OmniCraft-wt-83/backend
gofmt -w <本任务改动的精确 Go 文件列表>
GOCACHE=/tmp/omnicraft-t83-gocache go test ./internal/service ./internal/handler ./internal/model ./internal/repository -count=1
GOCACHE=/tmp/omnicraft-t83-gocache go test ./...
GOCACHE=/tmp/omnicraft-t83-gocache go vet ./...
GOCACHE=/tmp/omnicraft-t83-gocache go build ./...
cd ../tools/doc-validator
GOCACHE=/tmp/omnicraft-doc-validator-gocache go run . --fix
cd ../..
git diff --check
```

若 `config.go`、migration 或 route 被修改，doc-validator `--fix` 是强制门。检查它生成的文档 diff，不能只记录退出码。

两阶段审查必须分开记录：

1. **规格符合性**：逐条对照 #83、媒体设计、architecture 相对 key 合同，特别检查缓存与 legacy 行。
2. **代码质量**：资源关闭、读取上限、签名 TTL、错误脱敏、事务/grant 恢复、并发与缓存对象变异。

`DONE_WITH_CONCERNS` 中的 blocking 项处理完后再进入真实 smoke。

## 7. 真实私有 OSS smoke 是最终解除门

该门不能用 fake signer、公开 bucket 或仅配置 `oss.domain` 替代。需要用户提供/确认以下 gitignored 环境输入：

- 阿里云 OSS endpoint、private bucket、AccessKey ID/Secret；
- 可用 Redis；
- 本地测试 DB 与测试账号；
- 一个尺寸已知的小型 PNG/JPEG/WebP fixture。

不得把任何密钥、签名 query 或完整 `.env` 输出到终端记录、文档、issue、commit。

真实路径至少验证：

1. 通过现有 presign + upload grant 上传已知尺寸 poster 和 video/media 对象。
2. 调用真实发布 API；正常路径返回成功，错误路径覆盖 owner/MIME/尺寸不一致之一。
3. 查询数据库：保存相对 `cover_oss_key` 与服务端真实宽高；无完整 OSS/CDN/签名 URL。
4. 调用列表和详情 API：返回可用签名 `cover_image_url`；实际 GET 为 200 且内容类型正确。
5. 证明重新读取会生成新签名，且 private bucket 的无签名 endpoint 不能直接读取。
6. 错误路径无内容/附件半写入，grant 恢复后可受控重试。

若真实 OSS/Redis/DB 输入缺失或连接失败，严格按 `AGENTS.md` 的阻塞模板更新 `progress.txt` 并停止：不提交、不勾选、不关闭 issue、不把 mock 结果当发布证据。

## 8. 跟踪、提交与交接

只有第 4～7 节全部通过后：

1. 更新 t83 worktree 的 `progress.txt`，记录红测试、绿测试、真实 smoke 的脱敏命令与结果。
2. 重新核对 GitHub #83。它目前被过早关闭；若本次任务授权修改 tracker，先在 issue 留下“权威合同补强”说明并恢复进行中状态，完成后再同步验收与关闭。未授权时只报告状态漂移。
3. 使用 `git add <精确文件列表>`。heavy 任务要求最终只有一个提交；确认原提交未推送后，将修复与原 `f82b271` 整理为单一提交。任何历史重写前先核对远端并遵循用户指令。
4. 不自动合并。原会话最后一轮明确要求保留各 worktree 且不合并；只有新的用户指令明确授权后才按 DAG 合并。

解除 #83 后仍不要宣称 Web Agent Task 6 完成：`web/agent-t6` 的 Step 2/3/5/6 仍未勾选，生产 `agent.web_agent_enabled=false` 必须保持。Provider 直连 smoke 不是 AgentService/工具/引用/降级的全链路 smoke；这是独立未完成项，不应与 #83 的 OSS 阻塞混写。

## 9. 完成判定

同时满足以下条件才可向用户报告“#83 阻塞已解除”：

- 两个红测试均在旧实现上失败、在新实现上通过；
- 新媒体数据库只持久化相对 cover key 和服务端尺寸；
- 所有公开 cover 响应路径在读取时签名，legacy 行兼容，缓存不保存过期签名；
- 私有 OSS 真实正常/错误路径 smoke 通过且证据已脱敏记录；
- Go test/vet/build、doc-validator、gofmt、`git diff --check` 通过；
- 两阶段审查无 blocking concern；
- `progress.txt`、计划/issue 状态与实际完成度一致；
- 未混入其他 worktree 改动，未泄露密钥，未擅自合并或开启生产 Agent。
