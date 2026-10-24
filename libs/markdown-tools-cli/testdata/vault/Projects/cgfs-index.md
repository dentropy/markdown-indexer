---
uuid: e5f6a7b8-5555-4e50-62d5-5d627a8f90ae
title: CGFS Index
aliases:
  - "CGFS markdown-indexer"
  - the-indexer
status: in-progress
draft: false
priority: high
tags:
  - project
  - cgfs
  - indexer
  - markdown
leads:
  - "[[derek-logan]]"
  - "[[tarek-ziyad]]"
start: 2026-08-15
target: 2026-12-01
milestones:
  - 2026-08-15: kickoff
  - 2026-09-15: fixture vault
  - 2026-12-01: public beta
repositories:
  - name: markdown-indexer
    url: https://github.com/cgfs/markdown-indexer
  - name: cgfs-monorepo
    url: https://github.com/cgfs/cgfs-monorepo
spec:
  schema:
    document: "cid://document"
    link: "cid://link"
  link_labels:
    - website
    - assets
    - internal
related:
  - "[[content-addressed-storage]]"
  - "[[optimistic-concurrency]]"
  - "[[rfc-6902-json-patch]]"
  - "[[obsidian-link-syntax]]"
---

# CGFS Index

Indexes an Obsidian vault into content-addressed documents with a Nostr-flavored event stream.

- Depends on [[optimistic-concurrency]] notes and [[rfc-6902-json-patch]] diffs.
- Feeds off the [[obsidian-link-syntax|link grammar]] fixtures.
- See [[nostr-nip-01]] for the event envelope we mirror.

Ship diagram: ![](assets/cgfs-architecture.png). Related reading at [the plan](https://example.com/docs/cgfs-plan).