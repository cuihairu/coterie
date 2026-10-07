DELETE FROM products
WHERE provider_id IN (
    SELECT id FROM providers
    WHERE slug IN ('netflix', 'disney-plus', 'spotify', 'youtube-music',
                   'chatgpt', 'claude', 'adobe', 'jetbrains')
);

DELETE FROM providers
WHERE slug IN ('netflix', 'disney-plus', 'spotify', 'youtube-music',
               'chatgpt', 'claude', 'adobe', 'jetbrains');
