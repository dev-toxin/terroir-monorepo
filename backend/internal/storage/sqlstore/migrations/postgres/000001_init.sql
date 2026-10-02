-- Terroir schema v1 (personal wine assistant). PostgreSQL dialect.
-- Keep in sync with ../sqlite/000001_init.sql (same tables/columns, only types differ).

CREATE TABLE users (
    id           TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    locale       TEXT NOT NULL DEFAULT 'ru' CHECK (locale IN ('ru', 'en')),
    telegram_id  BIGINT UNIQUE,
    created_at   TIMESTAMPTZ NOT NULL
);

CREATE TABLE user_tokens (
    token_hash TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE cellars (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE cellar_members (
    cellar_id TEXT NOT NULL REFERENCES cellars(id) ON DELETE CASCADE,
    user_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role      TEXT NOT NULL CHECK (role IN ('owner', 'member')),
    PRIMARY KEY (cellar_id, user_id)
);

CREATE TABLE wines (
    id          TEXT PRIMARY KEY,
    cellar_id   TEXT NOT NULL REFERENCES cellars(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    producer    TEXT NOT NULL DEFAULT '',
    country     TEXT NOT NULL DEFAULT '',
    region      TEXT NOT NULL DEFAULT '',
    appellation TEXT NOT NULL DEFAULT '',
    grapes      TEXT NOT NULL DEFAULT '[]', -- JSON array
    vintage     INTEGER,                    -- NULL = non-vintage
    color       TEXT NOT NULL,
    sweetness   TEXT NOT NULL DEFAULT '',
    abv         DOUBLE PRECISION,
    body        INTEGER NOT NULL DEFAULT 0,
    acidity     INTEGER NOT NULL DEFAULT 0,
    tannin      INTEGER NOT NULL DEFAULT 0,
    style_notes TEXT NOT NULL DEFAULT '',
    source      TEXT NOT NULL DEFAULT 'manual',
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL
);
CREATE INDEX wines_cellar_idx ON wines (cellar_id);

CREATE TABLE cellar_items (
    id                   TEXT PRIMARY KEY,
    cellar_id            TEXT NOT NULL REFERENCES cellars(id) ON DELETE CASCADE,
    wine_id              TEXT NOT NULL REFERENCES wines(id) ON DELETE RESTRICT,
    quantity             INTEGER NOT NULL DEFAULT 1 CHECK (quantity >= 0),
    storage_location     TEXT NOT NULL DEFAULT '',
    purchase_price_minor BIGINT,
    currency             TEXT NOT NULL DEFAULT '',
    purchase_place       TEXT NOT NULL DEFAULT '',
    purchased_at         TIMESTAMPTZ,
    drink_from           INTEGER,
    drink_until          INTEGER,
    notes                TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL,
    updated_at           TIMESTAMPTZ NOT NULL
);
CREATE INDEX cellar_items_cellar_idx ON cellar_items (cellar_id);
CREATE INDEX cellar_items_wine_idx ON cellar_items (wine_id);

CREATE TABLE consumption_events (
    id               TEXT PRIMARY KEY,
    cellar_item_id   TEXT NOT NULL REFERENCES cellar_items(id) ON DELETE CASCADE,
    wine_id          TEXT NOT NULL,
    user_id          TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bottles          INTEGER NOT NULL DEFAULT 1,
    drunk_at         TIMESTAMPTZ NOT NULL,
    rating           INTEGER NOT NULL DEFAULT 0 CHECK (rating BETWEEN 0 AND 5),
    note             TEXT NOT NULL DEFAULT '',
    occasion         TEXT NOT NULL DEFAULT '',
    paired_recipe_id TEXT
);
CREATE INDEX consumption_item_idx ON consumption_events (cellar_item_id);
CREATE INDEX consumption_user_idx ON consumption_events (user_id);

CREATE TABLE taste_profiles (
    user_id    TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    data       TEXT NOT NULL, -- JSON of domain.TasteProfile
    version    INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE survey_answers (
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question_id   TEXT NOT NULL,
    answer_values TEXT NOT NULL, -- JSON array
    answered_at   TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, question_id)
);

CREATE TABLE recipes (
    id              TEXT PRIMARY KEY,
    cellar_id       TEXT NOT NULL REFERENCES cellars(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    cuisine         TEXT NOT NULL DEFAULT '',
    main_ingredient TEXT NOT NULL,
    cooking_method  TEXT NOT NULL,
    sauce           TEXT NOT NULL DEFAULT 'none',
    spiciness       INTEGER NOT NULL DEFAULT 0,
    richness        INTEGER NOT NULL DEFAULT 3,
    acidity         INTEGER NOT NULL DEFAULT 2,
    time_minutes    INTEGER NOT NULL DEFAULT 0,
    notes           TEXT NOT NULL DEFAULT '',
    tags            TEXT NOT NULL DEFAULT '[]',
    created_at      TIMESTAMPTZ NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL
);
CREATE INDEX recipes_cellar_idx ON recipes (cellar_id);

CREATE TABLE pairing_feedback (
    id         TEXT PRIMARY KEY,
    cellar_id  TEXT NOT NULL REFERENCES cellars(id) ON DELETE CASCADE,
    recipe_id  TEXT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    wine_id    TEXT NOT NULL REFERENCES wines(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating     INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
    note       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX pairing_feedback_cellar_idx ON pairing_feedback (cellar_id);

CREATE TABLE chat_messages (
    id              TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content         TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL
);
CREATE INDEX chat_messages_conv_idx ON chat_messages (conversation_id, created_at);

CREATE TABLE memory_facts (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    fact       TEXT NOT NULL,
    source     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX memory_facts_user_idx ON memory_facts (user_id, created_at);

CREATE TABLE llm_usage (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL,
    usage_period      TEXT NOT NULL, -- 'YYYY-MM' (UTC) for monthly budget
    provider          TEXT NOT NULL,
    model             TEXT NOT NULL,
    prompt_tokens     INTEGER NOT NULL,
    completion_tokens INTEGER NOT NULL,
    cost_micros       BIGINT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL
);
CREATE INDEX llm_usage_period_idx ON llm_usage (usage_period);
