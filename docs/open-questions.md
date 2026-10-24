# Open questions & unverified assumptions

When one is answered or verified, move the outcome to [decisions.md](decisions.md) (if it's a user decision) or to [progress/status.md](progress/status.md) (if it's a test result), and delete it here.

## Open user decisions
_None right now._

## Assumptions to verify in the proof of concept
| # | Assumption | How it's checked | Status |
|---|---|---|---|
| A3 | `baseUrl=d` downloads strip GPS | Upload a photo with GPS, download it, compare EXIF | ⬜ |
| A4 | Google Photos search finds text in the item `description` | Search the original path in Google Photos | ⬜ |
| A5 | Motion photos and HEIC keep their motion and format after an API upload | Upload a sample and view it in Google Photos | ⬜ |
| A6 | The exact error Google returns when an account is full | Observe it (hard to force; may need a nearly full test account). Meanwhile `google.IsStorageFull` matches code 8 or storage/quota wording | ⬜ |

## Verified (moved out)
- **A2, verified 2026-10-24:** an upload token created by the phone (with a backend-issued access token) is accepted by the backend's `mediaItems:batchCreate`. The split upload design works.
- **A1, verified 2026-10-24:** the `drive.file` scope is enough for `about.get` → `storageQuota`.
- **A7, verified 2026-10-24:** requesting `openid` + `userinfo.email` (not declared on the consent screen) works in Testing mode.

## For later (publishing)
- Google OAuth verification for the sensitive Photos scopes, plus a privacy policy (we store users' tokens and photo metadata).
- Play Store review of the media and "Media management" permissions.
- A request for a higher Photos API quota, since the 10k requests per day are shared by all users.
- Switch the consent screen to **In production**, so tokens stop expiring after 7 days.
