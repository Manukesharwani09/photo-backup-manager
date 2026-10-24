ALTER TABLE folder_rule DROP COLUMN IF EXISTS deleted_at;

DO $$
DECLARE t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY['app_user','device','account','account_period','media','device_file','album','backup_record','folder_rule'] LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS %I ON %I', t || '_set_updated_at', t);
        EXECUTE format('ALTER TABLE %I DROP COLUMN IF EXISTS updated_at', t);
    END LOOP;
    -- created_at existed before 005 on these two; only drop it where 005 added it.
    FOREACH t IN ARRAY ARRAY['device','account_period','media','device_file','album','backup_record','folder_rule'] LOOP
        EXECUTE format('ALTER TABLE %I DROP COLUMN IF EXISTS created_at', t);
    END LOOP;
END $$;

DROP FUNCTION IF EXISTS set_updated_at();
