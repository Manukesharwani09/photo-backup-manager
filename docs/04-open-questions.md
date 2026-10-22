# 04 — Open questions & next step

## Confirmed
- **Platform:** Android only, native Kotlin.
- **Audience:** personal use first, to prove it works. Publishing will be decided later.

## Still open
1. **Baseline for photos backed up before the app:** Google Takeout JSON import, manual date/folder rules, or both?
2. **Storage-left display:** add a Drive scope (`about.get` → `storageQuota`) to show remaining space per account, or just react to the "account full" error?
3. **Album naming:** full path (`DCIM/Camera`), last folder name (`Camera`), or a custom name per rule?
4. **Registry backup location:** Drive appDataFolder of the primary account, or a file you choose?
5. **Google Photos app's own backup:** turn it off completely, or keep it for folders the app doesn't manage?
6. **WhatsApp unclear matches:** review one by one, or default to "skip everything that existed at first scan"?

## If publishing later (not needed now)
- Google OAuth verification for the sensitive Photos scopes, plus a privacy policy and homepage.
- Play Store review of `MANAGE_MEDIA` / media permissions.
- A request for a higher Photos API quota (Google Photos partner program) if many users upload.

## Suggested first proof of concept (not built yet)
A small Kotlin app that tests the risky part before any further design:
1. Sign in to **2 Google accounts** (OAuth, scopes `photoslibrary.appendonly` + `photoslibrary.readonly.appcreateddata` + `photoslibrary.edit.appcreateddata`).
2. Upload **5 test photos** to account B.
3. **List them back** with app-created data access to prove verification works.
4. **Create an album** named after the source folder and add the photos to it.
5. Store the results in a tiny Room table, so this becomes the start of the registry.

If all 5 steps work, the remaining work is ordinary Android app work: MediaStore scan, hashing, rules, cleanup and UI.
