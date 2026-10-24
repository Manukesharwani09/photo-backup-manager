-- A photo (media) may have at most one live backup across all accounts: this is the database-level
-- guarantee behind "never upload the same photo twice". deleted_in_cloud / failed rows don't count.
CREATE UNIQUE INDEX IF NOT EXISTS backup_record_one_live_per_media
    ON backup_record (media_id) WHERE status IN ('uploaded', 'verified');

CREATE INDEX IF NOT EXISTS device_file_media_idx ON device_file (media_id);
