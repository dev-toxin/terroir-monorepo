CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS wines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    producer TEXT NOT NULL DEFAULT '',
    country TEXT NOT NULL DEFAULT '',
    region TEXT NOT NULL DEFAULT '',
    grape_varieties TEXT[] NOT NULL DEFAULT '{}',
    vintage SMALLINT,
    wine_type TEXT NOT NULL DEFAULT '',
    sweetness TEXT NOT NULL DEFAULT '',
    alcohol_percent NUMERIC(4, 2),
    tasting_profile JSONB NOT NULL DEFAULT '{}'::jsonb,
    tasting_embedding VECTOR(1536),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS cellars (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS cellar_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cellar_id UUID NOT NULL REFERENCES cellars(id) ON DELETE CASCADE,
    wine_id UUID NOT NULL REFERENCES wines(id) ON DELETE RESTRICT,
    bottle_count INTEGER NOT NULL DEFAULT 1 CHECK (bottle_count >= 0),
    purchase_price NUMERIC(12, 2),
    purchase_currency CHAR(3),
    storage_location TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS label_scans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    wine_id UUID REFERENCES wines(id) ON DELETE SET NULL,
    image_uri TEXT NOT NULL,
    ocr_provider TEXT NOT NULL,
    ocr_text TEXT NOT NULL DEFAULT '',
    llm_provider TEXT NOT NULL DEFAULT '',
    parsed_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'created',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS wines_name_trgm_idx ON wines USING GIN (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS wines_tasting_profile_gin_idx ON wines USING GIN (tasting_profile);
CREATE INDEX IF NOT EXISTS label_scans_user_id_idx ON label_scans (user_id);
CREATE INDEX IF NOT EXISTS cellar_items_cellar_id_idx ON cellar_items (cellar_id);
