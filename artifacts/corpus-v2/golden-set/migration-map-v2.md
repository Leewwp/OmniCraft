# Golden Set v1 → v2 全量迁移映射（196 条方案）

> 生成：2026-09-04 ｜ 由 `scripts/corpus/build_migration_map.py` 确定性生成（处置清单来自已双重复核的审计报告 v1.2）｜ 配套规范：`docs/working/2026-09-04-golden-set-v2-annotation-spec.md`（v2.0-draft-r2）｜ 验证：`scripts/corpus/verify_migration_map.py` ｜ 待用户确认规范后执行

处置汇总：{('exact', 'target_swap'): 19, ('exact', 'keep'): 45, ('semantic', 'rewrite'): 38, ('semantic', 'target_swap+rewrite'): 10, ('visibility', 'keep+principal'): 24, ('no_answer', 'keep+strategy'): 12, ('no_answer', 'drop(merged)'): 12, ('-', 'new'): 48}

| 旧 case_key | 旧层 | 处置 | v2 key | v2 层 | 说明 |
|---|---|---|---|---|---|
| gs-c2-0001 | exact | target_swap | ke-0001 | known_item_exact | 换同IP public title_form=exact 目标（候选池 31 条），查询随新标题重生成；保留「在IP内：」前缀 |
| gs-c2-0002 | exact | keep | ke-0002 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0003 | exact | keep | ke-0003 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0004 | exact | keep | ke-0004 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0005 | exact | keep | ke-0005 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0006 | exact | keep | ke-0006 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0007 | exact | keep | ke-0007 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0008 | exact | keep | ke-0008 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0009 | exact | keep | ke-0009 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0010 | exact | target_swap | ke-0010 | known_item_exact | 换同IP public title_form=exact 目标（候选池 28 条），查询随新标题重生成；保留「在IP内：」前缀 |
| gs-c2-0011 | exact | target_swap | ke-0011 | known_item_exact | 换同IP public title_form=exact 目标（候选池 31 条），查询随新标题重生成；保留「在IP内：」前缀 |
| gs-c2-0012 | exact | keep | ke-0012 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0013 | exact | target_swap | ke-0013 | known_item_exact | 换同IP public title_form=exact 目标（候选池 35 条），查询随新标题重生成；保留「在IP内：」前缀 |
| gs-c2-0014 | exact | target_swap | ke-0014 | known_item_exact | 换同IP public title_form=exact 目标（候选池 31 条），查询随新标题重生成；保留「在IP内：」前缀 |
| gs-c2-0015 | exact | target_swap | ke-0015 | known_item_exact | 换同IP public title_form=exact 目标（候选池 27 条），查询随新标题重生成；保留「在IP内：」前缀 |
| gs-c2-0016 | exact | target_swap | ke-0016 | known_item_exact | 换同IP public title_form=exact 目标（候选池 32 条），查询随新标题重生成；保留「在IP内：」前缀 |
| gs-c2-0017 | exact | keep | ke-0017 | known_item_exact | 查询与目标原样保留；ASCII冒号（#319 已修，回归观测子集） |
| gs-c2-0018 | exact | target_swap | ke-0018 | known_item_exact | 换同IP public title_form=exact 目标（候选池 32 条），查询随新标题重生成 |
| gs-c2-0019 | exact | keep | ke-0019 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0020 | exact | keep | ke-0020 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0021 | exact | keep | ke-0021 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0022 | exact | keep | ke-0022 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0023 | exact | keep | ke-0023 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0024 | exact | target_swap | ke-0024 | known_item_exact | 换同IP public title_form=exact 目标（候选池 32 条），查询随新标题重生成 |
| gs-c2-0025 | exact | target_swap | ke-0025 | known_item_exact | 换同IP public title_form=exact 目标（候选池 32 条），查询随新标题重生成 |
| gs-c2-0026 | exact | keep | ke-0026 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0027 | exact | keep | ke-0027 | known_item_exact | 查询与目标原样保留；ASCII冒号（#319 已修，回归观测子集） |
| gs-c2-0028 | exact | keep | ke-0028 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0029 | exact | target_swap | ke-0029 | known_item_exact | 换同IP public title_form=exact 目标（候选池 32 条），查询随新标题重生成 |
| gs-c2-0030 | exact | keep | ke-0030 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0031 | exact | keep | ke-0031 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0032 | exact | target_swap | ke-0032 | known_item_exact | 换同IP public title_form=exact 目标（候选池 34 条），查询随新标题重生成 |
| gs-c2-0033 | exact | keep | ke-0033 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0034 | exact | keep | ke-0034 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0035 | exact | keep | ke-0035 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0036 | exact | keep | ke-0036 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0037 | exact | keep | ke-0037 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0038 | exact | target_swap | ke-0038 | known_item_exact | 换同IP public title_form=exact 目标（候选池 28 条），查询随新标题重生成 |
| gs-c2-0039 | exact | target_swap | ke-0039 | known_item_exact | 换同IP public title_form=exact 目标（候选池 29 条），查询随新标题重生成 |
| gs-c2-0040 | exact | target_swap | ke-0040 | known_item_exact | 换同IP public title_form=exact 目标（候选池 34 条），查询随新标题重生成 |
| gs-c2-0041 | exact | target_swap | ke-0041 | known_item_exact | 换同IP public title_form=exact 目标（候选池 29 条），查询随新标题重生成 |
| gs-c2-0042 | exact | keep | ke-0042 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0043 | exact | keep | ke-0043 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0044 | exact | keep | ke-0044 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0045 | exact | target_swap | ke-0045 | known_item_exact | 换同IP public title_form=exact 目标（候选池 31 条），查询随新标题重生成 |
| gs-c2-0046 | exact | keep | ke-0046 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0047 | exact | keep | ke-0047 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0048 | exact | target_swap | ke-0048 | known_item_exact | 换同IP public title_form=exact 目标（候选池 29 条），查询随新标题重生成 |
| gs-c2-0049 | exact | keep | ke-0049 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0050 | exact | keep | ke-0050 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0051 | exact | keep | ke-0051 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0052 | exact | target_swap | ke-0052 | known_item_exact | 换同IP public title_form=exact 目标（候选池 30 条），查询随新标题重生成；ASCII冒号（#319 已修，回归观测子集） |
| gs-c2-0053 | exact | keep | ke-0053 | known_item_exact | 查询与目标原样保留；ASCII冒号（#319 已修，回归观测子集） |
| gs-c2-0054 | exact | keep | ke-0054 | known_item_exact | 查询与目标原样保留；ASCII冒号（#319 已修，回归观测子集） |
| gs-c2-0055 | exact | keep | ke-0055 | known_item_exact | 查询与目标原样保留；ASCII冒号（#319 已修，回归观测子集） |
| gs-c2-0056 | exact | keep | ke-0056 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0057 | exact | keep | ke-0057 | known_item_exact | 查询与目标原样保留；ASCII冒号（#319 已修，回归观测子集） |
| gs-c2-0058 | exact | keep | ke-0058 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0059 | exact | keep | ke-0059 | known_item_exact | 查询与目标原样保留；ASCII冒号（#319 已修，回归观测子集） |
| gs-c2-0060 | exact | keep | ke-0060 | known_item_exact | 查询与目标原样保留；ASCII冒号（#319 已修，回归观测子集） |
| gs-c2-0061 | exact | keep | ke-0061 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0062 | exact | keep | ke-0062 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0063 | exact | keep | ke-0063 | known_item_exact | 查询与目标原样保留；ASCII冒号（#319 已修，回归观测子集） |
| gs-c2-0064 | exact | keep | ke-0064 | known_item_exact | 查询与目标原样保留 |
| gs-c2-0065 | semantic | rewrite | sd-0001 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0066 | semantic | target_swap+rewrite | sd-0002 | semantic_discovery | 先换同IP public fuzzy/partial 目标（候选池 39 条）再藏标题重写+三档标注 |
| gs-c2-0067 | semantic | rewrite | sd-0003 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0068 | semantic | rewrite | sd-0004 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0069 | semantic | target_swap+rewrite | sd-0005 | semantic_discovery | 先换同IP public fuzzy/partial 目标（候选池 35 条）再藏标题重写+三档标注 |
| gs-c2-0070 | semantic | rewrite | sd-0006 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0071 | semantic | rewrite | sd-0007 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0072 | semantic | rewrite | sd-0008 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0073 | semantic | rewrite | sd-0009 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0074 | semantic | rewrite | sd-0010 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0075 | semantic | rewrite | sd-0011 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0076 | semantic | target_swap+rewrite | sd-0012 | semantic_discovery | 先换同IP public fuzzy/partial 目标（候选池 37 条）再藏标题重写+三档标注 |
| gs-c2-0077 | semantic | rewrite | sd-0013 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0078 | semantic | rewrite | sd-0014 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0079 | semantic | rewrite | sd-0015 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0080 | semantic | rewrite | sd-0016 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0081 | semantic | rewrite | sd-0017 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0082 | semantic | target_swap+rewrite | sd-0018 | semantic_discovery | 先换同IP public fuzzy/partial 目标（候选池 37 条）再藏标题重写+三档标注 |
| gs-c2-0083 | semantic | rewrite | sd-0019 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0084 | semantic | rewrite | sd-0020 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0085 | semantic | rewrite | sd-0021 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0086 | semantic | target_swap+rewrite | sd-0022 | semantic_discovery | 先换同IP public fuzzy/partial 目标（候选池 38 条）再藏标题重写+三档标注 |
| gs-c2-0087 | semantic | rewrite | sd-0023 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0088 | semantic | rewrite | sd-0024 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0089 | semantic | rewrite | sd-0025 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0090 | semantic | rewrite | sd-0026 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0091 | semantic | rewrite | sd-0027 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0092 | semantic | rewrite | sd-0028 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0093 | semantic | rewrite | sd-0029 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0094 | semantic | rewrite | sd-0030 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0095 | semantic | rewrite | sd-0031 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0096 | semantic | target_swap+rewrite | sd-0032 | semantic_discovery | 先换同IP public fuzzy/partial 目标（候选池 34 条）再藏标题重写+三档标注 |
| gs-c2-0097 | semantic | rewrite | sd-0033 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0098 | semantic | rewrite | sd-0034 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0099 | semantic | rewrite | sd-0035 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0100 | semantic | rewrite | sd-0036 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0101 | semantic | rewrite | sd-0037 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0102 | semantic | rewrite | sd-0038 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0103 | semantic | target_swap+rewrite | sd-0039 | semantic_discovery | 先换同IP public fuzzy/partial 目标（候选池 38 条）再藏标题重写+三档标注 |
| gs-c2-0104 | semantic | rewrite | sd-0040 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0105 | semantic | target_swap+rewrite | sd-0041 | semantic_discovery | 先换同IP public fuzzy/partial 目标（候选池 34 条）再藏标题重写+三档标注 |
| gs-c2-0106 | semantic | target_swap+rewrite | sd-0042 | semantic_discovery | 先换同IP public fuzzy/partial 目标（候选池 35 条）再藏标题重写+三档标注 |
| gs-c2-0107 | semantic | rewrite | sd-0043 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0108 | semantic | rewrite | sd-0044 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0109 | semantic | rewrite | sd-0045 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0110 | semantic | target_swap+rewrite | sd-0046 | semantic_discovery | 先换同IP public fuzzy/partial 目标（候选池 35 条）再藏标题重写+三档标注 |
| gs-c2-0111 | semantic | rewrite | sd-0047 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0112 | semantic | rewrite | sd-0048 | semantic_discovery | 藏标题重写（不复用标题独特短语）+ expected/acceptable/forbidden 三档 |
| gs-c2-0113 | visibility | keep+principal | vi-0001 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0114 | visibility | keep+principal | vi-0002 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0115 | visibility | keep+principal | vi-0003 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0116 | visibility | keep+principal | vi-0004 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0117 | visibility | keep+principal | vi-0005 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0118 | visibility | keep+principal | vi-0006 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0119 | visibility | keep+principal | vi-0007 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0120 | visibility | keep+principal | vi-0008 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0121 | visibility | keep+principal | vi-0009 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0122 | visibility | keep+principal | vi-0010 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0123 | visibility | keep+principal | vi-0011 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0124 | visibility | keep+principal | vi-0012 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0125 | visibility | keep+principal | vi-0013 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0126 | visibility | keep+principal | vi-0014 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0127 | visibility | keep+principal | vi-0015 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0128 | visibility | keep+principal | vi-0016 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0129 | visibility | keep+principal | vi-0017 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0130 | visibility | keep+principal | vi-0018 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0131 | visibility | keep+principal | vi-0019 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0132 | visibility | keep+principal | vi-0020 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0133 | visibility | keep+principal | vi-0021 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0134 | visibility | keep+principal | vi-0022 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0135 | visibility | keep+principal | vi-0023 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0136 | visibility | keep+principal | vi-0024 | visibility | 保留；principal=anon+fixture:viewer-anon 双身份，四泄漏面零 |
| gs-c2-0137 | no_answer | keep+strategy | na-0001 | no_answer | 策略=related_recommendation_allowed；干扰项重抽为真·近邻（人工判） |
| gs-c2-0138 | no_answer | keep+strategy | na-0002 | no_answer | 策略=strict_not_found；干扰项重抽为真·近邻（人工判） |
| gs-c2-0139 | no_answer | keep+strategy | na-0003 | no_answer | 策略=related_recommendation_allowed；干扰项重抽为真·近邻（人工判） |
| gs-c2-0140 | no_answer | keep+strategy | na-0004 | no_answer | 策略=related_recommendation_allowed；干扰项重抽为真·近邻（人工判） |
| gs-c2-0142 | no_answer | keep+strategy | na-0005 | no_answer | 策略=related_recommendation_allowed；干扰项重抽为真·近邻（人工判） |
| gs-c2-0143 | no_answer | keep+strategy | na-0006 | no_answer | 策略=related_recommendation_allowed；干扰项重抽为真·近邻（人工判） |
| gs-c2-0144 | no_answer | keep+strategy | na-0007 | no_answer | 策略=related_recommendation_allowed；干扰项重抽为真·近邻（人工判） |
| gs-c2-0145 | no_answer | keep+strategy | na-0008 | no_answer | 策略=related_recommendation_allowed；干扰项重抽为真·近邻（人工判） |
| gs-c2-0148 | no_answer | keep+strategy | na-0009 | no_answer | 策略=related_recommendation_allowed；干扰项重抽为真·近邻（人工判） |
| gs-c2-0149 | no_answer | keep+strategy | na-0010 | no_answer | 策略=strict_not_found；干扰项重抽为真·近邻（人工判） |
| gs-c2-0150 | no_answer | keep+strategy | na-0011 | no_answer | 策略=related_recommendation_allowed；干扰项重抽为真·近邻（人工判） |
| gs-c2-0155 | no_answer | keep+strategy | na-0012 | no_answer | 策略=related_recommendation_allowed；干扰项重抽为真·近邻（人工判） |
| gs-c2-0141 | no_answer | drop(merged) | na-0010 | no_answer | 与 na-0010（gs-c2-0149）同题，合并删除 |
| gs-c2-0146 | no_answer | drop(merged) | na-0002 | no_answer | 与 na-0002（gs-c2-0138）同题，合并删除 |
| gs-c2-0147 | no_answer | drop(merged) | na-0003 | no_answer | 与 na-0003（gs-c2-0139）同题，合并删除 |
| gs-c2-0151 | no_answer | drop(merged) | na-0006 | no_answer | 与 na-0006（gs-c2-0143）同题，合并删除 |
| gs-c2-0152 | no_answer | drop(merged) | na-0007 | no_answer | 与 na-0007（gs-c2-0144）同题，合并删除 |
| gs-c2-0153 | no_answer | drop(merged) | na-0001 | no_answer | 与 na-0001（gs-c2-0137）同题，合并删除 |
| gs-c2-0154 | no_answer | drop(merged) | na-0002 | no_answer | 与 na-0002（gs-c2-0138）同题，合并删除 |
| gs-c2-0156 | no_answer | drop(merged) | na-0009 | no_answer | 与 na-0009（gs-c2-0148）同题，合并删除 |
| gs-c2-0157 | no_answer | drop(merged) | na-0010 | no_answer | 与 na-0010（gs-c2-0149）同题，合并删除 |
| gs-c2-0158 | no_answer | drop(merged) | na-0005 | no_answer | 与 na-0005（gs-c2-0142）同题，合并删除 |
| gs-c2-0159 | no_answer | drop(merged) | na-0006 | no_answer | 与 na-0006（gs-c2-0143）同题，合并删除 |
| gs-c2-0160 | no_answer | drop(merged) | na-0004 | no_answer | 与 na-0004（gs-c2-0140）同题，合并删除 |
| - | - | new | na-0013 | no_answer | 域内新增 1/8：IP 内合理主题、检索验证零精确对应；策略按查询形态定 |
| - | - | new | na-0014 | no_answer | 域内新增 2/8：IP 内合理主题、检索验证零精确对应；策略按查询形态定 |
| - | - | new | na-0015 | no_answer | 域内新增 3/8：IP 内合理主题、检索验证零精确对应；策略按查询形态定 |
| - | - | new | na-0016 | no_answer | 域内新增 4/8：IP 内合理主题、检索验证零精确对应；策略按查询形态定 |
| - | - | new | na-0017 | no_answer | 域内新增 5/8：IP 内合理主题、检索验证零精确对应；策略按查询形态定 |
| - | - | new | na-0018 | no_answer | 域内新增 6/8：IP 内合理主题、检索验证零精确对应；策略按查询形态定 |
| - | - | new | na-0019 | no_answer | 域内新增 7/8：IP 内合理主题、检索验证零精确对应；策略按查询形态定 |
| - | - | new | na-0020 | no_answer | 域内新增 8/8：IP 内合理主题、检索验证零精确对应；策略按查询形态定 |
| - | - | new | be-0001 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0002 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0003 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0004 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0005 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0006 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0007 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0008 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0009 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0010 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0011 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0012 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0013 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0014 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0015 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0016 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0017 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0018 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0019 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0020 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0021 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0022 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0023 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | be-0024 | body_evidence | 正文出题：只见 chunk 正文，答案关键信息不得出现在标题；span=codepoint 半开区间 |
| - | - | new | hn-0001 | hard_neighbor | 同 IP 近邻：多检索配置 Top-K 候选并集挖掘 + 人工判三档 |
| - | - | new | hn-0002 | hard_neighbor | 同 IP 近邻：多检索配置 Top-K 候选并集挖掘 + 人工判三档 |
| - | - | new | hn-0003 | hard_neighbor | 同 IP 近邻：多检索配置 Top-K 候选并集挖掘 + 人工判三档 |
| - | - | new | hn-0004 | hard_neighbor | 同 IP 近邻：多检索配置 Top-K 候选并集挖掘 + 人工判三档 |
| - | - | new | hn-0005 | hard_neighbor | 同 IP 近邻：多检索配置 Top-K 候选并集挖掘 + 人工判三档 |
| - | - | new | hn-0006 | hard_neighbor | 同 IP 近邻：多检索配置 Top-K 候选并集挖掘 + 人工判三档 |
| - | - | new | hn-0007 | hard_neighbor | 同 IP 近邻：多检索配置 Top-K 候选并集挖掘 + 人工判三档 |
| - | - | new | hn-0008 | hard_neighbor | 同 IP 近邻：多检索配置 Top-K 候选并集挖掘 + 人工判三档 |
| - | - | new | hn-0009 | hard_neighbor | 同 IP 近邻：多检索配置 Top-K 候选并集挖掘 + 人工判三档 |
| - | - | new | hn-0010 | hard_neighbor | 同 IP 近邻：多检索配置 Top-K 候选并集挖掘 + 人工判三档 |
| - | - | new | hn-0011 | hard_neighbor | 同 IP 近邻：多检索配置 Top-K 候选并集挖掘 + 人工判三档 |
| - | - | new | hn-0012 | hard_neighbor | 同 IP 近邻：多检索配置 Top-K 候选并集挖掘 + 人工判三档 |
| - | - | new | hn-0013 | hard_neighbor | 同 IP 近邻：多检索配置 Top-K 候选并集挖掘 + 人工判三档 |
| - | - | new | hn-0014 | hard_neighbor | 同 IP 近邻：多检索配置 Top-K 候选并集挖掘 + 人工判三档 |
| - | - | new | hn-0015 | hard_neighbor | 同 IP 近邻：多检索配置 Top-K 候选并集挖掘 + 人工判三档 |
| - | - | new | hn-0016 | hard_neighbor | 同 IP 近邻：多检索配置 Top-K 候选并集挖掘 + 人工判三档 |
