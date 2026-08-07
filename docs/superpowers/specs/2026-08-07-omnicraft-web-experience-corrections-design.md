# OmniCraft Web 体验修复与数据契约收口规格

> 日期：2026-08-07  
> 状态：confirmed  
> 设计输入：2026-08-07 本地 UI/功能审计、代码与浏览器实测、本轮用户确认决策  
> 适用范围：Web 页面外壳、内容详情浮层、内容系列导航、IP 访问历史、社区互动、收藏集 cutover

## Problem Statement

OmniCraft 当前 Web 产品已经具备推荐流、原创区、二创分区、IP 库、内容详情浮层、内容系列、社区互动和收藏集等主要能力，但这些能力在本地真实运行时暴露出一组相互关联的体验与数据契约问题：页面外壳尺寸和背景不统一，品牌入口没有返回推荐流，内容详情浮层缺少从触发卡片自然放大的共享元素转场，系列目录会离开浮层上下文，部分响应式布局溢出，最近访问 IP 只存在匿名本机状态，社区互动状态无法稳定回显，筛选和排序控件缺少一致性，收藏状态仍被已经过时的 `favorites` 模型影响。

这些问题会让用户感到同一个产品由多套不一致的页面与状态拼接而成，也会让前端显示状态与数据库真实关系脱节。旧收藏接口和旧 `favorites` 表仍被后端详情查询、推荐画像、双写逻辑与运维工具引用，使收藏集已经成为主模型后仍无法安全完成 cutover。

本规格要解决的不是一次局部换色，而是把用户可见体验、当前查看者私有状态和后端事实源重新对齐，使共享组件在推荐流、分区页、IP 详情页和内容详情浮层中保持一致，并为后续拆票、实现和发布验证建立稳定合同。

## Solution

统一 Web 页面外壳的宽度、背景和品牌入口，使推荐流成为点击“万象工坊”后的稳定落点；修复侧边栏、筛选、关注、内容封面和内容系列导航的视觉及响应式问题。

内容详情浮层采用以 FLIP 几何计算为可靠核心的共享元素转场，支持环境良好时使用 View Transition API 渐进增强，并在触发卡片不可定位时优雅降级。浮层中的内容系列目录改为可滚动、可键盘访问的章节选择器，章节切换继续留在同一个浮层导航栈中。

社区互动读取合同明确区分公开反应聚合与当前登录用户的查看者反应；收藏状态明确以收藏成员关系为唯一事实源。匿名最近访问 IP 保留本机记录，登录后幂等合并到独立的 IP 访问历史模型。

完成收藏集 cutover：移除仓库内旧收藏接口与 `favorites` 表的运行时依赖，在云端访问日志确认无旧客户端调用后，通过受控的 forward-only 迁移删除旧表。云端现有测试、演示和旧收藏数据允许丢弃，不要求为旧表数据提供业务保留承诺。

## User Stories

1. As a visitor, I want the public sidebar and page body to use a coherent background, so that the site feels like one continuous product surface.
2. As a desktop visitor, I want the brand mark and the main page shell to share a clear alignment grid, so that navigation and content do not look horizontally disconnected.
3. As a mobile visitor, I want the mobile brand entry to follow the same destination as the desktop brand entry, so that device choice does not change navigation meaning.
4. As any visitor, I want clicking “万象工坊” to open the recommendation feed, so that the product brand consistently returns me to the primary discovery surface.
5. As a recommendation-feed visitor, I want content cards to open the shared content detail overlay without losing my feed context, so that I can explore and return naturally.
6. As a content explorer, I want a card cover to expand smoothly into the overlay cover, so that the opening interaction preserves visual continuity.
7. As a content explorer, I want closing the overlay to visually return toward the triggering card when that card can still be located, so that the exit feels connected to the entry.
8. As a content explorer, I want the overlay to use a centered scale-and-fade fallback when the trigger is offscreen or unavailable, so that opening and closing remain graceful in every entry path.
9. As a user on a browser with View Transition support, I want enhanced transitions without losing compatibility with other browsers, so that better platform capability improves rather than gates the experience.
10. As a user who prefers reduced motion, I want the overlay transition to become a short opacity change, so that the interaction remains comfortable and understandable.
11. As a user opening media content, I want the overlay to show an explicit media loading state, so that blank or partially composed content is not mistaken for a broken page.
12. As a user opening media content, I want the main overlay content to appear in sync with the completed cover load, so that text does not visibly arrive before the visual anchor.
13. As a user whose cover image fails to load, I want a stable placeholder followed by usable content, so that a media failure does not block the entire detail experience.
14. As a content reader, I want the cover and the detail body to share the same content width, so that the overlay frame is visually aligned.
15. As a content reader, I want overlay controls and media to remain inside their borders at desktop, tablet and mobile widths, so that no action becomes clipped or visually broken.
16. As a series reader, I want previous, directory and next actions to remain within the series navigation container, so that chapter navigation stays usable at every breakpoint.
17. As a series reader, I want to open a scrollable chapter directory inside the content detail overlay, so that I do not have to leave the context I am reading.
18. As a keyboard user, I want to open, navigate and close the series directory using standard keyboard controls, so that chapter selection is fully accessible.
19. As a series reader, I want choosing a chapter from the directory to behave like choosing the next chapter, so that all chapter transitions use one overlay navigation model.
20. As a series reader, I want browser back and overlay back to return through previously opened chapters, so that I can retrace my reading path.
21. As a direct-link visitor, I want the full series and full content pages to remain available, so that shareable URLs and standalone browsing continue to work.
22. As an anonymous IP browser, I want my recent IP visits to remain available on the same device, so that I can quickly revisit what I explored before signing in.
23. As a user who signs in after anonymous browsing, I want my local recent IP visits merged into my account history, so that signing in does not discard my recent context.
24. As a signed-in user, I want recent IP visits to come from an account-bound source, so that the history can remain consistent across sessions and devices.
25. As a returning user, I want duplicate IP visits collapsed to the most recent visit, so that the recent list is useful rather than repetitive.
26. As a user experiencing a temporary merge failure, I want local history retained until the server confirms success, so that my recent IP visits are not lost.
27. As an IP community participant, I want a visible “发起讨论” action in the compact discussion area, so that I can start a discussion without first navigating to the full discussion list.
28. As an IP community participant viewing an empty discussion area, I want an empty-state call to action, so that an empty community still explains the next action.
29. As an anonymous or interaction-restricted visitor, I want discussion actions to respect authentication and reputation rules, so that the UI does not promise an unavailable operation.
30. As a creator visitor, I want the not-following state to be visually prominent, so that the primary follow action is easy to discover.
31. As a follower, I want the following state to be visually subdued and reveal “取消关注” on hover or focus, so that state and reversal are understandable.
32. As a user who likes content, I want the liked state to remain after refresh, so that the UI reflects my stored action.
33. As a user who dislikes content, I want the disliked state to remain after refresh, so that the UI reflects my stored action.
34. As a user, I want like and dislike to remain mutually exclusive, so that one target has only one reaction from me.
35. As a user, I want clicking my current reaction again to remove it, so that I can return to a neutral state.
36. As a user, I want switching between like and dislike to update atomically, so that counts and my state never show an impossible combination.
37. As an anonymous visitor, I want to see public reaction totals without receiving another user’s private reaction state, so that aggregation and viewer-specific data remain separated.
38. As a signed-in visitor, I want the API to return my viewer reaction separately from public aggregates, so that the interface can render my state without scanning other users’ records.
39. As a content browser, I want selected filters to use one consistent colored-pill treatment across the IP library, fanwork zone and original zone, so that selection semantics look the same everywhere.
40. As a keyboard or screen-reader user, I want selected filters to expose their state semantically, so that selection is not communicated by color alone.
41. As a user changing content sort order, I want an accessible custom listbox that opens without covering its trigger, so that options remain readable and the current control remains understandable.
42. As a user on different platforms, I want the sort control to prioritize usability and visual quality over pixel-identical popup placement, so that native platform differences do not force a poor interaction.
43. As a user who has added content to any collection, I want the content detail action to display “已收藏”, so that the visible state matches my collection membership.
44. As a user who has not added content to any collection, I want the content detail action to offer adding it to a collection, so that the next action is clear.
45. As a user who opens the collection picker, I want each collection’s membership state to agree with the detail-level “已收藏” state, so that two views of the same fact cannot contradict one another.
46. As a user with content in multiple collections, I want the global “已收藏” state to remain true until the content is removed from every active collection, so that the state represents collection membership rather than one special collection.
47. As a recommendation user, I want collection membership to contribute to my profile without depending on the legacy favorites table, so that recommendations use the current domain model.
48. As an operator, I want the application to stop reading and writing the legacy favorites table before it is dropped, so that the cutover does not create runtime failures.
49. As an operator, I want old favorites endpoints removed only after repository callers and cloud access logs show no supported client dependency, so that endpoint removal is deliberate and observable.
50. As an operator, I want legacy test and demo favorite rows to be disposable during the cloud cutover, so that an unused compatibility model does not delay the migration.
51. As an operator, I want historical migrations left immutable and a new forward-only cleanup migration added, so that database history remains auditable.
52. As a maintainer, I want tests, seed tooling, reconciliation tooling and documentation updated with the cutover, so that removed concepts do not survive as misleading maintenance contracts.
53. As a maintainer, I want each UI change to use internationalized strings and established design tokens, so that fixes do not introduce hard-coded copy or visual drift.
54. As a maintainer, I want UI work and schema cutovers split into appropriately scoped tickets, so that low-risk polish and destructive migration work receive proportionate review.

## Implementation Decisions

1. The recommendation feed remains the primary discovery surface. Both desktop and mobile brand entries navigate to `/recommend`; the fanwork zone remains available at `/` and is not renamed into the recommendation feed.
2. Header, public sidebar and page content use one shared page-shell width and gutter contract. Implementations must remove independent max-width values that produce visible drift and use design-system tokens for page backgrounds.
3. The public sidebar and adjacent page surface use a coherent background treatment. The selected treatment must follow the visual authority in the UI specification and remain valid in light and dark modes.
4. Filter selected states on the IP library, fanwork zone and original zone adopt the existing colored-pill treatment from the IP library. Selected state must also be exposed through semantic attributes or text, not color alone.
5. The three native sort selects are replaced by one shared accessible sort control built from a trigger and positioned listbox/menu. It must support keyboard navigation, focus return, outside-click and Escape dismissal, sticky-toolbar stacking and viewport collision handling. Cross-platform pixel-identical placement is not required.
6. The content detail overlay keeps a single shared implementation for recommendation, zone, IP and Agent citation entries.
7. Shared-element motion uses manually calculated FLIP geometry as the reliable core. View Transition API support is a progressive enhancement guarded by feature detection; unsupported environments continue to use FLIP.
8. The source geometry is the triggering card cover or media region. The opening motion expands that visual anchor into the overlay cover while the overlay shell progresses on the same timeline. Closing reverses toward the source only when a usable source rectangle remains available.
9. When the triggering element is missing, outside the viewport, detached or cannot be measured, the overlay falls back to centered scale-and-fade. This fallback is also valid for direct programmatic openings without a card source.
10. Motion timing follows the established overlay contract: 300 ms opening, 240 ms closing, with the shared easing curve already defined by the UI specification. Reduced-motion uses a short opacity-only transition.
11. The overlay displays a media loading state inside the final cover geometry. Main detail content becomes visible when cover loading settles successfully; failure displays a stable placeholder and then reveals usable detail content rather than blocking indefinitely.
12. Cover layout fills the same available content width as the body. A height cap may crop or contain media according to content type, but must not shrink the cover container’s horizontal frame.
13. The content series action row may reflow across breakpoints. No control is required to occupy an identical position across platforms, but all actions must remain within the container, meet touch-target requirements and preserve a logical focus order.
14. Inside the overlay, “目录” opens a bounded-height, scrollable and keyboard-accessible chapter selector. Selecting a chapter pushes that content onto the existing overlay navigation stack, exactly like previous/next navigation, and does not navigate to a new page.
15. Standalone series and content routes remain supported for direct URLs. The in-overlay directory behavior does not remove or redefine those public pages.
16. Anonymous recent IP history remains in local storage. Signed-in recent IP history uses a new independent IP visit history model rather than overloading content browse history.
17. The IP visit history model records the account, IP entity and most recent visit time, with one current row per account/IP pair. Repeated visits update recency. The visible recent list preserves the current cap of six entries ordered by latest visit.
18. Login merge is idempotent: local IP visits are upserted into the signed-in user’s server history, duplicates resolve by latest visit time, and local records are cleared only after confirmed server success.
19. The compact discussion area on an IP detail page exposes a create-discussion action for eligible signed-in users, including an empty-state CTA. Existing authentication, ban and reputation interaction guards remain authoritative.
20. FollowButton states are corrected to the established UI contract: not-following is the primary action; following is the restrained outline state; hover/focus on following communicates “取消关注”.
21. Reaction persistence continues to use the existing reactions model. No new viewer-reaction database column or duplicate table is introduced.
22. The reactions model maintains the unique identity `(user_id, target_type, target_id)`. The stored reaction is either `like` or `dislike`; repeating the current reaction removes it, and choosing the opposite reaction updates it atomically.
23. Reaction read responses separate public aggregate data from `viewer_reaction`. For an optionally authenticated request, `viewer_reaction` is `like`, `dislike` or `null`; anonymous responses return `null` and never expose another user’s private state as the current viewer’s state.
24. Public consumers should use aggregate counts rather than requiring the client to scan raw user reaction rows. Existing count fields remain the public source for like/dislike totals.
25. “已收藏” is defined exclusively by collection membership: content is favorited for the current user when it belongs to at least one of that user’s active collections. Membership in deleted collections does not count.
26. The content detail normalizer and UI preserve and render collection membership state. The collection picker and detail action must derive from the same fact and remain consistent after add/remove operations.
27. Legacy favorites endpoints, model, repository operations, service double-write behavior, content-detail lookup and recommendation reads are removed during the collection cutover. The interaction guard currently shared by collection routes must be retained under an accurate generic name rather than accidentally deleted with legacy code.
28. The legacy reconciliation command is retired or redefined so no supported tool reads or writes the removed table. Local seed tooling and tests stop creating or asserting legacy favorite rows.
29. Historical migrations remain unchanged. A new forward-only cleanup migration removes the legacy table only after application code no longer depends on it.
30. The cloud cutover may discard existing legacy favorites, test accounts and demo data. No legacy-to-collection reconciliation completeness requirement applies to that disposable cloud dataset, but a recoverable pre-cutover backup is still required for operational rollback.
31. Repository inspection has found no current frontend runtime calls to the legacy favorites endpoints. Before production removal, cloud access logs must also show no supported client or external script use of the old endpoint family.
32. The safe cutover order is: inspect access logs and capture backup; deploy code that no longer reads/writes the legacy model and removes old routes; run application smoke tests; execute the forward-only table removal; rerun collection, detail and recommendation smoke tests.
33. All new visible strings use the existing internationalization system. All visual changes use design-system tokens and the UI specification is updated before implementation wherever this confirmed behavior changes its current wording.
34. Implementation is split by risk: ordinary visual fixes and non-schema interaction contracts use the light lane; the independent IP history migration and legacy favorites removal use separate heavy tasks with test-first evidence and two-stage review.

## Testing Decisions

1. Tests assert externally visible behavior and stable contracts, not Tailwind class strings, private component state, FLIP helper internals or SQL implementation details.
2. The primary UI seam is the real browser journey through public discovery pages and the shared content detail overlay. Browser tests verify navigation destinations, computed geometry, focus behavior, history behavior, visible state, responsive containment and screenshots.
3. Existing navigation-shell and recommendation-page tests provide prior art for brand routing, Header/sidebar contracts and responsive shell behavior.
4. Existing card-entry and content-detail-overlay tests provide prior art for opening one shared overlay, preserving source context, restoring focus, maintaining a navigation stack and handling reduced motion.
5. Overlay motion tests cover: measurable in-viewport source, source leaving the viewport before close, detached source, direct opening without a source, View Transition support present/absent, reduced motion, successful image load and failed image load.
6. Geometry assertions verify that the cover and body share the same horizontal frame and that series actions remain inside their container at representative mobile, tablet and desktop viewports.
7. Existing series navigation and content-series end-to-end tests provide prior art for previous/next disabled states, multiple memberships and public series visibility. New tests exercise the in-overlay directory as a listbox/menu and verify that chapter selection pushes the overlay stack rather than changing pages.
8. Existing interaction-capability tests provide prior art for authentication and reputation-disabled controls. Discussion, reaction and collection actions retain those capability checks.
9. Reaction API behavior is tested at the HTTP handler/router seam with anonymous and authenticated requests. Tests cover aggregate counts, `viewer_reaction`, create, remove, switch, mutual exclusion and refresh/read-back.
10. IP history is tested at the HTTP seam for recording, recency upsert, per-user isolation, idempotent anonymous merge, duplicate resolution, six-item ordering and failure behavior that preserves local state.
11. Collection membership is tested at the collection/content API seam for zero, one and multiple active collections, removal from one of several collections, removal from the final collection, deleted collections and anonymous viewing.
12. Existing collection-picker and collection end-to-end tests provide prior art for membership display and add/remove behavior. The detail-level “已收藏” state must be asserted in the same user journey.
13. The collection cutover is tested through PostgreSQL migration integration tests using both empty-database and historical-fixture upgrade paths. Historical migration fixtures remain checksum-protected; the latest schema must contain collection tables and the new IP history table, and must not contain the legacy favorites table.
14. Route-contract tests prove the legacy endpoint family is no longer registered while collection endpoints retain their interaction policy.
15. Recommendation service tests prove collection membership alone contributes to cold-start and profile behavior after legacy favorite reads are removed.
16. Seed and maintenance-tool contract tests prove no supported runtime or local setup path requires the removed table.
17. Browser validation saves desktop and mobile screenshots for page-shell alignment, filter states, sort menu, overlay opening/loaded/error states, series directory, discussion CTA, follow states, reaction persistence and collection membership.
18. Final validation includes backend test/build/vet, frontend lint/build, the project verifier, relevant mocked browser contracts, real browser interaction checks and database migration integration tests. Heavy tasks must record the expected failing test before implementation.
19. Production cutover evidence includes the backup identity, cloud access-log query for old endpoint paths, deployed commit, migration identity and post-cutover collection/detail/recommendation smoke results.

## Out of Scope

- Desktop/Tauri behavior, desktop release claims and one-click deploy work.
- Completing Production Readiness Ops-08 or Web Agent Task 6 external-provider smoke gates.
- Redesigning the standalone content series detail page or removing direct content URLs.
- Changing recommendation ranking weights or algorithms beyond removing the legacy favorites source.
- Preserving or reconciling disposable cloud legacy favorites, test users or demo data.
- Editing already-applied historical migration files or historical checksum fixtures to pretend the legacy table never existed.
- Supporting reaction types beyond like and dislike.
- Publishing individual users’ reaction rows as a new social graph feature.
- Synchronizing anonymous IP history across devices before login.
- Replacing content browse history with IP visit history; they remain distinct domain records.
- Requiring the custom sort popup to occupy pixel-identical coordinates on every operating system.
- Restoring deferred Desktop scope or changing its feature flags.

## Further Notes

- The 2026-08-07 audit remains reproducibility evidence for the original symptoms; this confirmed specification is the design input for implementation and supersedes the audit’s unresolved recommendations.
- The UI specification remains the sole visual authority. Sections covering Header/page shell, IP detail discussions, ContentDetailOverlay, SeriesNav, ReactionBar, FollowButton, filters and SortSelect must be reconciled with this confirmed behavior before their implementation tickets edit UI code.
- “收藏成员关系”和“查看者反应” are the required domain terms. “收藏夹” and a database `viewer_reaction` column are not the intended model.
- Source-linkage, collaboration-invites and this work share content-detail, route and translation surfaces. Tickets must publish exact file reservations and serialize overlapping edits.
- Ops-08 Step 5 is complete with approved, API-verifiable RPO/RTO and real staging/archive evidence. Web Agent Task 6 real Provider validation remains blocked by its missing external credentials; that evidence must not be simulated by this work.
- The next phase is ticket decomposition with explicit blocking edges. Low-risk visual corrections should not be bundled into the destructive collection cutover, and the IP history migration must remain independent from legacy favorites removal.
