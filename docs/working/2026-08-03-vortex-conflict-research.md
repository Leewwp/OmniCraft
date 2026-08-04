# 2026-08-03 Vortex 冲突检测专项调研

> **创建日期**: 2026-08-03
> **预计失效日期**: 2026-10-03
> **用途**: wayfinder 机票 #9「Vortex 冲突检测专项调研」的 resolution 落盘；作为「L4 动作集」ticket 的决策输入（ADR-0002）
> **上游调研**: `2026-08-03-agent-reference-oss-research.md` 候选 B-1（Vortex）的首轮结论与本调研的缺口清单
> **调研方法**: 一手来源——GitHub API（repo 元数据、master 文件树）+ `raw.githubusercontent.com` 逐文件抓取源码 + 仓库内官方文档（`docs/`）；所有行号为 2026-08-03 master 分支快照实测，非推测
> **仓库状态**: Nexus-Mods/Vortex，TypeScript + Electron，最近推送 2026-08-03，license 字段 GPL-3.0（GitHub API 核实）

---

## 结论速览（TL;DR）

| # | 问题 | 结论 |
|---|------|------|
| 1 | 冲突检测算法 | **两层机制**：① 文件级冲突 = 扫描 staging 全量文件构建「路径 → 拥有者 mod 列表」map，拥有者 >1 即为冲突（`mod-dependency-manager/src/util/conflicts.ts`），按 mtime 给出 before/after 建议；② 部署覆盖 = **确定性 last-write-wins**：按优先级升序激活 mod，输出路径为 key 的部署 map 后写覆盖先写（`LinkingDeployment.ts:383`），顺序由用户规则 + DFS 拓扑排序决定 |
| 2 | 游戏目录发现 | **注册表 + 商店 + 回调 + 全盘搜索 + 手动** 五通道。Windows registry 查询走 `winapi.RegGetValue`（`GameStoreHelper.ts:98`）；每游戏一个扩展（`extensions/games/game-*`）声明 `queryArgs`/`queryPath`/`requiredFiles`；兜底 `searchDiscovery` 按可执行文件名全盘递归搜索；用户可手动指定路径（`pathSetManually` 优先） |
| 3 | 部署/备份回滚 | 差异部署：`diffActivation` 算 added/removed/sourceChanged/contentChanged，只动变化文件；link 前把目标原文件 rename 为 `<path>.vortex_backup`（`LinkingDeployment.ts:783`），卸载/回滚时 rename 回来（`:737`）；hardlink 因 inode 语义天然可恢复（`canRestore()=true`）；部署清单 `vortex.deployment.json` 落在游戏目录 |
| 4 | 下载完整性 | **确认首轮结论，附修正**：无「对照期望值」的完整性校验。下载后确实算 MD5（`postprocessDownload.ts`），但仅用于 Nexus 元数据匹配（modmeta-db）与集合安装的下载匹配（`md5Hint`），**失败也不阻塞**（仍标 finished）；HTTP 层 probe（size/ETag/ranges）只为断点续传；服务端 schema 有 `md5` 字段（"for verification"）但客户端未使用 |
| 5 | License | GPL-3.0（GitHub API + `LICENSE.md` 核实）。代码级复用不可行（传染性）；模式级借鉴可行（算法思想不受版权约束），实现须 clean-room |

---

## Q1. 冲突检测机制

### 1.1 机制总览：检测与执行是两条独立的链

Vortex 的「冲突检测」不是一个算法，而是**检测层（只读报告）**与**执行层（确定性覆盖）**的分离：

- 检测层回答「哪些文件被多个 mod 拥有、哪些 mod 之间互相覆盖」→ 展示给用户 + 游戏启动前阻塞
- 执行层回答「最终哪个文件生效」→ 纯确定性：mod 优先级（部署顺序）升序激活，同一输出路径后写覆盖先写

### 1.2 检测层：文件级冲突（源码证据）

**入口**：`extensions/mod-dependency-manager/src/index.tsx:739` `checkConflictsAndRules()`
- 触发：mod 状态/启用变更后 debounce 触发（`:1310`）；`game.mergeMods === false` 时直接跳过（各 mod 部署到独立子目录，无冲突可能，`:757-765`）
- 结果存 `state.session.dependencies.conflicts`（per-mod 的 `IConflict[]`），并在 `state.session.dependencies` 更新覆盖关系

**算法**：`extensions/mod-dependency-manager/src/util/conflicts.ts`

1. `getAllFiles()`（`:95`）：对所有 enabled + installed 的 mod，turbowalk 遍历 staging 目录下每个文件，构造 `IFileMap`：`{ [绝对路径小写化]: [{mod, relPath, time}] }`。`ABSOLUTE_PATHS = true`（`:18`，注释说明用绝对路径才能跨 mod type 检测冲突）
2. `getConflictMap()`（`:167`）：拥有者数量 >1 且未被 blacklist（`isBlacklisted`）的文件 = 冲突文件；对每个冲突文件做**两两 mod 配对**，累加 `conflicts[lhsId][rhsId].files`，并基于两侧文件 mtime 推断 `suggestion`（`"before"`/`"after"`/`null`，`:179-184` —— 时间戳更早的一方建议先加载）
3. 输出 per-mod 的冲突清单 `{otherMod, files, suggestion}`（`IConflict`，`types/IConflict.ts`）

**消费方**：
- `extensions/mod-dependency-manager/src/unsolvedConflictsCheck.ts`：游戏启动前检查是否存在**未解决**（无规则、无 fileOverride）的冲突 → 弹「Unresolved Conflict」对话框并可阻塞启动（`ProcessCanceled`）
- 依赖管理器 UI（`ConflictGraph.tsx`、`OverrideEditor.tsx`）与 mod 列表冲突徽标
- 自动消解：`addFileOverrides`/`updateOverrides`（`index.tsx:1309-1330`）—— 对冲突文件自动为一方设置 `fileOverrides`（「此文件的文件始终由本 mod 提供」）

### 1.3 执行层：确定性覆盖（源码证据）

- **部署 map 的 key = 输出路径**：`src/renderer/src/extensions/mod_management/LinkingDeployment.ts:379-389`（`activate()`）：每个文件写入 `newDeployment[输出路径] = {relPath, source, target, time}`，注释原文 "mods are activated in order of ascending priority so overwriting is fine here" —— **同路径后激活者直接覆盖先激活者**，map 里只留最终赢家
- **激活顺序 = mod 优先级升序**：`src/renderer/src/extensions/mod_management/modActivation.ts:46` `deployMods(..., mods: IMod[])` —— 注释 "sorted from lowest to highest priority"；`for` 循环逐 mod 调 `method.activate()`
- **优先级从哪来**：规则图拓扑排序 `extensions/mod-dependency-manager/src/util/topologicalSort.ts`（DFS 遍历 `before`/`after` 规则产出顺序）；规则含环时 `showCycles` 报错并拒绝部署（`mod_management/index.ts:222-249`）
- **fileOverrides 参与部署**：`modActivation.ts:81-90` —— 拥有 fileOverrides 的 mod，其 override 文件加入 `BlacklistSet`（跳过），从而不被其它 mod 覆盖（赢家语义）
- **合并（merge）特殊路径**：`mod_management/modMerging.ts` —— 对需要合并的 archive/文件（如 injector 类 mod），用 **MD5 逐文件比较**决定差异后再打包合并，产出 `__merged` 目录（`:21`），合并结果仍走同一条 LinkingDeployment 链（`modActivation.ts:96-101`）

### 1.4 关键推断（标注）

- **推断**：检测层不做任何覆盖计算（不回答"谁赢"），只报告「重叠存在」；回答"谁赢"完全由执行层的部署顺序决定。因此 Vortex 的 UI 冲突提示与实际生效文件可能不一致（检测报告所有重叠，执行只认顺序）——这是源码结构支持的推断（两层数据流完全独立）。
- **事实**：冲突检测是 O(文件数 × mod 数) 的全量磁盘扫描，无索引/增量（`conflicts.ts` 每次重建 `IFileMap`；`index.tsx:1309` 有 200ms debounce 与活动抑制 `shouldSuppressUpdate`）。

---

## Q2. 游戏目录发现/注册机制

### 2.1 五通道发现（源码证据）

**调度器**：`src/renderer/src/extensions/gamemode_management/util/discovery.ts`

| 通道 | 源码锚点 | 机制 |
|------|----------|------|
| 商店查询 | `quickDiscovery()` `:287` → `queryByArgs()` `:149` → `GameStoreHelper.find(queryArgs)` | 按 `game.queryArgs`（steam/gog/epic/xbox 商店 ID）查已安装商店 |
| Registry 查询 | `GameStoreHelper.ts:98` `registryLookup()` | `winapi.RegGetValue(hive, path, key)`；查询串格式 `hive:path:key`（如 `HKEY_LOCAL_MACHINE:Software\\...:Installed Path`） |
| 游戏扩展回调 | `queryByCB()` `:183` | 调 `game.queryPath()` —— 扩展可自行实现任意查找（含查配置文件等） |
| 全盘文件名搜索 | `searchDiscovery()` `:653` → `walk()` `:375` | 兜底：递归搜索用户指定路径，按 `requiredFiles` 的 basename（如 `SkyrimSE.exe`）匹配，命中后校验目录（`testApplicationDirValid` → `verifyToolDir` `:450` 逐个 stat 必需文件） |
| 用户手动 | `:296` `pathSetManually` | 用户手动指定路径时**不覆盖**（`"don't override manually set game location"`），只补充 store 识别（`updateManuallyConfigured`） |

- 发现结果存 `state.settings.gameMode.discovered[gameId]`（`IDiscoveryResult {path, executable, store}`）
- 候选排序：优先同 store（`discovery.ts:170-179`），否则按 store 的 `priority`
- 目录有效性校验：`requiredFiles` 逐一 stat（`verifyToolDir` `:450`）；游戏扩展还可用 `queryModPath`（如 Skyrim 的 `"Data"`）声明 mod 子目录

### 2.2 Games extension 机制（源码证据）

- 每游戏一个扩展：`extensions/games/game-*`（当前 ~90 个，`extensions/games/` 目录核实），通过 `context.registerGame({...})` 注册 `IGame` 描述
- 实例：`extensions/games/game-skyrimse/src/index.js:147-160`
  ```js
  queryArgs: {
    steam: [{ name: "The Elder Scrolls V: Skyrim Special Edition", prefer: 0 }],
    xbox: [{ id: MS_ID }], gog: [{ id: GOG_ID }], epic: [{ id: EPIC_ID }],
    registry: [{ id: "HKEY_LOCAL_MACHINE:Software\\Wow6432Node\\Bethesda Softworks\\Skyrim Special Edition:Installed Path" }],
  },
  queryModPath: () => "Data",
  executable: () => "SkyrimSE.exe",
  requiredFiles: ["SkyrimSE.exe"],
  mergeMods: true,
  ```
- 接口定义：`src/renderer/src/types/IGame.ts`（`queryPath`/`queryArgs`/`requiredFiles`/`executable`/`getModPaths`/`mergeMods`/`getGameVersion` 等）
- 反例说明 mergeMods 语义：`game-palworld` 声明 `mergeMods: false`（各 mod 独立子目录，无文件冲突）；`mergeMods: true` 的游戏（如 Skyrim）才有冲突检测
- 官方文档佐证：仓库内 `docs/mod-management/EXTERNAL-CHANGES.md`（部署/清单机制的官方说明）；`AGENTS-DIRECTORIES.md` 确认 `extensions/games/` 为游戏扩展目录

### 2.3 对 OmniCraft 的映射

- Vortex 的 registry 通道依赖 Windows 专用库（`winapi-bindings`）；macOS/Linux 下依赖商店查询与回调
- 「多商店 + registry + 文件名搜索 + 手动覆盖」的分层结构可直接映射为 OmniCraft 的「常见路径扫描 + 用户配置 + 可执行文件名验证」

---

## Q3. 部署机制与 `.vortex_backup` 备份回滚

### 3.1 差异部署（源码证据）

`src/renderer/src/extensions/mod_management/LinkingDeployment.ts`（`LinkingActivator` 抽象基类，所有基于文件链接的部署方法的公共实现）：

- `prepare()`（`:120`）：从上次部署清单 `lastDeployment: IDeployedFile[]` 重建 `previousDeployment`（按归一化输出路径为 key 的 map）
- `finalize()`（`:154`）→ `diffActivation()`（`:802`）：
  ```
  added          = after − before
  removed        = before − after
  sourceChanged  = 两侧同 key 但 source（mod）不同
  contentChanged = 两侧同 key 同 source 但 mtime 不同
  ```
  只对四类差异执行操作（`finalize()` 内 removed/sourceChanged/contentChanged 先 `removeDeployedFile`，added 再 `deployFile`，`:177-292`），**未变化的文件零操作** —— 这就是差异部署
- 并发：unlink 50 并发、link 100 并发（`:215, 270`）；失败文件计数上报错误通知（`:294-308`）
- 目录清理：`postLinkPurge()`（`:817`）—— 递归删除**带 tag** 的空目录；tag 文件 `__folder_managed_by_vortex`（Win）/`.__folder_managed_by_vortex`（类 Unix）（`:64-65`），旧名 `__delete_if_empty`；`directoryCleaning: "all" | "tag"` 模式（`:201, 831-837`）

### 3.2 hardlink / symlink 具体实现

| 方法 | 源码 | 关键实现 |
|------|------|----------|
| hardlink_activator | `src/renderer/src/extensions/hardlink_activator/index.ts`（priority 5） | `linkFile` = `fs.linkAsync`（EEXIST → 删后重链，`:294-302`）；`isLink` = `nlink > 1 && ino === ino`（`:308-330`）；**`canRestore() = true`**（`:332`，硬链接下源被删文件数据仍存在，可恢复）；`purgeLinks` 用 inode/NTFS fileID 匹配删除游戏目录中的链接（`:221-292`，`linkCount > 1 && idStr` 集合求交）；`isSupported` 要求 staging 与游戏同盘（dev 比较，`:112`）+ canary 实测能否建链接（`:160-201`） |
| symlink_activator | `src/renderer/src/extensions/symlink_activator/index.ts`（priority 10） | `fs.symlinkAsync`；Windows 需管理员权限（`:92`）；Gamebryo 系游戏显式不兼容（`:65-75`）；`canRestore()` 为 false（symlink 源删后不可恢复） |
| move_activator / null_activator | `src/renderer/src/extensions/move_activator/index.ts`、`null_activator/index.ts` | 真实文件移动 / 空实现（不部署） |

### 3.3 `.vortex_backup` 备份/回滚机制（源码证据）

- 常量：`BACKUP_TAG = ".vortex_backup"`（`LinkingDeployment.ts:33`）
- **部署前备份**：`deployFile()`（`:757-800`）—— `replace=false` 时若目标文件已存在且不是正确链接 → `fs.renameAsync(输出路径, 输出路径 + ".vortex_backup")` 把**游戏原文件**改名备份，再建链接（`:783`）；若备份失败（非 ENOENT）则**中止**（防数据丢失，`:786-791`）
- **回滚恢复**：`removeDeployedFile(restoreBackup)`（`:705-755`）—— unlink 后 `fs.renameAsync(输出路径 + ".vortex_backup", 输出路径)`（`:737`）；`restoreBackup()`（`:933-1007`）—— 递归 purge 时把残留 `.vortex_backup` 改回原名；EEXIST（用户/其它管理器已放回文件）→ 弹对话框让用户选择「Keep Existing File / Restore Vortex Backup」（`:949-975`）
- **部署清单**：`util/activationStore.ts` —— 主清单 `vortex.deployment.json`（落在**游戏 mod 目录**，`:380`），staging 目录有 msgpack 备份；结构 `IDeploymentManifest {version, instance, files: IDeployedFile[]}`；`instance` 字段用于多 Vortex 实例交叉检测（跨实例 purge 需用户确认，`:110-140`）
- **外部变更检测**：`LinkingDeployment.externalChanges()`（`:486-592`）→ 变更分类 `srcdeleted`（源删，hardlink 可恢复）/ `deleted`（链接被删）/ `refchange`（内容被替换）→ 部署前 `dealWithExternalChanges`（`util/deploy.ts:256`）弹对话框处理；官方文档 `docs/mod-management/EXTERNAL-CHANGES.md` 完整描述该流程（含 `valchange` 已注释不展示的说明）
- **部署串行化**：`withActivationLock`（`util/deploy.ts:236, 384`）—— 防并发部署竞态

---

## Q4. 下载完整性校验（确认首轮结论 + 修正）

### 4.1 结论：**确认首轮「未发现明确实现」**——Vortex 不做「对照期望值」的下载完整性校验；但有三个重要的修正性事实

### 4.2 事实一：下载后确实计算 MD5，但用途是元数据识别，且失败不阻塞

- `src/renderer/src/extensions/download_management/util/postprocessDownload.ts`：
  - `finalizeDownload()` 下载完成后调 `api.genMd5Hash(filePath)`（流式 MD5，主进程实现 `src/main/src/hash/compute.ts` `hashFileStream`），结果 `setDownloadHashByFile(md5sum, numBytes)`（`:31-43`）
  - **注释原文**："If MD5 calculation fails, still mark download as finished"（`:55-58`）—— 计算失败不拦截下载
- MD5 消费方（均为识别/匹配，非校验）：
  - `queryDLInfo.ts`：`lookupModMeta({fileMD5, fileSize})` —— 用 hash 到 modmeta-db / Nexus API 反查「这是哪个 mod」（Nexus 侧的核心身份标识，安装后 `mod.attributes.fileMD5`）
  - `InstallManager.ts:296`：`findDownloadByReferenceTag` —— 集合安装时用 `md5Hint` 把 mod 引用匹配到具体下载文件（`:3130-3160` 同样模式）
  - `conflicts.ts:34`：`fileMD5` 参与 mod 去重/匹配
- 主进程下载器：`src/main/src/downloading/downloader.ts` —— HTTP probe（`probeUrl`：size/ETag/Accept-Ranges）仅用于断点续传与分块（`:106-124`），**无内容校验**；`resolver.ts` 只是 URL→endpoint 归一化

### 4.3 事实二：服务端数据模型其实支持 md5 校验，客户端没接

- `packages/nexus-api-v3/schema/openapi.yaml:2722-2725`：`CollectionManifestModSource.md5` —— "An MD5 hash of the file **for verification**"（集合清单 schema 自带 md5 字段）
- 即 Nexus 平台的数据模型支持完整性验证，但 Vortex 客户端代码（下载链路）没有把计算出的 MD5 与服务器期望值做比较

### 4.4 事实三：Vortex 对「下载损坏」的防护是间接的

- 安装环节失败（解压错误等）会中止安装（`InstallManager.installInner` 错误路径），损坏文件通过安装失败间接暴露，而非下载时校验

### 4.5 对 OmniCraft 的含义

- 「签名 grant」自研路线不受影响（首轮结论不变）；Vortex 提供的是「MD5 反查元数据」的模式可借鉴（低成本 mod 识别），但不构成完整性校验先例
- 若 OmniCraft 要完整性校验，需自研（下载后对照服务器哈希/签名），Vortex 无参考实现

---

## Q5. License 与可借鉴程度

### 5.1 事实

- GitHub API `license` 字段：`gpl-3.0`（2026-08-03 核实）；仓库根 `LICENSE.md` 为 GNU GPL v3（全文 32KB，抓取核实）
- 同类 mod 管理器（Modrinth App GPL-3.0-only、GDLauncher GPL-3.0）同为 GPL 系 —— 首轮调研结论一致

### 5.2 对 OmniCraft 的约束（推断与事实分列）

- **事实（法律面）**：GPL-3.0 传染性 —— 分发基于 Vortex 代码（含逐行翻译）的衍生作品必须以 GPL-3.0 开源；与 OmniCraft 现有闭源/专有路线冲突，**代码级复用与逐行移植不可行**
- **推断（合规面，非法律意见）**：算法与设计思想（如"文件→拥有者 map 检测冲突"、"last-write-wins 按优先级部署"、"先备份再链接"）不受版权保护，可 clean-room 重实现；借鉴时避免：直接复制源码片段、保留相同注释/标识符、照搬文件结构
- **可行借鉴面**（模式层面）：
  1. 「检测层/执行层分离」的冲突架构（报告重叠 + 确定性覆盖）
  2. 冲突 UI 语义（conflict badge、before/after 建议、游戏启动前阻塞）
  3. 部署清单（`vortex.deployment.json`）＋ `.vortex_backup` 先备份再链接的差异部署模式
  4. 「requiredFiles 验证目录 + 多通道发现 + 手动覆盖优先」的游戏定位模式
  5. MD5 反查元数据（modmeta）的识别模式

---

## 对 OmniCraft「一键安装」动作集的事实性建议

> 面向 wayfinder「L4 动作集」ticket；动作集现有成员（ADR-0002）：下载、解压、移动、建目录、读写配置

| 动作 | 建议 | 依据 |
|------|------|------|
| 冲突检测 | **可借鉴，建议实现为「两步」**：① 静默检测层（文件→拥有者 map，拥有者>1 即冲突，全量扫描即可，无需索引）② 执行层 last-write-wins（按安装顺序覆盖）。两步分离与 Vortex 一致，OmniCraft 不需要规则图/拓扑排序这类重设施 | Q1 全部源码 |
| 冲突 UX | 借鉴「游戏启动前阻塞 + 报告（before/after 建议）」；Vortex 的规则编辑器（依赖图）对 OmniCraft 一键安装**不适用**（太重） | Q1.2 |
| 游戏目录发现 | 借鉴「多通道 + 手动优先」：常见路径扫描（requiredFiles 验证）+ 用户手动指定 + 手动不覆盖；registry 通道仅 Windows 有意义，桌面端若先发 macOS 可省略 | Q2 |
| 差异部署 | **可借鉴（高价值）**：部署清单（JSON 记录 relPath→source+time）+ diff 四分类（added/removed/sourceChanged/contentChanged）+ 只动差异文件；hardlink 优先（同盘时）、symlink 兜底；staging/game 目录分离 | Q3.1-Q3.2 |
| 备份回滚 | **可借鉴（高价值）**：`<path>.vortex_backup` 先备份再链接 + 回滚 rename 恢复 + EEXIST 冲突对话框；对「一键安装回滚」是现成且验证充分的模式 | Q3.3 |
| 外部变更检测 | 可借鉴：部署前对比清单检测 srcdeleted/deleted/refchange 三类，弹窗让用户决策；注意 Vortex 官方文档（EXTERNAL-CHANGES.md）是这套逻辑的唯一权威说明 | Q3.3 |
| 下载完整性校验 | **不适用（Vortex 无先例）**：保持自研（签名 grant / 服务器哈希对照）；Vortex 的「下载后 MD5 反查元数据」模式可单独借鉴用于 mod 识别 | Q4 |
| 代码复用 | **不适用**：GPL-3.0 传染，所有借鉴仅限模式层面（clean-room） | Q5 |

### 事实 vs 推断清单

- **事实**：Q1~Q4 全部源码锚点（master 分支 2026-08-03 快照实测行号）；MD5 计算失败不阻塞（`postprocessDownload.ts` 注释原文）；服务端 md5 字段存在但客户端未对照；GPL-3.0
- **推断**：Q1.4 的「检测与执行两层独立」架构解读；Q5.2 的合规性分析（非法律意见）；「Vortex 安装环节失败间接暴露损坏」的因果解读（Q4.4）
- **未验证**：真实 Windows registry 行为（无 Windows 环境实测，仅源码）；`gamebryo-plugin-management` 的插件级排序（load order）未纳入本次范围（与文件冲突正交，插件排序是另一个维度）

### 调研缺口（供后续 ticket）

1. `gamebryo-plugin-management` 的插件 load order 排序算法（ESP/ESM 主从与覆盖规则）——若 OmniCraft 后续要支持 Bethesda 系游戏需单独调研
2. `modmeta-db`（Nexus 的 mod 元数据离线库）的匹配细节 —— 若要做「文件 hash → mod 识别」需深挖
3. Vortex 的 `__merged` 合并（modMerging）在多文件合并时的取舍规则 —— 目前仅确认其存在与 MD5 比较机制
