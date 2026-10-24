---
uuid: b8c9d0e1-8888-4b83-95a8-8a950b13d2cb
title: Optimistic Concurrency
aliases:
  - opt-conc
  - "version conflict notes"
created: 2026-09-05
updated: 2026-09-15T09:10:00
tags:
  - note
  - concurrency
  - crdt
  - nostr
related:
  - "[[nostr-nip-01]]"
  - "[[content-addressed-storage]]"
  - "[[derek-logan]]"
mode: thinking
prominent_thinkers:
  - name: Derek
    takes: "events are the source of truth"
  - name: Tarek
    takes: "content addressing beats sequencing"
conflict_strategies:
  lww: last-write-wins
  merkle: merge-by-hash
  event-id: nostr-event-id
---

# Optimistic Concurrency

Most notes collide. Handle it gracefully.

- Relevant to [[cgfs-index]], which merges edits from [[nostr-nip-01|Nostr events]].
- Warriors: [[derek-logan]] and [[tarek-ziyad]].
- Background: [[content-addressed-storage]].
- The diff format lives in [[rfc-6902-json-patch]].