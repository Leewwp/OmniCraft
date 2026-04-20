CREATE TABLE IF NOT EXISTS categories (
    id          BIGSERIAL    PRIMARY KEY,
    zone        VARCHAR(20)  NOT NULL,
    level       VARCHAR(20)  NOT NULL,
    parent_id   BIGINT       REFERENCES categories(id),
    name_i18n   JSONB        NOT NULL DEFAULT '{}',
    slug        VARCHAR(100) NOT NULL UNIQUE,
    sort_order  INT          NOT NULL DEFAULT 0,
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_categories_zone_level ON categories (zone, level, sort_order);
CREATE INDEX IF NOT EXISTS idx_categories_parent ON categories (parent_id);

INSERT INTO categories (zone, level, slug, sort_order, name_i18n) VALUES
('fanwork', 'ip_category', 'gaming',      1, '{"zh":"游戏","en":"Gaming"}'),
('fanwork', 'ip_category', 'anime',       2, '{"zh":"动漫","en":"Anime"}'),
('fanwork', 'ip_category', 'music',       3, '{"zh":"音乐","en":"Music"}'),
('fanwork', 'ip_category', 'film_tv',     4, '{"zh":"影视","en":"Film/TV"}'),
('fanwork', 'ip_category', 'literature',  5, '{"zh":"文学","en":"Literature"}'),
('fanwork', 'ip_category', 'other',       99,'{"zh":"其他","en":"Other"}'),
('original','primary', 'film_tv_orig',    1, '{"zh":"影视","en":"Film/TV"}'),
('original','primary', 'gaming_orig',     2, '{"zh":"游戏","en":"Gaming"}'),
('original','primary', 'literature_orig', 3, '{"zh":"文学","en":"Literature"}'),
('original','primary', 'pet',            4, '{"zh":"宠物","en":"Pet"}'),
('original','primary', 'food',           5, '{"zh":"美食","en":"Food"}'),
('original','primary', 'beauty_fashion', 6, '{"zh":"美妆穿搭","en":"Beauty/Fashion"}'),
('original','primary', 'home',           7, '{"zh":"家居","en":"Home"}'),
('original','primary', 'tech_digital',   8, '{"zh":"数码科技","en":"Tech/Digital"}'),
('original','primary', 'travel',         9, '{"zh":"旅行","en":"Travel"}'),
('original','primary', 'sports',         10,'{"zh":"运动","en":"Sports"}'),
('original','primary', 'productivity',   11,'{"zh":"效率","en":"Productivity"}')
ON CONFLICT (slug) DO NOTHING;
