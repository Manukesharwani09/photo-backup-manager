# 05 — Discussion: Google Drive instead of Google Photos

_Discussion only, 2026-10-24. **Decision: stay with Google Photos.** This records what we looked at and why._

## The idea
Upload to **Google Drive** instead of Google Photos, and let our app be the photo viewer.

Clarification: Google Photos does **not** store photos in Drive. They only **share the same storage** (15 GB free per account). Since 2019, Photos and Drive have been separate, and items in one don't appear in the other.

## Where Drive is better (technically)
| Point | Google Photos | Google Drive |
|---|---|---|
| Delete from our app | ❌ API has no delete, so the "To delete" album workaround is needed | ✅ `files.delete` / trash |
| Folder structure | Flat albums (`DCIM › Camera`) | ✅ Real nested folders |
| Exact copy on restore | Download may drop GPS, and the bytes may change | ✅ Same bytes back: GPS kept, same hash |
| Server-side verification | Only that the item exists | ✅ Drive returns the file's SHA-256 |
| API limits | 10,000 requests per day | ✅ Thousands per minute |
| Share links from our app | ❌ Sharing removed from the API in 2025 | ✅ Can create links for our files |
| Future API risk | Already cut in 2025 | ✅ Stable |
| Publishing | Sensitive scopes, so Google review is needed | ✅ `drive.file` is non-sensitive, so review is easier |
| Storage cost | Same | Same |

## Where Google Photos is better
| Feature | Google Photos | Drive + our app |
|---|---|---|
| Smart search ("dog", "passport", "Goa") | ✅ | ❌ Only folder, date or filename |
| Face and pet groups | ✅ | ❌ |
| Memories, collages, highlight videos | ✅ | ❌ |
| Editing (Magic Eraser, filters; original kept) | ✅ | ❌ |
| Map / places view | ✅ | ❌ |
| Partner sharing, shared albums with comments | ✅ | ❌ Plain file sharing |
| TV / Chromecast / photo frames | ✅ | ❌ |
| Motion / Live photos play | ✅ | ❌ Shown as still images |
| Locked Folder | ✅ | ❌ |
| Copy text from photo (Lens) | ✅ | ❌ |
| Polished viewer on phone, web and TV | ✅ | ❌ Our app must become the gallery |
| Trash | 60 days | 30 days |
| Prints / photo books | ✅ | ❌ |

## Sharing (both options)
- **Sending a photo to WhatsApp and similar apps works the same with either.** Our app fetches the photo from whichever account holds it into a temporary cache and opens Android's share menu. You don't download anything by hand or switch accounts.
- **Share links and shared albums:**
  - Google Photos: done in the Google Photos app, which our app can open at the right photo or album.
  - Drive: our app could create them directly.

## Effect on our design
- The backend design stays the same either way: registry, rules, accounts, account-full handling and account periods.
- Only the upload, albums, delete and restore part would differ.
- With Drive, the backend gets simpler (no album, GPS or "To delete" workarounds), but the app gets bigger because it has to be a full gallery.

## Ideas noted, not chosen
- **Mixed per folder:** for example Documents → Drive (real folders) and Camera → Google Photos (search, memories). The backend could support it with a per-folder destination, with no double storage.
- **Both for the same photos:** rejected, because it doubles the storage.

## Why we chose Google Photos
Google Photos offers a full photo experience: search, faces, memories, editing, family sharing and a polished viewer on every device. Drive is only file storage. The Google Photos limits (no delete, flat albums, GPS on download) all have workarounds in our design.
