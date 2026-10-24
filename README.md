# Photo Backup Manager

An Android app that sits in front of **Google Photos** and manages backup across **several Google accounts**. It decides:

- which photos get uploaded,
- which account each photo goes to,
- what may be deleted from the phone.

Google Photos stays the place where the files are stored. The app keeps its own record (the registry) of where every photo is, so it never uploads a photo twice when you switch accounts. It also skips WhatsApp media restored on a new phone, sends set folders to a set account, and only deletes local copies of folders you allow, after their backup is confirmed.

**Structure:** two services, a **backend** (the brain: accounts, registry, rules, Google API) and an **Android app** (scan, hash, upload bytes, delete).

**Status:** feasibility stage, backend first. No code yet.

## Docs
1. [Problem & requirements](docs/01-problem-and-requirements.md)
2. [Feasibility: what Google / Android / third parties support](docs/02-feasibility-third-party.md)
3. [Architecture](docs/03-architecture.md)
4. [Open questions & next step](docs/04-open-questions.md)
