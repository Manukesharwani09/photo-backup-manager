# Google Cloud setup

This is what must be set up in Google Cloud before the Go backend can talk to Google Photos. Everything here is done in the **Google Cloud Console website**. Google has no CLI or API for the consent screen, test users, scopes or OAuth client of an External app.

- **Current project owner account:** `mkesharwani125@gmail.com`.
- **Current project:** `photo-backup-manager` (project number `627633892991`), created on 2026-10-24 with no organization and billing not enabled.
- **Previous setup:** the completed steps under `1by22cs111@bmsit.in` do not apply to this new project. The Gmail account cannot access `capable-avatar-512606-a8`.
- **Console:** https://console.cloud.google.com, with the project selected in the top bar.

## Status
| # | Step | Status |
|---|---|---|
| 1 | Create / select the project | ✅ Done |
| 2 | Enable Photos Library API + Google Drive API | ✅ Done: both enabled |
| 3 | Create the consent screen (External) | ✅ Done: "Photo Backup Manager", External, Testing, User Data Policy agreed |
| 4 | Add test users | ✅ Done: `tanyamishra12346789@gmail.com`, `mkesharwani125@gmail.com`, `1by22cs111@bmsit.in` |
| 5 | Add scopes | ✅ Done: the 4 scopes below are saved |
| 6 | Create the OAuth client | ✅ Done: Web application `photo-backup-backend-local`, redirect `http://localhost:8080/oauth/callback` |
| 7 | Save credentials to `.env` | ✅ Done: `.env` written (mode 600, git-ignored). The secret can't be viewed again in the Console; if it's lost, create a new secret on the client. |
| 8 | Prepare test accounts and test photos | ⬜ Test accounts ready; test photos optional |

_Note: Google says new client settings can take 5 minutes to a few hours to take effect._

---

## 1. Project ✅
Created and verified project `photo-backup-manager`, number `627633892991`, under `mkesharwani125@gmail.com`.

## 2. Enable APIs ⬜
**APIs & Services → Library**, then enable:
- **Photos Library API**
- **Google Drive API** (used only to read the remaining storage per account)

## 3. Consent screen ⬜
**Google Auth Platform** (https://console.cloud.google.com/auth/overview) → **Get started**:
- App name: `Photo Backup Manager`
- User support email: owner's email
- Audience: **External**
- Contact email: owner's email → agree → **Create**

## 4. Test users ⬜
**Google Auth Platform → Audience**:
- Publishing status: leave as **Testing**.
- **Test users → Add users**: add the **2 Gmail accounts** used for testing → **Save**.

Notes:
- Only these accounts can sign in while the app is in Testing.
- In Testing mode, sign-ins (refresh tokens) **expire after 7 days**. That's fine for testing; just sign in again.
- Use **spare accounts**, not main ones. Uploads use real storage, and the API can't delete them, so they're cleaned up by hand in Google Photos.

## 5. Scopes ⬜
**Google Auth Platform → Data access → Add or remove scopes**. Under **"Manually add scopes"**, paste:
```
https://www.googleapis.com/auth/photoslibrary.appendonly
https://www.googleapis.com/auth/photoslibrary.readonly.appcreateddata
https://www.googleapis.com/auth/photoslibrary.edit.appcreateddata
https://www.googleapis.com/auth/drive.file
```
**Add to table → Update → Save**.

| Scope | Used for |
|---|---|
| `photoslibrary.appendonly` | Upload photos and create albums |
| `photoslibrary.readonly.appcreateddata` | List, verify and download photos our app uploaded |
| `photoslibrary.edit.appcreateddata` | Add or remove our photos in our albums, set descriptions |
| `drive.file` | Read the account's remaining storage (`about.get` → `storageQuota`). Expected to be enough; to be confirmed in the test. |

If a Photos scope doesn't show up in the list, check that step 2 (Photos Library API enabled) was done in **this** project.

## 6. OAuth client ⬜
**Google Auth Platform → Clients → Create client**:
- Application type: **Web application**
- Name: `photo-backup-backend-local`
- **Authorized redirect URIs → Add URI:** `http://localhost:8080/oauth/callback`
- **Create**, then copy the **Client ID** and **Client secret**.

The redirect URI must match exactly: `http`, `localhost`, port `8080`, path `/oauth/callback`, with no trailing slash.

## 7. Save credentials ⬜
Create `/home/manu-kesharwani/work/photo-backup-manager/.env`:
```
GOOGLE_CLIENT_ID=xxxxxxxx.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=xxxxxxxx
GOOGLE_REDIRECT_URL=http://localhost:8080/oauth/callback
```
- **Never commit this file.** `.env` is listed in `.gitignore`.
- Don't paste the secret into chats or docs.

## 8. Test accounts and test photos ⬜
- **2 Gmail accounts** added as test users (step 4). Each is signed in through the backend's local sign-in link in the browser, by the account owner.
- **Test photos (optional, recommended):** put them in `/home/manu-kesharwani/work/photo-backup-manager/testdata/` with phone-like folders:
  ```
  testdata/
    DCIM/Camera/          2–3 photos WITH location (GPS), 1 HEIC, 1 motion photo, 1 short video
    Documents/Bills/      2 photos of documents
    WhatsApp/Images/      1 photo received on WhatsApp + the same photo sent again (different bytes)
  ```
  `testdata/` is also listed in `.gitignore`.

## Local machine (already checked)
- **PostgreSQL 18** is running locally (`/var/run/postgresql:5432`). The backend will use a separate `photobackup` database.
- **Go:** installed as a snap (`/snap/bin/go`), but `go version` printed nothing in Claude's shell. Run `go version` in a normal terminal; if it doesn't print a version, reinstall Go (`sudo snap refresh go`, or the official tarball from https://go.dev/dl/).

## When done
Steps 4–8 complete → the backend proof of concept in [progress/status.md](../progress/status.md) can start.

## Later (not now)
- Before real daily use: switch the consent screen to **In production** (still unverified, ≤100 users). Sign-ins then stop expiring after 7 days.
- After hosting on Cloud Run: add the Cloud Run callback URL as a second redirect URI on the client.
- If publishing: Google OAuth verification for the sensitive Photos scopes, plus a privacy policy.
