# Open questions & unverified assumptions

When one is answered or verified, move the outcome to [decisions.md](decisions.md) (if it's a user decision) or to [progress/status.md](progress/status.md) (if it's a test result), and delete it here.

## Open user decisions
_None right now._

## Assumptions to verify in the proof of concept
| # | Assumption | How it's checked | Status |
|---|---|---|---|
| A1 | The `drive.file` scope is enough for `about.get` → `storageQuota` | `GET /accounts` shows storage for a connected account | ⬜ Waiting for the first account sign-in |
| A2 | An upload token created by the "phone" (using an access token the backend issued) can be used by the backend in `mediaItems.batchCreate` | Proof of concept step 3 | ⬜ |
| A3 | `baseUrl=d` downloads strip GPS | Upload a photo with GPS, download it, compare EXIF | ⬜ |
| A4 | Google Photos search finds text in the item `description` | Search the original path in Google Photos | ⬜ |
| A5 | Motion photos and HEIC keep their motion and format after an API upload | Upload a sample and view it in Google Photos | ⬜ |
| A6 | The exact error Google returns when an account is full | Observe it (hard to force; may need a nearly full test account) | ⬜ |
| A7 | Requesting `openid` + `userinfo.email` (not declared on the consent screen) works in Testing mode | First sign-in succeeds | ⬜ |

## For later (publishing)
- Google OAuth verification for the sensitive Photos scopes, plus a privacy policy (we store users' tokens and photo metadata).
- Play Store review of the media and "Media management" permissions.
- A request for a higher Photos API quota, since the 10k requests per day are shared by all users.
- Switch the consent screen to **In production**, so tokens stop expiring after 7 days.
