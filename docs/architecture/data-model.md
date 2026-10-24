# Data model

## Where the "entities" are
This is a Go backend **without an ORM**, so there are no entity classes like in Java/Spring:
- **Schema (source of truth):** `internal/store/migrations/*.sql`. They are applied in order on every server start (001 init, 002 one-live-backup, 003 To delete, 004 account ownership).
- **Go types:** each query returns a small struct shaped for its use, in `internal/store/*.go`:

| Go struct (`internal/store`) | Reads from | Used for |
|---|---|---|
| `Account` (store.go) | `account` | sign-in, tokens, roles |
| `Period` (store.go) | `account_period` | active periods |
| `FileMeta` (backup.go) | → `media` + `device_file` | what the phone reports per file |
| `UploadFile` (backup.go) | `device_file` | upload completion |
| `DeviceFileStatus` (cleanup.go) | `device_file` + live `backup_record` + `account` | cleanup decisions |
| `RestoreCandidate` (restore.go) | `device_file` + `media` + `backup_record` | restore |
| `MediaDetail`, `ToDeleteItem` (similar.go) | `media` + `device_file` + `backup_record` | Similar photos, To delete |
| `LiveRecord` (cloudcheck.go) | `backup_record` | cloud check |
| `AccountStats`, `TakenIn` (dashboard.go) | aggregates | dashboard, photos-by-date |
| `plan.Rule`, `plan.Account`, `plan.Photo` (`internal/plan`) | `folder_rule`, `account`, `media` | pure decision logic |

## Diagram
```mermaid
erDiagram
    app_user ||--o{ account : "owns (1 primary + N overflow)"
    app_user ||--o{ device : "has phones"
    app_user ||--o{ media : "owns photos"
    app_user ||--o{ folder_rule : "configures"
    account ||--o{ account_period : "was active during"
    account ||--o{ album : "holds"
    account ||--o{ backup_record : "stores"
    account |o--o{ folder_rule : "fixed target of"
    device ||--o{ device_file : "reports"
    media ||--o{ device_file : "same bytes at paths"
    media ||--o| backup_record : "at most 1 live backup"
    album |o--o{ backup_record : "contains"
```

## Tables
| Table | One row is… | Key columns | Rules enforced by the database |
|---|---|---|---|
| `app_user` | an **owner** (a person using the system) | `id`, `email` | `email` unique. Today a single local owner (`email = 'local'`) |
| `account` | a **Google account** linked to an owner | `user_id`, `email`, `google_sub`, `role` (primary/overflow), `active`, `full_flag`, `refresh_token_enc` (AES-GCM) | **A Google account belongs to one owner, ever** (unique `google_sub`, unique `lower(email)`). **One primary per owner.** Role ∈ {primary, overflow} |
| `account_period` | a span when an account was **the** backup destination | `account_id`, `started_at`, `ended_at`, `end_reason` (full/switched/manual) | At most one open period per owner (in code) |
| `device` | a **phone** | `user_id`, `model`, `first_scan_at` (= the start point) | — |
| `media` | a unique **photo content** (by bytes) | `user_id`, `sha256`, `phash`, `width`, `height`, `date_taken`, `gps_lat/lng` | unique (`user_id`, `sha256`): the same bytes are one media, wherever they are |
| `device_file` | a **file on a phone** at a path | `device_id`, `media_id`, `relative_path`, `display_name`, `size`, `source`, `state`, `present_on_device` | unique (`device_id`, path, name). `state` ∈ {existing_skip, pending, backed_up, excluded} |
| `backup_record` | a media **stored in an account** | `media_id`, `account_id`, `gphotos_media_item_id`, `product_url`, `album_id`, `status`, `marked_for_delete_at`, `verified_at`, `last_checked_at` | **At most one live backup per media** (unique `media_id` where status ∈ {uploaded, verified}). This is the "never upload twice" guarantee |
| `album` | a Google Photos **album** per account per phone folder | `account_id`, `folder_path`, `title`, `gphotos_album_id`, `product_url` | unique (`account_id`, `folder_path`). `folder_path = ':to-delete:'` is the "🗑 To delete" album |
| `folder_rule` | a **per-folder rule** | `user_id`, `path_glob`, `target_account_id`, `backup` (include/exclude), `cleanup` (allowed/protected), `priority` | — |

## How accounts are managed
```
owner (app_user)
 ├── primary account     exactly 1. Fixed target for rules like Documents → primary
 ├── overflow account    0..N. Used when the current account is full, first by id
 └── ...
```
- **Linking:** `/oauth/start?role=primary|overflow`. The backend stores the encrypted refresh token, plus the account's email and Google `sub` (its permanent id).
- **Ownership is exclusive.** Once a Google account is linked to an owner, as primary **or** overflow, it can't be linked to any other owner, as primary or anything else. This holds even if the account's email later changes, because `sub` is checked too. Trying gives HTTP 409 "already linked to another owner". The same owner reconnecting (token refresh) is fine.
- **One primary per owner.** A second primary gets a 409; connect it as overflow instead.
- **Which account receives new photos:** the one with the open `account_period` ("current"). When it's full, the next usable overflow account takes over: the period is closed with `end_reason = full` and a new one opens. Folders with a fixed target (e.g. Documents → primary) **never** fall back; they wait until that account has space.
- **Today there's one owner** (`DefaultUserID`), and the app ↔ backend connection has no login yet. Planned: the owner signs in to our app with their **primary Google account**, which becomes the owner's identity. Every overflow account hangs off that owner. The ownership rule above is already in place for that.
