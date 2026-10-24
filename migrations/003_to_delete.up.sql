-- Link to open an album in Google Photos (used for the "To delete" album card).
ALTER TABLE album ADD COLUMN IF NOT EXISTS product_url TEXT;

-- Set when the user picked this cloud copy for deletion and it was added to the account's "To delete" album.
ALTER TABLE backup_record ADD COLUMN IF NOT EXISTS marked_for_delete_at TIMESTAMPTZ;
