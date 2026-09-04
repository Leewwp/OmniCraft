# Golden Set 草稿 ↔ 注入清单对账报告（#291）

> v2（2026-09-04 审计修订）：修复 v1 抽样表 query[:50] / rubric[:60] 硬截断；抽样与统计口径不变（seed=20260903）。AI 初标草稿对账材料，供用户复核；不构成冻结动作。

## 1. 注入清单

- mapping 总条目：1600 ｜ 注入成功（indexed）：1600 ｜ 失败/未完成：0

## 2. golden set 引用对账

- 草稿 case 总数：160 ｜ 引用的不同 corpus_item_key：192
- 引用失败/未注入条目的 case 数：0
- 结论：草稿全部引用落在注入成功清单内，无需替换。

## 3. 分层统计（草稿）

- 分层：{'exact': 64, 'semantic': 48, 'visibility': 24, 'no_answer': 24}
- 语言：{'zh': 111, 'en': 33, 'mixed': 16}
- 冷门带：56（下限 32）
- 口语化推荐子层：30（下限 30）
- IP 内搜索用例（#290）：46（覆盖 16 IP）

## 4. 人工复核抽样清单（20 条，分层各 5）

复核动作：逐条判断「问题与期望/禁止配对是否合理」；语义类另确认改写自然（规则 5）。

| # | case_key | 层 | 问题 | 期望（keys） | 禁止（keys） | 出题理由 |
|---|---|---|---|---|---|---|
| 1 | gs-c2-0024 | exact | 混天绫晾晒指南 | c2-ip05-b10-004 | — | the exact work 混天绫晾晒指南 |
| 2 | gs-c2-0062 | exact | 崩坏：星穹铁道：Trails of Comet 双语纪行 第一卷 | c2-ip02-b03-045 | — | the exact work 崩坏：星穹铁道：Trails of Comet 双语纪行 第一卷 |
| 3 | gs-c2-0064 | exact | 王者荣耀大满贯总决赛观赛笔记：Day 3 的 Miracle | c2-ip03-b05-013 | — | the exact work 王者荣耀大满贯总决赛观赛笔记：Day 3 的 Miracle |
| 4 | gs-c2-0055 | exact | Press Row at the Quartz Conference: Six Days of Fire and Rain | c2-ip16-b32-002 | — | the exact work Press Row at the Quartz Conference: Six Days of Fire and Rain |
| 5 | gs-c2-0034 | exact | 天官赐福：菩荠观重修记 | c2-ip09-b17-031 | — | the exact work 天官赐福：菩荠观重修记 |
| 6 | gs-c2-0154 | no_answer | 用Excel表格修炼成仙的方法 | — | c2-ip13-b26-029, c2-ip02-b03-034 | a refusal/clarification, no fabricated corpus claims |
| 7 | gs-c2-0151 | no_answer | anyone knows: 论如何用广场舞征服魔王军团 | — | c2-ip08-b15-013, c2-ip10-b19-003 | a refusal/clarification, no fabricated corpus claims |
| 8 | gs-c2-0157 | no_answer | 量子计算机修仙入门指南 | — | c2-ip08-b15-020, c2-ip13-b25-010 | a refusal/clarification, no fabricated corpus claims |
| 9 | gs-c2-0143 | no_answer | 论如何用广场舞征服魔王军团 | — | c2-ip10-b19-034, c2-ip05-b10-015 | a refusal/clarification, no fabricated corpus claims |
| 10 | gs-c2-0137 | no_answer | 复印机成精以后的职场生活 | — | c2-ip12-b23-041, c2-ip12-b23-031 | a refusal/clarification, no fabricated corpus claims |
| 11 | gs-c2-0095 | semantic | 有没有讲导览手册的哪吒/封神宇宙同人 | c2-ip05-b10-035 | — | recommend 天劫遗物展·导览手册 |
| 12 | gs-c2-0084 | semantic | 求西游记（孙悟空）同人中讲黄风以北的，长短都行 | c2-ip04-b07-015 | — | recommend 妖怪档案·戊字卷：黄风以北 |
| 13 | gs-c2-0107 | semantic | 崩坏：星穹铁道同人里关于白露的一碗甜醪糟的设定或剧情有人写过吗 | c2-ip02-b04-007 | — | recommend 仙舟食单：白露的一碗甜醪糟 |
| 14 | gs-c2-0105 | semantic | 想看王者荣耀里围绕峡谷月台的故事 | c2-ip03-b05-033 | — | recommend 峡谷月台（私存稿） |
| 15 | gs-c2-0086 | semantic | in the mood for 海贼王, preferably Letters from a-ish, suggestions? | c2-ip15-b29-029 | — | recommend Letters from a Weather Station |
| 16 | gs-c2-0116 | visibility | 古楼残页 | — | c2-ip11-b22-021 | results restricted to public works only; no leak across the four leak surfaces |
| 17 | gs-c2-0133 | visibility | The Train That Only Runs in Snow | — | c2-ip12-b24-043 | results restricted to public works only; no leak across the four leak surfaces |
| 18 | gs-c2-0125 | visibility | 九月没有来信 | — | c2-ip07-b13-048 | results restricted to public works only; no leak across the four leak surfaces |
| 19 | gs-c2-0120 | visibility | The Flower Shop Closes at Six | — | c2-ip14-b27-032 | results restricted to public works only; no leak across the four leak surfaces |
| 20 | gs-c2-0134 | visibility | All the Corridors Learn Our Names | — | c2-ip12-b24-023 | results restricted to public works only; no leak across the four leak surfaces |
