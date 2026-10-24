# Android app: design brief

_For a design agent or designer starting the app's UX/UI. Written 2026-10-24. This is a brief, not a final spec. Anything marked **(proposal)** has not been confirmed by the user._

## 1. What the app is
A personal Android app that manages **photo backup across several Google accounts**. The user has more photos than one free Google account can hold, so they use several.

The app (with its backend) decides:
- **which new photos get uploaded,**
- **which Google account each one goes to,**
- **what may be deleted from the phone.**

**Google Photos stays the place where photos live and are viewed.** Our app is the control panel, not a full gallery.

**One user, personal use.** No sign-up or social features. Publishing to the Play Store may come later.

Read for background (short):
- [../product/requirements.md](../product/requirements.md): the 7 problems and FR-1…FR-9
- [../decisions.md](../decisions.md): confirmed decisions

## 2. The user's problems (design must solve all of these)
| # | Problem | What the user needs to see or do |
|---|---|---|
| 1 | Uses several Google accounts because each is too small | Add accounts, see free space per account, know which one is receiving backups now |
| 2 | Doesn't know which account has which photos or time period | Per account: when it was active, which photo dates it holds, which folders. "Where is my photo from March 2024?" |
| 3 | When an account is full, wants to continue on another without re-uploading or clearing the phone | Clear "Account full → pick next account" moment. Reassurance that nothing is re-uploaded and nothing is deleted |
| 4 | After a phone change, restored WhatsApp photos shouldn't be uploaded again; new ones should | Explain the "start point" (photos already on the phone when the app starts are skipped). Show them as *Existing, skipped* |
| 5 | Some folders (e.g. Documents) must always go to the primary account and never be cleaned up | Per-folder rules: target account and **Protected** |
| 6 | Free up space only for chosen folders | Folder-based cleanup with preview and confirmation |
| 7 | Keep the phone's folder structure | Browse by the phone's folders inside the app. Each folder becomes one Google Photos album named after its path (`DCIM › Camera`) |
| + | Similar photos (look-alikes with different bytes) | Review groups and remove extra copies |
| + | Restore a folder back to the phone | Pick a folder, then download it back into the same folder |

## 3. Key concepts and vocabulary (use these words in the UI)
- **Primary account:** the main account. Protected folders such as Documents always go here. Exactly one.
- **Overflow accounts:** extra accounts used when the current one is full.
- **Active account:** the account currently receiving new backups. It has an **active period** (from date → to date, plus a reason it ended: Full / Switched).
- **Start point:** the moment the app first scanned the phone. Photos already there are **Existing, skipped** and never uploaded.
- **Folder rule:** per phone folder:
  - **Back up:** on / off
  - **Send to:** Auto (the active account) or a specific account
  - **Cleanup:** Allowed / Protected
- **Photo status** (badge per photo):

| Status | Meaning |
|---|---|
| Existing, skipped | Was on the phone at the start point; never uploaded |
| Pending | Waiting to upload |
| Uploading | In progress |
| Backed up | Uploaded and confirmed by Google, with which account |
| Failed | Upload failed; will retry |
| Excluded | Its folder's backup is off |
| Removed from Google | The user deleted it in Google Photos; no longer counts as backed up |

## 4. Screens to design
Proposed information architecture, open to change. **(proposal)** Bottom navigation with: **Home · Folders · Accounts · Tools**.

### 4.1 Onboarding (first run)
1. **Welcome:** what the app does in one or two lines, and "Google Photos stays your photo library."
2. **Permissions:**
   - Photos & videos (required)
   - Notifications (backup progress)
   - "Unrestricted battery" (so backups aren't killed; explain why)
   - Optional: **Media management**, which lets cleanup skip the per-batch Android confirmation
3. **Turn off Google Photos backup.** Our app must be the only uploader. Show short steps and a "Done" check.
4. **Connect Google accounts:**
   - Primary first, then optional overflow accounts.
   - Sign-in happens in a **browser tab** (Google's own screens; we can't style them).
   - Warn the user they'll see "Google hasn't verified this app" (Advanced → continue) and must **tick all permission boxes**.
5. **Start point and first scan.** Explain that "Photos already on your phone won't be uploaded; from now on, new photos are backed up." Show scan progress, because a large library can take minutes.
6. **Folder setup (optional):** suggest common rules, e.g. Documents → Primary + Protected.

### 4.2 Home / Dashboard
- **Backup status:** up to date / N pending / uploading X of Y / paused (no Wi-Fi, not charging) / error.
- **Active account card:** email, free space bar, "receiving backups since <date>".
- **Alerts, as action cards:**
  - Account full → choose next
  - Account needs reconnecting
  - Similar photos found
  - Photos waiting in a "To delete" album
  - Permission missing
- **Quick actions:** Free up space, Restore folder, Find a photo.

### 4.3 Folders
- The **phone's folder tree** (`DCIM › Camera`, `Pictures › Documents › Bills`, `WhatsApp › Images` …).
- Per folder row:
  - photo count by status (backed up / pending / skipped)
  - the rule chips (Back up on/off, Send to, Protected)
- **Folder detail:** edit the rule, a grid of photos with status badges and an account indicator, plus actions: Restore folder, Free up space (if Allowed).

### 4.4 Photo detail
- Preview (local if still on the phone, otherwise a thumbnail), status, **which account**, original path, date taken.
- Actions:
  - **Open in Google Photos** (opens that account's copy)
  - **Share** (works even if deleted locally; the app fetches it)
  - Delete from phone (only if backed up and not Protected)

### 4.5 Accounts
- List: email, role (Primary / Overflow), **free space**, Active badge, and Full/Reconnect warnings.
- **Account detail:**
  - active periods (timeline)
  - photo date range ("photos taken Mar 2019 → Jun 2026")
  - number of items
  - folders it holds
- Add account; set which overflow account is next; mark as full manually.
- **"Find photos by date":** pick a month or range → see which accounts hold photos from then, and jump to them.

### 4.6 Free up space (cleanup)
1. Pick folders. Only **Allowed** folders are selectable; **Protected** ones show a lock and are disabled.
2. **Preview:** count, total size, thumbnails. Only photos **confirmed backed up** are included. Explain why others are excluded (pending, protected).
3. **Confirm.** Android then shows **its own system dialog** (we can't style it), unless Media management is granted.
4. **Result:** space freed, with "Photos are still in Google Photos (Account A)".

### 4.7 Similar photos
- Groups of look-alike photos (e.g. a WhatsApp photo received twice), with a suggested "keep" choice.
- **Remove phone copies:** direct, through Android's delete dialog.
- **Remove cloud copies:** Google gives apps no way to delete. The app moves them into a **"🗑 To delete"** album in each account, then shows a per-account card: "Account A: 12 photos to delete → Open Google Photos". The user selects all and deletes there. The app later detects they're gone.

### 4.8 Restore folder
- Pick a folder → summary: N photos, size, from which accounts → Restore → progress → done.
- Explain that restored photos go back into the **same phone folder**.

### 4.9 Settings
- Upload only on Wi-Fi / only while charging.
- Notifications.
- Battery optimisation status.
- Connected accounts (reconnect).
- About the start point (read-only date).
- Backend connection (developer section for now).

## 5. States and edge cases every screen must handle
| Situation | What the UI should do |
|---|---|
| **Active account becomes full during a backup** | Pause and alert: "Account A is full. Continue with Account B?" Nothing is re-uploaded or deleted |
| **Account sign-in expired** (happens every 7 days in the current testing setup) | "Reconnect account" card. Backups to that account pause |
| **User unticked a permission during sign-in** | Explain which permission is missing, then retry |
| **No accounts connected** | Empty state → connect primary |
| **Offline / waiting for Wi-Fi or charging** | Show "paused, waiting for Wi-Fi", not an error |
| **App or phone restarted mid-upload** | Resumes automatically; show "resumed" rather than starting over |
| **First scan of a huge library** | Progress with counts; the app stays usable |
| **Photos permission revoked** | Blocking state with a "Grant again" action |
| **User cancels the Android delete dialog** | Nothing deleted; say so neutrally |
| **Photo deleted by the user inside Google Photos** | Status *Removed from Google*. If it's still on the phone, it becomes Pending again |
| **Protected folder in cleanup** | Visible but locked, with an explanation |
| **Restored WhatsApp media after a phone change** | Shows as *Existing, skipped*, with an explanation of why |
| **Look-alike photos** | Both are uploaded and also flagged in Similar photos (never silently skipped) |

## 6. Platform constraints (design around these, don't fight them)
- **Google Photos viewing happens in Google Photos.** We open the photo there and don't rebuild a full gallery.
- **Google sign-in screens and the "unverified app" warning** are Google's. We can only prepare the user.
- **Deleting from the phone** uses Android's system confirmation dialog (unstyled, one per batch) unless Media management is granted.
- **No cloud delete and no share-link creation via the API.** Hence the "To delete" album flow, and "Open in Google Photos" for sharing links.
- **Albums are flat.** The folder tree exists only inside our app; in Google Photos the user sees albums like `DCIM › Camera`.
- **Google Photos can't tell our app about photos uploaded before the app existed.** We never show "already backed up before"; we only show *Existing, skipped*.
- **Uploads are original quality** and use the account's storage.

## 7. Data the backend will provide (provisional)
The backend is being built and endpoint shapes **may change**, so design against the *information*, not exact fields.

| Screen | Data available |
|---|---|
| Accounts | email, role, active, full, free/used/limit storage, active periods (start, end, reason), photo-date range, item count, folders |
| Home | pending / uploading / failed counts, last backup time, alerts |
| Folders | folder path, rule (backup, target account, cleanup), counts per status |
| Photo | status, account, original path, date taken, Google Photos link |
| Cleanup | eligible photos for chosen folders (count, size, list) |
| Similar | groups of look-alikes, with account and phone presence per photo |
| Restore | photos of a folder with account, size, download link |

**Update 2026-10-24:** every row above now has a working backend endpoint (proof of concept, tested end-to-end and partly live against real Google). See [../architecture/overview.md](../architecture/overview.md#backend-api). Fields may still change before the app ships.

Two confirmed facts that affect design:
- Google strips GPS from downloads, so restore writes it back silently. No user action is needed.
- Albums can't be deleted through the API, so the "🗑 To delete" album stays in Google Photos after it's emptied. The copy should say so.

## 8. Out of scope for v1
- A full photo gallery, editing, or AI search (Google Photos does these).
- iOS.
- Multi-user / family sharing inside the app.
- Backing up anything that isn't a photo or video.

## 9. What we'd like from the design work
1. **User flows:** onboarding, daily backup, account full → switch, cleanup, similar photos, restore, reconnect account.
2. **Wireframes** for every screen in §4, including the empty, loading, error and paused states from §5.
3. **Component list:** status badges, account card with storage bar, folder row with rule chips, alert cards.
4. **Copy suggestions** for the tricky explanations: start point, unverified-app warning, protected folders, "To delete" album.
5. **Open questions for the user** (record answers in [../decisions.md](../decisions.md) only once confirmed):
   - App name and visual style (Material 3? dark mode?)
   - Navigation pattern (bottom nav vs. single dashboard)
   - How much photo browsing the app itself should offer, given that Google Photos is the viewer
