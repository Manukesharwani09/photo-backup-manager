-- Initial schema. See docs/03-architecture.md for what each table is for.

CREATE TABLE IF NOT EXISTS app_user (
    id          BIGSERIAL PRIMARY KEY,
    email       TEXT UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS device (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT NOT NULL REFERENCES app_user(id),
    model          TEXT,
    registered_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    first_scan_at  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS account (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            BIGINT NOT NULL REFERENCES app_user(id),
    email              TEXT NOT NULL,
    role               TEXT NOT NULL CHECK (role IN ('primary', 'overflow')),
    active             BOOLEAN NOT NULL DEFAULT TRUE,
    full_flag          BOOLEAN NOT NULL DEFAULT FALSE,
    refresh_token_enc  BYTEA NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, email)
);

CREATE TABLE IF NOT EXISTS account_period (
    id          BIGSERIAL PRIMARY KEY,
    account_id  BIGINT NOT NULL REFERENCES account(id),
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at    TIMESTAMPTZ,
    end_reason  TEXT CHECK (end_reason IN ('full', 'switched', 'manual'))
);

CREATE TABLE IF NOT EXISTS media (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES app_user(id),
    sha256      BYTEA NOT NULL,
    phash       BIGINT,
    width       INT,
    height      INT,
    date_taken  TIMESTAMPTZ,
    gps_lat     DOUBLE PRECISION,
    gps_lng     DOUBLE PRECISION,
    UNIQUE (user_id, sha256)
);

CREATE TABLE IF NOT EXISTS device_file (
    id                 BIGSERIAL PRIMARY KEY,
    device_id          BIGINT NOT NULL REFERENCES device(id),
    media_id           BIGINT NOT NULL REFERENCES media(id),
    relative_path      TEXT NOT NULL,
    display_name       TEXT NOT NULL,
    size               BIGINT NOT NULL,
    source             TEXT NOT NULL CHECK (source IN ('camera', 'whatsapp', 'screenshot', 'download', 'other')),
    state              TEXT NOT NULL CHECK (state IN ('existing_skip', 'pending', 'backed_up', 'excluded')),
    present_on_device  BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE (device_id, relative_path, display_name)
);

CREATE TABLE IF NOT EXISTS album (
    id                BIGSERIAL PRIMARY KEY,
    account_id        BIGINT NOT NULL REFERENCES account(id),
    folder_path       TEXT NOT NULL,
    title             TEXT NOT NULL,
    gphotos_album_id  TEXT NOT NULL,
    UNIQUE (account_id, folder_path)
);

CREATE TABLE IF NOT EXISTS backup_record (
    id                     BIGSERIAL PRIMARY KEY,
    media_id               BIGINT NOT NULL REFERENCES media(id),
    account_id             BIGINT NOT NULL REFERENCES account(id),
    gphotos_media_item_id  TEXT,
    product_url            TEXT,
    album_id               BIGINT REFERENCES album(id),
    status                 TEXT NOT NULL CHECK (status IN ('uploaded', 'verified', 'failed', 'deleted_in_cloud')),
    uploaded_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    verified_at            TIMESTAMPTZ,
    last_checked_at        TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS folder_rule (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            BIGINT NOT NULL REFERENCES app_user(id),
    path_glob          TEXT NOT NULL,
    target_account_id  BIGINT REFERENCES account(id),
    backup             TEXT NOT NULL CHECK (backup IN ('include', 'exclude')),
    cleanup            TEXT NOT NULL CHECK (cleanup IN ('allowed', 'protected')),
    priority           INT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS backup_record_media_idx ON backup_record (media_id);
CREATE INDEX IF NOT EXISTS media_date_taken_idx ON media (user_id, date_taken);
