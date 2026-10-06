---
uuid: c9d0e1f2-9999-4c94-a6b9-9ba61c24e3dc
title: Content Addressed Storage
aliases:
  - CAS
  - "content addressing"
created: 2026-09-03
updated: 2026-09-12T16:45:00
tags:
  - note
  - storage
  - ipld
  - sha256
related:
  - "[[optimistic-concurrency]]"
  - "[[cgfs-index]]"
  - "[[derek-logan]]"
  - "[[holochain-dht]]"
cite:
  source: https://ipld.io
  encoding: DAG-CBOR
  hash: sha2-256
prefer:
  - immutable
  - deduplicated
  - "content-addressed"
---

# Content Addressed Storage

Every line hashes to a DID; every document body hashes to a CID. Then nothing can silently change.

- Used by [[cgfs-index|the indexer]].
- Paired with [[optimistic-concurrency]] for safe merging.
- DHT storage notes at [[holochain-dht]].
- Embedded visualization: ![[cas-diagram.png]].