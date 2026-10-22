# 03 — Architecture (draft)

Android app, native Kotlin. Local database: Room (SQLite). Background work: WorkManager. No server needed for a personal build.

## Three layers
```
┌────────────────┐   ┌──────────────────┐   ┌──────────────────────┐
│ 1. Inventory    │   │ 2. Registry       │   │ 3. Policy engine      │
│ MediaStore scan │──▶│ what is backed up │◀──│ folder rules, accounts│
│ path, hash, date│   │ where, verified?  │   │ what to upload/delete │
└────────────────┘   └──────────────────┘   └──────────────────────┘
          │                    ▲                        │
          └──────▶ Upload worker / Cleanup worker ◀─────┘
                               │
                     Google Photos Library API (per account)
```

## Data model (first cut)
```
account(id, email, display_name, role[primary|overflow], active, full_flag,
        refresh_token_ref, created_at)

local_file(id, mediastore_id, relative_path, display_name, mime, size,
           date_taken, date_modified, sha256, source[camera|whatsapp|screenshot|download|other],
           first_seen_at, present_on_device)

backup_record(id, sha256, account_id, gphotos_media_item_id NULL, album_id NULL,
              status[uploaded|verified|baseline_assumed|failed|skipped_restored],
              origin[app_upload|takeout_import|manual_rule|user_resolved],
              uploaded_at, verified_at)

folder_rule(id, path_glob, target_account_id NULL /*auto*/, backup[include|exclude],
            cleanup[allowed|protected], priority)

album(id, account_id, folder_path, gphotos_album_id)
```
**Key rule:** a file is *backed up* if **any** `backup_record` with its `sha256` has status `verified` / `uploaded` / `baseline_assumed`. Accounts don't affect this check, which is what solves problems #3 and #4.

## Flows
**Backup run**
1. Scan MediaStore and add or update `local_file`. Hash new or changed files.
2. For each file: find the matching folder rule. If `exclude`, skip it. If the hash is already backed up, skip it.
3. Choose the account: the rule's target, or else the active overflow account that isn't full.
4. Upload the bytes, then `batchCreate` (50 at a time). Save the `mediaItemId` and set status = `uploaded`.
5. Add the items to the folder album (create it on first use).
6. Verify with `mediaItems.get` and set status = `verified`.

**Account full**
The upload fails with a storage error, so the app sets `full_flag` on that account, pauses, and asks you to pick or confirm the next account. It then resumes with only the pending files. Nothing on the device is touched.

**New phone / WhatsApp restore**
- When setting up on the new phone, the app imports the registry export (see below) first.
- At first scan, WhatsApp files whose hash is already in the registry are skipped.
- WhatsApp files with no matching hash that are dated before the **restore cutoff** go into a "review" bucket where you choose skip or upload. You can also say "skip all".
- WhatsApp files that arrive after the cutoff are backed up as usual.

**Selective cleanup**
1. You choose folders, and only those with `cleanup = allowed` are selectable.
2. Candidates are files in those folders that have a `verified` record and are not protected.
3. The app shows a preview with count, size and thumbnails.
4. It calls `MediaStore.createDeleteRequest`, and you confirm. It then marks `present_on_device = false`.

## Dashboard (problem #2)
For each account it shows: the number of files, the date range (min/max `date_taken`), the folders covered, the last upload, and full/active status. A search box answers "where is this photo?".

## Registry safety
The registry is the map of where everything is. If it lives only on the phone, losing the phone loses the map. Options: auto-export a JSON/SQLite snapshot to the **primary account's Drive appDataFolder** (needs the `drive.appdata` scope), or to a user-chosen file location. Import it on a new phone before the first scan.
