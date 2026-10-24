# Problem & Requirements

## The problems (in the user's words, cleaned up)
1. I back up one phone to **several Google accounts**, because one account doesn't have enough free space.
2. I want **one place that knows all my accounts**: which photos are backed up, and which account holds which time period.
3. When account A fills up, I want to back up to account B **only the photos not already backed up**. I don't want to first "Free up space" on the phone to make that work.
4. After a phone change, a **WhatsApp restore** brings back old photos that were already backed up from the old phone. They should stay on the device but **not be uploaded again** to the new account. **New** WhatsApp photos should be backed up.
5. Some folders, such as **Documents** (photos of documents), should always go to my **primary account** and should **never** be deleted by "clear device space".
6. "Clear device space" should work **per folder** (only folders I choose), not on everything.
7. Backups should keep the **same folder grouping** as on my phone, not one mixed-up pile.

Destination: **Google Photos** (not Drive).

## Scope decisions (confirmed)
- **Two services:** a **backend** (accounts, registry, rules, Google API calls) and an **Android app** (native Kotlin: scan, hash, upload bytes, delete). **The backend comes first**, to prove every requirement can be met. The app comes after.
- **Audience:** personal use first, to prove it works. Publishing will be decided later.
- **Only new photos from the day the app starts.** The app doesn't try to find out what was backed up before it was installed. Every photo already on the phone at the first scan is recorded as *existing, don't upload*. Only photos that arrive after that get backed up.
- **Original quality always.** Google Photos API uploads are always original quality, which is what we want.
- **Documents folder holds photos only**, so it goes to Google Photos like every other folder. No Google Drive needed for files.
- **Folders become albums, one album per folder, flat.** Each phone folder becomes one Google Photos album holding that folder's photos, e.g. `DCIM › Camera`, `Documents › Bills`. Albums inside albums are not needed.

## Problem statement
Google Photos limits storage per account, so people with large libraries spread backups across several accounts. Nothing records which account holds which file. Switching accounts re-uploads media that is already backed up. Restoring WhatsApp media on a new phone causes more duplicates. Built-in cleanup can't be limited to chosen folders. This app keeps a central registry of accounts and files, uploads only new media that isn't backed up yet, applies backup and retention rules per folder, and frees device space only where you allow it. You never have to delete media just to switch accounts.

## Functional requirements
| ID | Requirement | Solves |
|---|---|---|
| FR-1 | **Multi-account management.** Register several Google accounts, set roles (primary / overflow), notice when an account is full, and switch destination. | #1 |
| FR-2 | **Central backup registry.** Keep a lasting list of local files: fingerprint, path, date, backup status, destination account, and whether the upload is confirmed. Show coverage per account (date ranges, folders, counts). | #2 |
| FR-3 | **Incremental backup across accounts.** Upload only new files that aren't recorded in *any* account. Never require deleting local files to continue. | #3 |
| FR-4 | **WhatsApp restore dedup.** Media present at the first scan, or whose fingerprint (SHA-256) is already in the registry, is skipped. New WhatsApp media is backed up as usual. Look-alikes with different bytes are uploaded and shown under "Similar photos". | #4 |
| FR-5 | **Folder policies.** Each folder has a target account, include/exclude, and protected/cleanable. Example: Documents goes to the primary account and is protected. | #5 |
| FR-6 | **Selective cleanup.** Delete local copies only for folders you allow, only when the upload is confirmed, never from protected or unbacked files. Show a preview and ask for confirmation first. | #6 |
| FR-7 | **Folder grouping.** Store each file's original path. Create one album per phone folder, named after its path. Show the folder view inside the app. **Restore** a folder's photos back into the same phone folder (first version). | #7 |
| FR-8 | **Similar photos.** Group look-alike photos (different bytes, same picture) so the user can review them and free phone space. | extra |
| FR-9 | **Storage left.** Show the remaining free space for each account. | #1 |

## Core design principle
There are three separate layers, and none may silently override another:
1. **Inventory:** what exists on the device and where it is.
2. **Registry:** what is backed up, to which account, and how well it is verified.
3. **Policy:** what to upload, where to upload it, and what may be deleted.
