# Photo Backup Manager (backend)

The backend for a personal **multi-account Google Photos backup manager**. You use several Google accounts because each has limited free storage. This system keeps one registry of which photo is backed up in which account. It:
- uploads only **new** photos to the current account (never re-uploads when you switch accounts),
- skips WhatsApp media restored on a new phone,
- sends chosen folders (like Documents) to a fixed account,
- frees phone space only for the folders you allow.

Google Photos remains the storage. An Android app (a separate service, built later) does the on-phone work.

**Status:** backend proof of concept, Go + PostgreSQL. See [docs/progress/status.md](docs/progress/status.md).

- **Docs:** start at [docs/README.md](docs/README.md).
- **Run locally:** see [docs/setup/local-dev.md](docs/setup/local-dev.md).
- **For AI agents:** see [AGENTS.md](AGENTS.md).
