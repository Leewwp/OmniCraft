INSERT INTO rehab_courses (violation_type, content_i18n, min_reading_sec, reward_points) VALUES
(
    'malicious_report_tag',
    '{"zh": "# 恶意标签举报行为说明\n\n请勿滥用举报功能进行恶意操作...", "en": "# Malicious Tag Report\n\nPlease do not abuse the report feature..."}',
    120, 1
),
(
    'malicious_comment',
    '{"zh": "# 评论规范\n\n请文明发言，尊重他人...", "en": "# Comment Guidelines\n\nPlease be respectful..."}',
    90, 1
),
(
    'malicious_contribution',
    '{"zh": "# 协作贡献规范\n\n请勿提交无效或破坏性 PR...", "en": "# Contribution Guidelines\n\nDo not submit invalid or destructive PRs..."}',
    90, 1
),
(
    'malicious_report_comment',
    '{"zh": "# 评论举报规范\n\n请合理使用举报功能...", "en": "# Report Guidelines\n\nUse the report function responsibly..."}',
    60, 1
),
(
    'judge_error',
    '{"zh": "# 判官职责说明\n\n投票错误率过高将影响社区公正性...", "en": "# Judge Responsibilities\n\nHigh error rates affect community fairness..."}',
    180, 2
)
ON CONFLICT (violation_type) DO NOTHING;
