# 2026-08-03 下载完整性校验与安全模型调研（wayfinder #7 事实补齐）

> **创建日期**: 2026-08-03
> **预计失效日期**: 2026-10-03
> **用途**: wayfinder ticket「下载完整性校验与安全模型调研」（GitHub issue #7）的事实调研产出；为 L4 下载动作的校验方案设计提供一手来源
> **上游决策**: ADR-0002（客户端 Agent 扩展与活人感约束）——L4 = 下载/解压/移动/建目录/读写配置，执行模式 = 签名 grant + 动作预览 + 逐级确认
> **调研方法**: 全部论断来自官方文档/官方源码/官方 SDK（URL + 锚点），区分事实与推断（推断一律标注）；未使用二手博客

## 0. 结论速览

1. **可直接借鉴的校验模型（按适配度排序）**：OSS 原生 CRC64（零成本，传输完整性）> Homebrew sha256 元数据存证模式（发布侧存证 + 下载后阻断式校验）> Go modules「可信元数据 + 阻断式校验」模式（go.sum/checksum DB，透明度日志可选）> npm「integrity 强校验 + 注册表签名审计」双层模式 > TUF 的「targets 元数据携带 hash+length、校验完成前不交付文件」原则（不引入 TUF 全框架）。
2. **校验时机**：所有主流模型都是「下载完成后、副作用（安装/解压/执行）前」做**阻断式**校验；没有「先确认再校验」或「校验通过才让用户确认」的先例。对应到 OmniCraft L4：动作预览（下载前，展示目标+hash+路径）→ 下载 → **立即校验** → 校验通过才允许后续动作（解压/移动/安装）；校验失败 = 动作失败，不产生"已下载"成果。
3. **失败呈现**：阻断式错误（Homebrew 中止安装 / Go 报告 security error 并删除缓存 / apt 拒绝仓库）与**警告式降级**（Homebrew 无 checksum 时 opoo 警告并附上实际 sha256；npm 签名审计输出"X packages verified / Y missing"报告）两种模式可分层使用：新内容强校验，历史/无 hash 内容警告 + 标记。
4. **OSS 事实**：OSS 提供对象级 CRC64（服务端计算、随对象返回，支持任意上传方式含 CompleteMultipartUpload）与 Content-MD5（上传时服务端比对），支持服务端签名 URL（SignURL GET，项目已在用）；**CRC64 是传输校验和，不是防篡改安全 hash**——防篡改仍需要发布侧存证 SHA-256。
5. **需自研的部分**：hash 存证与签名 grant 的绑定（grant 内联 `sha256 + size + oss_key + 目标路径 + trace_id`）、校验失败的确认 UX（重试/覆盖/放弃 + 审计记录）。开源生态无此先例（与首轮 A/B 类调研结论一致：mod 管理器无签名级校验，agent 客户端无签名 grant）。

## 1. 背景与现状核查（OmniCraft 代码事实）

| 环节 | 现状（代码事实） |
|------|------------------|
| 上传 | 后端签发 OSS 签名 PUT URL（`GeneratePresignUploadURL`，`backend/internal/service/oss_service.go:66-96`），客户端直传 OSS |
| 上传后验证 | `VerifyUploadedObject` 仅比对 **Content-Length 与 Content-Type**（`oss_service.go:142-158`），`ObjectMeta` 只取这两个字段（`backend/internal/pkg/aliyun/oss.go:30-33`）——**未捕获/存证任何 hash** |
| 下载 | `GET /api/v1/contents/:id/download` 返回 JSON `{download_url, expires_in}`（OSS 签名 GET URL，TTL 可配 `download_url_ttl`），客户端直连 OSS 下载（`content_download_test.go` 断言契约：返回 JSON 非重定向） |
| 哈希存证 | 后端 grep 未见内容文件 hash 列/字段——**发布时无 SHA-256/CRC64 存证** |
| L4 下载动作 | ADR-0002 定义：签名 grant + 动作预览 + 逐级确认，仅 desktop；签名 grant 为自研设计（HMAC→Ed25519 路径），尚无校验层设计 |

结论：**「内容 hash 由后端存证 vs 下载时校验」目前两端都缺失**——本调研的 OSS/生态事实可直接支撑补齐。

## 2. 可借鉴模型的事实与来源

### 2.1 Homebrew（cask sha256）

**机制**：每个 cask 定义中必须声明 `sha256`（"SHA-256 checksum of the file downloaded from `url`"），或显式 `:no_check`。
- 来源：Homebrew Cask Cookbook「Required stanzas」表——https://docs.brew.sh/Cask-Cookbook（`sha256` 行：值 = `shasum -a 256 <file>` 计算结果，或特殊值 `:no_check`）

**校验实现（源码）**：
- `Library/Homebrew/cask/download.rb`：`fetch` 流程 = 下载 → `quarantine(downloaded_path)`（macOS 隔离属性）→ `verify_download_integrity`；无 checksum 且非官方 tap 时 `opoo "No checksum defined ... skipping verification"`；`--require-sha` 且无 sha256 时直接报错。来源：https://raw.githubusercontent.com/Homebrew/brew/main/Library/Homebrew/cask/download.rb（`fetch`/`verify_download_integrity`/`verify_has_sha` 方法）
- `Library/Homebrew/downloadable.rb`：`verify_download_integrity` → `VerificationCache#verify` → `filename.verify_checksum(checksum)`；**缺 checksum 时 `opoo` 警告并打印实际计算的 sha256 供维护者参考**（"For your reference, the checksum is: sha256 ..."）；`downloaded_and_valid?` 捕获 `ChecksumMismatchError` → 视为未下载成功。来源：https://raw.githubusercontent.com/Homebrew/brew/main/Library/Homebrew/downloadable.rb
- 阶段状态机（同文件）：`downloading → downloaded → verifying → verified → extracting`——**校验在解包/安装副作用之前**。

**失败行为**：checksum 不匹配 → `ChecksumMismatchError` → 安装中止；缺失 → 警告 + 给出实际 hash（非阻断）。

### 2.2 npm（lockfile integrity + 注册表 ECDSA 签名）

**机制 A（integrity，安装期）**：`package-lock.json` 每个包描述带 `integrity` 字段 = "A `sha512` or `sha1` Standard Subresource Integrity string **for the artifact that was unpacked** in this location"（lockfileVersion 2/3 的 `packages` 段）。
- 来源：https://docs.npmjs.com/cli/v9/configuring-npm/package-lock-json（`packages` → `integrity` 字段定义）
- 事实边界：文档定义该值为解包产物的 SRI 校验值；「安装时校验失败即中止」为 npm 安装语义的公认行为，本轮未在抓取文本中直接断言，**标注推断**。

**机制 B（注册表签名，审计期）**：
- 公共 registry 对发布包做 ECDSA 签名：签名置于 packument 每个版本的 `dist.signatures`（`keyid: "SHA256:{{SHA256_PUBLIC_KEY}}"` + `sig`）；签名串 = `${package.name}@${package.version}:${package.dist.integrity}`；公钥端点 `registry-host.tld/-/npm/v1/keys`（`keytype: "ecdsa-sha2-nistp256"`）；**目的原文**："protects against an attacker controlling a registry mirror or proxy where they attempt to intercept and tamper with the package tarball content"。
- 校验命令：`npm audit signatures`，输出 "N packages have verified registry signatures / M packages have missing registry signatures"；注册表提供签名公钥而包缺失签名时 CLI 报错。
- 来源：https://docs.npmjs.com/about-registry-signatures（"Supporting signatures on third-party registries" 节）；https://docs.npmjs.com/verifying-registry-signatures

**可借鉴点**：双层模型——完整性（integrity，安装期阻断）+ 真实性（签名，独立审计命令）；签名对象是"元数据串"而非二进制内嵌。

### 2.3 Go modules（go.sum + checksum DB）

**机制**：
- `go.sum` 每行 = `module path version hash`（`h1:` SHA-256）；go 命令下载 `.mod`/`.zip` 后计算 hash 与主模块 `go.sum` 比对，不一致 → **"reports a security error and does not install the file in the module cache"**（阻断）。
- hash 不在 go.sum 时查 checksum database（默认 `sum.golang.org`）："a [Transparent Log](https://research.swtch.com/tlog) (or 'Merkle Tree') of go.sum line hashes, backed by Trillian ... independent auditors can verify that it hasn't been tampered with"；"It makes untrusted proxies possible since they can't serve the wrong code without it going unnoticed"。
- `GOSUMDB=off` / `GONOSUMDB` / `GOPRIVATE` 可关闭（私有模块）；zip 的 hash 是"文件内容的 hash"而非 zip 本身（ziphash，文件顺序/压缩不影响）。
- 来源：https://go.dev/ref/mod#go-sum-files、#checksum-database、#authenticating（"Authenticating modules" 一节 + 术语表 ziphash 条目）

**可借鉴点**：可信元数据（go.sum 提交在仓库）与独立审计源（透明度日志）分离；失败即删除缓存文件、不污染后续构建。透明度日志对 UGC 内容平台是否必要，见 §5。

### 2.4 apt（签名仓库链）

**机制**（apt-secure(8) 原文要点）：
- Release 文件（InRelease/Release.gpg）由归档密钥签名；Release 含 Packages 文件的校验和；Packages 含各 .deb 的校验和——apt 自动沿链校验（"End users can check the signature of the Release file, extract a checksum of a package from it and compare it with the checksum of the package they downloaded - or rely on APT doing this automatically"）。
- **不做包级签名**："apt-secure does not review signatures at a package level"（包级校验是 debsig-verify 的职责）；防 MITM 与镜像被攻破，**不防主服务器被攻破**。
- 未签名仓库默认拒绝（"current APT versions will refuse to download data from them by default"）。
- 来源：https://manpages.debian.org/bookworm/apt/apt-secure.8.en.html（SIGNED REPOSITORIES / UNSIGNED REPOSITORIES 节）

### 2.5 dnf/yum（RPM 签名）

**机制**：仓库/主配置项 `gpgcheck`（对仓库内包做 GPG 签名校验）、`repo_gpgcheck`（对仓库元数据校验）、`localpkg_gpgcheck`（本地包）、`gpgkey`（签名密钥来源）；`gpgcheck` 默认 **False**（该选项"只能强化 %_pkgverify_level 宏定义的 RPM 安全策略"）。
- 来源：https://man7.org/linux/man-pages/man5/dnf.conf.5.html（`gpgcheck`/`repo_gpgcheck`/`gpgkey`/`localpkg_gpgcheck` 条目）
- 与 apt 的差异：RPM 是**包级签名**（RPM v4 内嵌签名）逐包校验，apt 是仓库索引签名 + 索引内 hash 链。

### 2.6 Windows Authenticode

**机制**（Windows 驱动文档原文要点）：
- "identifies the publisher of Authenticode-signed software" + "verifies the software has no changes since it was signed and published"；证书链到受信根 CA。
- 两种载体：**embedded signature**（数字签名嵌入 PE 文件非执行段）与 **catalog files**（.cat 文件内含每个文件的 hash，签名后作为"detached signature"——即"对象外清单 + 清单签名"模式）。
- 来源：https://learn.microsoft.com/en-us/windows-hardware/drivers/install/authenticode（Embedded signatures / Digitally signed catalog files 节）
- 事实边界：强制范围（驱动/内核必签，普通桌面应用由 OS 不阻断、走 SmartScreen 信誉）不在上述页面断言，**标注推断**。

### 2.7 macOS notarization（Gatekeeper）

**机制**（Apple 官方文档原文要点）：
- notary 服务自动扫描软件恶意成分 + 检查代码签名问题；通过后签发 ticket，**在线发布 + 可 staple 到可执行文件**；用户首次安装/运行时 Gatekeeper 查 ticket，并在首次启动对话框展示描述信息帮助用户决定。
- 强制时间线："Beginning in macOS 10.15, all software built after June 1, 2019, and distributed with Developer ID **must be notarized**"；流程要求 Developer ID 证书 + hardened runtime + 安全时间戳。
- 来源：https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution（Apple 文档站需 JS，本调研经其 JSON API `https://developer.apple.com/tutorials/data/documentation/security/notarizing-macos-software-before-distribution.json` 取到正文）

**可借鉴点**：校验结果在**执行时机**（首次启动）由 OS 呈现给用户；"ticket 在线发布 + 本地 staple"双通道。OS 级强制，对 OmniCraft 是 Tauri 打包侧（Ops-09 桌面暂缓范围）的事，与 L4 内容下载无直接关系。

### 2.8 TUF（The Update Framework）

**机制**（规范原文要点）：
- 五角色：root（离线密钥）/ targets / snapshot / timestamp（在线）/ mirrors（可选）；threshold 阈值签名。
- targets 元数据携带目标文件 **hash + length**；客户端流程：root → timestamp → snapshot → targets → fetch target，**"The application code is not given access to the file until the security checks have been completed"**（校验完成前不交付文件）。
- 防护目标：arbitrary installation / rollback / freeze / mix-and-match / endless data 等；委托链（delegated roles，按 path 模式授权）。
- 来源：https://theupdateframework.github.io/specification/latest/（§1.5.2 攻击防护目标、§2.1 角色、§4.5 targets 格式、§5.7 Fetch target）

**可借鉴点**：不是整个框架，而是两个原则——① 可信元数据与对象分离（hash 在签名元数据里，不在对象里）；② 校验完成前不交付。映射到 OmniCraft：下载 grant 响应即"签名元数据"，应内联 hash+size。

### 2.9 cargo（补充对照）

**机制**：registry index 每个版本条目带 `cksum` 字段 = "A SHA256 checksum of the `.crate` file"；下载 URL 模板支持 `{sha256-checksum}` 占位符。
- 来源：https://doc.rust-lang.org/cargo/reference/registry-index.html（Index Configuration `dl` 占位符；JSON schema `cksum` 字段）
- 事实边界：cargo changelog（1.86~1.99，https://doc.rust-lang.org/nightly/cargo/CHANGELOG.html 全量 grep）与 Cargo Book 均**未见索引级数字签名机制**（索引签名 RFC 未落地）；`cargo package` 生成的 `.cargo-checksum.json` 官方明示"is **not** a security mechanism"（changelog 1.97 文档修订条目）。即：cargo 只有"index 内 hash"一层，无签名、无透明度日志。

## 3. 与「签名 grant + 动作预览 + 逐级确认」的衔接

### 3.1 校验时机（事实对照）

| 模型 | 校验时机 | 校验失败效果 |
|------|----------|--------------|
| Homebrew | 下载完成后、解包/安装前（阶段机 verifying → verified → extracting） | 中止安装（ChecksumMismatchError）；缺 hash 警告不阻断 |
| npm | integrity：解包时（阻断，推断）；注册表签名：安装后独立 `npm audit signatures` 审计命令 | 审计输出"X verified / Y missing"报告 |
| Go | 下载后写入模块缓存前 | security error + 删除已下载文件 |
| apt/dnf | 仓库更新/安装事务时 | 拒绝仓库/包 |
| Authenticode | 签名内嵌于对象；OS 在执行驱动时验证 | 驱动不加载 |
| notarization | Gatekeeper 首次启动时（OS 强制） | 首启对话框呈现，用户决定 |
| TUF | fetch target 后、交付应用前 | 文件不交付（不通过安全校验） |

**推断（基于上表归纳）**：主流一致性是「校验 = 下载完成后、副作用执行前的阻断门」。没有任何模型把校验放在用户确认之前作为"确认的前置条件"，也没有"确认后才校验"的先例（notarization 是 OS 执行期兜底特例，且是厂商发布流程而非用户下载流程）。

### 3.2 对 L4 的映射建议

把「签名 grant + 动作预览 + 逐级确认」扩成五步：

1. **预览（确认前）**：展示目标内容 + 存证 hash（如可展示简短前缀）+ 目标路径——对齐 Homebrew 安装时打印校验信息、Copilot 式动作预览；hash 在此阶段即来自服务端 grant 响应。
2. **签名 grant**：服务端签发下载 grant，**内联 `sha256 + size + oss_key + 签名 URL + TTL + trace_id`**（对齐 TUF targets 元数据携带 hash+length、Go go.sum 可信元数据模式）。
3. **下载**：客户端经签名 URL 直连 OSS。
4. **校验（阻断门，放在任何解压/移动/写入动作之前）**：本地计算 SHA-256 与 grant 内 hash 比对；可同时启用 OSS SDK CRC64 做传输层快速校验（§4）。失败 → 动作标记失败、文件不进入下一步（对齐 Go 删除缓存 / TUF 不交付）。
5. **逐级确认与呈现**：校验通过才展示下一级动作（解压/移动/mod 安装）的确认；校验失败呈现 = 阻断式错误 + 重试/更换来源选项，全程记录 trace（对齐 M1 数据标注）。

### 3.3 失败呈现分层（借鉴 Homebrew + npm 双模式）

- **阻断层（强校验）**：新发布内容必须带存证 hash；下载后不匹配 → 动作失败，不允许"继续"（对齐 Go 的 security error 与 apt 的拒绝）。
- **警告层（弱校验）**：历史内容/缺失 hash → 可下载但标记"未经完整性校验"，并给出实际 hash 供审计（对齐 Homebrew `opoo` + 建议 hash、npm audit signatures 的 missing 报告）。
- 可审计：下载动作结果附"校验报告"（verified/missing/mismatch），与 `assisted_by_agent` + `trace_id` 一并落库（M1/M5 可回滚语义天然兼容——失败动作不产生数据副作用）。

## 4. 阿里云 OSS 生态事实

### 4.1 对象级校验和（官方文档事实）

OSS 支持三种校验方式（官方「数据一致性校验」页，https://help.aliyun.com/zh/oss/user-guide/data-verification）：

| 方式 | 语义（原文要点） |
|------|------------------|
| ETag | 判断资源是否变化；PutObject 创建的对象的 ETag = 内容 MD5，**"不建议使用 ETag 作为 Object 内容的 MD5 来校验数据完整性"** |
| MD5 | 上传时客户端经 **Content-MD5 请求头**传给服务端，服务端比对，不一致上传失败；PutObject/GetObject/AppendObject/PostObject/UploadPart 支持；**CompleteMultipartUpload 的 Content-MD5 只校验请求体，校验不了文件完整性** |
| crc64 | **服务端计算后传给客户端比对**；"OSS 现在支持对各种方式上传的 Object 返回其 crc64 值"；CompleteMultipartUpload 若所有 Part 都有 crc64 则返回整个对象 crc64 |

**对 OmniCraft 的含义**：
- 上传侧：分片上传场景（未来 mod 大文件）Content-MD5 头无效，CRC64 是唯一可靠的传输完整性手段；单次 PutObject 可带 Content-MD5 让服务端比对。
- 下载侧：对象 crc64 随响应返回（响应头 `x-oss-hash-crc64ecma`），客户端可本地计算比对。

### 4.2 SDK 支持（Go SDK v3，pkg.go.dev 事实）

- `Bucket.SignURL(objectKey, method, expiredInSec, options)`——服务端签名 URL（GET/PUT），**项目已在用**（`backend/internal/pkg/aliyun/oss.go:66-80`）。
- 签名 URL 下载同样有 SDK 封装：`GetObjectWithURL` / `GetObjectToFileWithURL(signedURL, filePath, options)`。
- 完整性选项：`EnableCRC(isEnableCRC)`、`EnableMD5(isEnableMD5)` 客户端选项；`CheckDownloadCRC(clientCRC, serverCRC)`、`CheckCRC(resp, operation)` 校验函数；`VerifyObjectStrict(enable)` 严格校验选项。
- 来源：https://pkg.go.dev/github.com/aliyun/aliyun-oss-go-sdk/oss（`Bucket.SignURL` / `GetObjectToFileWithURL` / `ClientOption` 枚举 / `CheckDownloadCRC` 等条目）
- 事实边界：SDK 选项「启用后自动比对 CRC64」的运行时行为未在本轮逐行读源码，**标注推断**（函数存在性为事实）。

### 4.3 签名 URL（事实边界）

- 官方「服务端签名直传（PostObject）」文档证实服务端签名 + STS 临时凭证是 OSS 官方授权模式：https://help.aliyun.com/en/oss/developer-reference/...（页面标题 "Server-side signing for direct upload"，含 Go/Node.js 示例）；其聚焦**上传**。
- **下载**侧签名 URL 的官方用户指南页本轮未能定位（多路径 404，阿里云帮助中心改版），以 SDK `SignURL` 文档 + 项目既有实现为事实依据，报告标注此缺口。

### 4.4 防篡改边界（推断，基于 4.1 事实）

CRC64（ECMA-182）与 MD5 均为**非密码学安全**校验和：可检测传输损坏/随机错误，**不能防恶意篡改**（攻击者可重算）。防篡改 = 发布侧对内容计算并存证 SHA-256（密码学强 hash），下载侧与存证比对——CRC64 只作传输层快速校验，不作安全断言。

## 5. 对 L4 下载动作的事实性建议

### 5.1 直接借鉴（附来源）

| # | 借鉴项 | 来源 | 适配说明 |
|---|--------|------|----------|
| 1 | **发布侧 SHA-256 存证 + 下载后阻断式校验**（Homebrew 模式） | Cask Cookbook `sha256` + `downloadable.rb` 源码 | OmniCraft 上传验证时计算并存证内容 SHA-256（补齐 §1 现状缺口）；下载 grant 响应内联 hash；下载完成后、解压/移动前校验。hash 存储 = 后端 DB 列 + 迁移，属于计划内改动，非 OSS 能力 |
| 2 | **OSS 原生 CRC64 做传输层校验**（零额外存储） | OSS 数据一致性校验文档 + Go SDK `EnableCRC` | 上传（含未来分片上传）与下载均可启用；传输损坏快速检出；**不作为防篡改断言** |
| 3 | **可信元数据模式**（Go modules go.sum/checksum DB） | go.dev/ref/mod#checksum-database | 「服务端 DB 为 hash 唯一权威 + 下载侧比对」即 go.sum 的语义；「透明度日志」层对 UGC 平台**不直接采用**（推断：内容非代码供应链，无全球一致性需求，运营成本高） |
| 4 | **hash+size 进签名 grant 元数据、校验完成前不交付**（TUF 原则） | TUF spec §4.5 / §5.7 | grant 响应 = 轻量 targets 元数据：`{url, sha256, size, expires_in, oss_key, trace_id}`；失败不进入下一级动作 |
| 5 | **双层校验：完整性阻断 + 签名/报告审计**（npm 模式） | npm package-lock-json integrity + about-registry-signatures / verifying-registry-signatures | Phase 1 = SHA-256 阻断校验 + 校验报告入审计；Phase 2 = 服务端对 `${content_id}@${version}:${sha256}` 做 Ed25519 签名（对齐 npm 的 `${name}@${version}:${integrity}` 签名串与 `/-/npm/v1/keys` 公钥分发端点），客户端内置平台公钥验证，`assisted_by_agent` 动作同样过此门 |
| 6 | **缺 hash 内容警告式降级**（Homebrew opoo / npm missing 报告） | downloadable.rb / verifying-registry-signatures | 历史无 hash 内容：可下载 + 标记"未校验" + 记录实际 hash，供后续补证或审计 |

### 5.2 明确不采用 / 过重（附理由）

| 模型 | 不采用理由 |
|------|-----------|
| TUF 全框架（五角色/委托链/快照） | 单仓库单内容源，无镜像与多角色需求；只取「元数据内 hash + 不交付原则」两个点（推断：TUF 为多镜像分发设计，本场景需求面过小） |
| apt/dnf 仓库级 GPG 签名链 | OmniCraft 下载不经"仓库索引"，是 API 签发签名 URL + TLS；来源真实性已由签名 URL 覆盖（事实：项目现有流程为直发签名 URL） |
| Authenticode/notarization 对象内嵌签名 | 对象内嵌签名要求 CA 链与 OS 强制，针对可执行二进制；UGC 内容（zip/mod/图文）无此生态，且 L4 签名 grant 已承担来源真实性 |
| cargo 模式 | 只有 index 内 hash 一层，且 `.cargo-checksum.json` 官方明示非安全机制——相比 go.sum/Homebrew 无增量借鉴价值 |

### 5.3 需自研（无开源先例）

1. **签名 grant 与 hash 校验的绑定**：grant 语义（`oss_key + sha256 + size + 目标路径 + trace_id` 强绑定、签名、防重放/防漂移）——首轮 A/B 调研已确认 agent 客户端与 mod 管理器均无此实现（调研骨架 §A 小结 5、§B 小结 3）。
2. **校验失败的确认 UX**：重试 / 覆盖（显式风险确认并记录）/ 放弃 三态呈现 + 审计记录（对齐 M5 回滚语义）。
3. **上传侧 hash 存证闭环**：直传 OSS 模式下服务端无法直接读文件内容——需客户端上传时上报 SHA-256（上传 grant 内绑定预期 hash，上传后 OSS 侧 CRC64 交叉验证），或服务端异步拉取计算；此为业务设计决策，非现成组件。

### 5.4 落地顺序建议（推断，供 wayfinder 后续计划参考）

1. 上传 grant/下载 grant 响应增加 `sha256`（+现有 `expires_in`）；`VerifyUploadedObject` 增加 CRC64 比对（SDK `GetObjectDetailedMeta` 已可取 CRC64 响应头）。
2. 内容发布表增加 hash 列 + 迁移（迁移编号需与活计划注册表协调，避免占用已规划编号）。
3. L4 下载动作管线：预览（含 hash）→ grant（含 hash）→ 下载 → 校验 → 下一级动作确认；失败呈现 + trace 审计。
4. Phase 2：内容签名（§5.1 #5），公钥经受信端点分发。

## 6. 关键事实与来源清单

| # | 事实 | 来源 |
|---|------|------|
| F1 | Homebrew cask 必填 sha256（或 :no_check）；校验在解包前；不匹配中止，缺失警告并给实际 hash | docs.brew.sh/Cask-Cookbook；github.com/Homebrew/brew 主分支 `Library/Homebrew/cask/download.rb`、`Library/Homebrew/downloadable.rb` |
| F2 | npm lockfile `integrity` = 解包产物的 SRI（sha512/sha1）；注册表 ECDSA 签名（dist.signatures，签名串 `${name}@${version}:${integrity}`，密钥端点 `/-/npm/v1/keys`）；`npm audit signatures` 审计 | docs.npmjs.com/cli/v9/configuring-npm/package-lock-json；docs.npmjs.com/about-registry-signatures；docs.npmjs.com/verifying-registry-signatures |
| F3 | go.sum 校验下载 hash，不匹配报 security error 且不装缓存；checksum DB = sum.golang.org 透明日志（Trillian Merkle 树），使不可信代理成为可能；GOSUMDB/GONOSUMDB/GOPRIVATE 可关 | go.dev/ref/mod#go-sum-files、#checksum-database、#authenticating |
| F4 | apt：Release 签名 + Packages hash 链自动校验；无包级签名；未签名仓库默认拒绝；不防主服务器被攻破 | manpages.debian.org/bookworm/apt/apt-secure.8.en.html |
| F5 | dnf：gpgcheck/repo_gpgcheck/localpkg_gpgcheck/gpgkey 配置项；gpgcheck 默认 False | man7.org/linux/man-pages/man5/dnf.conf.5.html |
| F6 | Authenticode：embedded signature（PE 非执行段内嵌）与 catalog files（对象外清单+签名，detached signature）；验证发布者身份 + 签名后内容未变 | learn.microsoft.com/en-us/windows-hardware/drivers/install/authenticode |
| F7 | notarization：notary 扫描 + ticket 在线发布/可 staple；Gatekeeper 首启查证并呈现对话框；macOS 10.15 起 Developer ID 分发包强制 | developer.apple.com/documentation/security/notarizing-macos-software-before-distribution（JSON API 取文） |
| F8 | TUF：targets 元数据带 hash+length；校验完成前不交付文件；root 离线密钥、threshold、委托链 | theupdateframework.github.io/specification/latest/（§4.5、§5.7、§1.5.2） |
| F9 | cargo：index 条目 `cksum` = .crate 的 SHA-256；无索引签名机制（changelog 1.86~1.99 未见）；.cargo-checksum.json 官方明示非安全机制 | doc.rust-lang.org/cargo/reference/registry-index.html；doc.rust-lang.org/nightly/cargo/CHANGELOG.html（1.97 条目） |
| F10 | OSS 三种校验：ETag（PutObject 对象 = MD5，官方不建议作完整性校验）/ Content-MD5（上传服务端比对，分片不适用）/ crc64（服务端计算返回客户端比对，全上传方式支持） | help.aliyun.com/zh/oss/user-guide/data-verification |
| F11 | OSS Go SDK：SignURL（GET/PUT 签名 URL）、GetObjectToFileWithURL、EnableCRC/EnableMD5、CheckDownloadCRC、VerifyObjectStrict | pkg.go.dev/github.com/aliyun/aliyun-oss-go-sdk/oss |
| F12 | OSS 服务端签名授权（STS + 签名）为官方模式（文档聚焦上传直传） | help.aliyun.com "Server-side signing for direct upload" |
| F13 | OmniCraft 现状：上传验证仅大小/类型；下载为签名 GET URL JSON 响应；无 hash 存证 | backend/internal/service/oss_service.go、backend/internal/pkg/aliyun/oss.go、content_download_test.go |
| F14 | ADR-0002：L4 = 签名 grant + 动作预览 + 逐级确认 | docs/adr/0002-client-agent-expansion-and-authenticity.md |
| F15 | 开源 agent/mod 生态无签名级下载校验先例（首轮结论） | docs/working/2026-08-03-agent-reference-oss-research.md（§A 小结 5、§B 小结 3） |

## 7. 调研缺口与标注说明

- **OSS 下载侧签名 URL 官方文档页**：阿里云帮助中心改版后多路径 404（user-guide/developer-reference/document_detail 三种形态均尝试），以 SDK 文档 + 项目既有实现代替（F11/F13），如需官方页可后续从 SDK 文档页内链回找。
- **Apple 文档**：官网页面需 JS 不可抓取，改经官方 JSON API 端点获取正文（F7）。
- **推断项汇总**：§2.2 npm 安装期完整性校验阻断性；§2.6 Authenticode 普通应用非强制；§3.1 时机归纳；§4.2 SDK 自动比对行为；§4.4 CRC64 非防篡改；§5.1 #3 透明度日志不采用、#6 降级模式；§5.4 落地顺序。
- 未抓取项：Go modules checksum DB 的协议细节（sum.golang.org 查询格式，go.dev/ref/mod 表格已有，未逐条展开）；dnf 发行版默认值（以 man 为准）。
