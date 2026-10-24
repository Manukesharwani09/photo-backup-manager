# Decisions log

Confirmed decisions only. **Add a row only after the user explicitly confirms the decision.** Proposals go in [open-questions.md](open-questions.md). Newest first within each date.

| Date | Topic | Decision | Why / notes |
|---|---|---|---|
| 2026-10-24 | Account ownership | An owner has **one primary** Google account and any number of **overflow** accounts linked to it. **A Google account linked to one owner (in any role) can never be linked to another owner**, as primary or otherwise | Stated by the user. Enforced by DB unique indexes on `google_sub` and `lower(email)`, plus one primary per owner. See [architecture/data-model.md](architecture/data-model.md) |
| 2026-10-24 | Repo layout | The repo root **is the backend**. No `backend/` subfolder; the Android app will be a separate service. | User correction |
| 2026-10-24 | Google Cloud project | Project `photo-backup-manager` (no. 627633892991), owned by `mkesharwani125@gmail.com`, OAuth app in **Testing** mode | See [setup/google-cloud.md](setup/google-cloud.md) |
| 2026-10-24 | Destination | **Google Photos**, not Google Drive | Search, faces, memories, sharing and viewer matter more than Drive's technical advantages. See [product/drive-alternative-discussion.md](product/drive-alternative-discussion.md) |
| 2026-10-24 | Account time periods | Store when each account was the active destination (`account_period`), and calculate the photo-date range per account | Answers "which account has March 2024?" |
| 2026-10-24 | Deleting cloud copies | Not possible through the API. Duplicates go into a **"🗑 To delete"** album per account, and the user deletes them in Google Photos. A scheduled cloud check marks them `deleted_in_cloud` | The Library API has no delete method |
| 2026-10-24 | Similar photos | A "Similar photos" feature groups look-alikes (perceptual hash + name/date/size) | Look-alikes are uploaded, not skipped |
| 2026-10-24 | Same-photo rule | **Only an exact SHA-256 match is skipped.** Different bytes mean upload | |
| 2026-10-24 | Restore-to-folder | **In the first version.** GPS is stored at upload so it can be written back | API downloads may strip GPS |
| 2026-10-24 | Storage-left display | Show remaining space per account via Drive `about.get` → `storageQuota` (`drive.file` scope). The "account full" error is the fallback | |
| 2026-10-24 | Google Photos app backup | **Off completely** on the phone. Our app is the only uploader | Avoids two uploaders racing |
| 2026-10-24 | Hosting | **Local first** (Go + local PostgreSQL 18, no Docker). Later: Google Cloud Run + Neon Postgres. A custom domain only once it works | Free tiers; same Google Cloud project |
| 2026-10-24 | Backend stack | **Go + PostgreSQL** | |
| 2026-10-24 | Structure | Two services: **backend** (accounts, registry, rules, Google API calls) and **Android app** (scan, hash, upload bytes, delete). **Backend first** | Only the phone can touch files; everything else is central |
| 2026-10-24 | Folder grouping | One flat album per phone folder, titled by path (`DCIM › Camera`) | Google Photos has no nested albums |
| 2026-10-24 | Documents folder | Holds photos only → Google Photos, primary account, protected from cleanup | |
| 2026-10-24 | Quality | Original quality always (API default) | |
| 2026-10-24 | Scope | **Only new photos from the day the app starts.** Everything on the phone at the first scan is `existing_skip`. No Takeout or baseline import | The API can't see pre-existing library content |
| 2026-10-22 | Platform | Android only, native Kotlin (for the later app). Personal use first; publishing decided later | |
