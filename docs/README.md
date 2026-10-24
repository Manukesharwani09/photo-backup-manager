# Docs index

Read in this order if you're new (human or agent):

| # | Doc | What it answers |
|---|---|---|
| 1 | [progress/status.md](progress/status.md) | **Where are we right now and what's next?** |
| 2 | [product/requirements.md](product/requirements.md) | What problem we solve; the 7 problems, FR-1…FR-9, scope |
| 3 | [decisions.md](decisions.md) | Every confirmed decision, with date and reason |
| 4 | [architecture/overview.md](architecture/overview.md) | Backend vs app split, data model, flows, API (built vs planned) |
| 5 | [architecture/data-model.md](architecture/data-model.md) | **DB tables ("entities"), relationships, account ownership rules** |
| 5b | [architecture/code-layout.md](architecture/code-layout.md) | Where code lives, conventions, env vars |
| 6 | [architecture/google-apis-feasibility.md](architecture/google-apis-feasibility.md) | What the Google Photos/Drive/Android APIs can and can't do |
| 7 | [open-questions.md](open-questions.md) | Unanswered questions and assumptions still to verify |
| 8 | [setup/local-dev.md](setup/local-dev.md) | How to build, run and connect accounts locally |
| 9 | [setup/google-cloud.md](setup/google-cloud.md) | Google Cloud project, OAuth client, test users |
| 10 | [progress/worklog.md](progress/worklog.md) | What was done, when, and the gotchas found |
| 11 | [app/design-brief.md](app/design-brief.md) | **Android app design brief:** screens, flows, states, constraints (for design agents) |
| — | [product/drive-alternative-discussion.md](product/drive-alternative-discussion.md) | Why Google Drive was not chosen (discussion only) |

## Folder structure
```
docs/
  README.md                  this index
  decisions.md               confirmed decisions (append-only log)
  open-questions.md          open decisions + assumptions to verify
  product/                   what & why (requirements, discussions)
  architecture/              how (design, code layout, API feasibility)
  setup/                     how to run it (local dev, Google Cloud)
  app/                       Android app docs (design brief); the app's code lives in a separate repo
  progress/                  status (current) + worklog (history)
```

## Rules for keeping docs current
- **Work landed:** update `progress/status.md` and add an entry to `progress/worklog.md` (one entry per task).
- **User confirmed a decision:** add a row to `decisions.md`. Never record a proposal as decided.
- **Assumption verified:** remove it from `open-questions.md` and note the result in `progress/status.md`.
- **Endpoint or table added or changed:** update `architecture/overview.md` (API table status, data model note) and, if layout changed, `architecture/code-layout.md`.
