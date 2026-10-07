-- Seed the provider catalog (design §5.4 Provider Registry): the
-- well-known providers and their common plans as plain metadata rows.
-- Users pick these when creating subscriptions; nothing here binds an
-- account — the registry stays a pure catalog.

INSERT INTO providers (slug, name, category) VALUES
    ('netflix', 'Netflix', 'streaming'),
    ('disney-plus', 'Disney+', 'streaming'),
    ('spotify', 'Spotify', 'music'),
    ('youtube-music', 'YouTube Music', 'music'),
    ('chatgpt', 'ChatGPT', 'ai'),
    ('claude', 'Claude', 'ai'),
    ('adobe', 'Adobe Creative Cloud', 'software'),
    ('jetbrains', 'JetBrains All Products', 'software');

INSERT INTO products (provider_id, name, tier)
SELECT p.id, v.name, v.tier
FROM (VALUES
    ('netflix', 'Standard', 'standard'),
    ('netflix', 'Premium', 'premium'),
    ('disney-plus', 'Premium', 'premium'),
    ('spotify', 'Premium Individual', 'premium'),
    ('spotify', 'Family', 'family'),
    ('youtube-music', 'Premium', 'premium'),
    ('chatgpt', 'Plus', 'plus'),
    ('chatgpt', 'Pro', 'pro'),
    ('claude', 'Pro', 'pro'),
    ('claude', 'Max', 'max'),
    ('adobe', 'All Apps', 'all_apps'),
    ('jetbrains', 'All Products', 'all_products')
) AS v(slug, name, tier)
JOIN providers p ON p.slug = v.slug;
