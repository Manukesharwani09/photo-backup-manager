# 01 — Problem & Requirements

## The problems (in the user's words, cleaned up)
1. I back up one phone to **several Google accounts**, because one account doesn't have enough free space.
2. I want **one place that knows all my accounts**: which photos are backed up, and which account holds which time period.
3. When account A fills up, I want to back up to account B **only the photos not already backed up**. I don't want to first "Free up space" on the phone to make that work.
4. After a phone change, a **WhatsApp restore** brings back old photos that were already backed up from the old phone. They should stay on the device but **not be uploaded again** to the new account. **New** WhatsApp photos should be backed up.
5. Some folders, such as **Documents**, should always go to my **primary account** and should **never** be deleted by "clear device space".
6. "Clear device space" should work **per folder** (only folders I choose), not on everything.
7. Backups should keep the **same folder structure** as on my phone, not one mixed-up pile.

Destination: **Google Photos** (not Drive).

## Problem statement
Google Photos limits storage per account, so people with large libraries spread backups across several accounts. Nothing records which account holds which file. Switching accounts re-uploads media that is already backed up. Restoring WhatsApp media on a new phone causes more duplicates. Built-in cleanup can't be limited to chosen folders. This app keeps a central registry of accounts and files, uploads only media that isn't backed up yet, applies backup and retention rules per folder, and frees device space only where you allow it. You never have to delete media just to switch accounts.

## Functional requirements
| ID | Requirement | Solves |
|---|---|---|
| FR-1 | **Multi-account management.** Register several Google accounts, set roles (primary / overflow), notice when an account is full, and switch destination. | #1 |
| FR-2 | **Central backup registry.** Keep a lasting list of local files: fingerprint, path, date, backup status, destination account, and whether the upload is confirmed. Show coverage per account (date ranges, folders, counts). | #2 |
| FR-3 | **Incremental backup across accounts.** Upload only files that aren't recorded in *any* account. Never require deleting local files to continue. | #3 |
| FR-4 | **WhatsApp restore dedup.** Recognise restored historical media by fingerprint and metadata, and skip it. Back up new WhatsApp media as usual. Let the user decide unclear matches. | #4 |
| FR-5 | **Folder policies.** Each folder has a target account, include/exclude, and protected/cleanable. Example: Documents goes to the primary account and is protected. | #5 |
| FR-6 | **Selective cleanup.** Delete local copies only for folders you allow, only when the upload is confirmed, never from protected or unbacked files. Show a preview and ask for confirmation first. | #6 |
| FR-7 | **Folder structure.** Store each file's original path. Mirror folders as Google Photos albums, which are flat and only approximate a folder tree. | #7 |

## Core design principle
There are three separate layers, and none may silently override another:
1. **Inventory:** what exists on the device and where it is.
2. **Registry:** what is backed up, to which account, and how well it is verified.
3. **Policy:** what to upload, where to upload it, and what may be deleted.
