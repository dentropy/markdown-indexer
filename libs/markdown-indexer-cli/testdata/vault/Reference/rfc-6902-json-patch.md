---
uuid: e1f2a3b4-bbbb-4eb6-c8cb-bdc83e46a5fe
title: "RFC 6902 — JSON Patch"
aliases:
  - "JSON Diff RFC"
doctype: rfc
status: published
published: 2013-04
source: https://datatracker.ietf.org/doc/html/rfc6902
authors:
  - P. Bryan
  - M. Nottingham
tags:
  - reference
  - rfc
  - json
  - diff
complexity: 2
sections:
  - section: 4
    title: Operations
    ops: [add, remove, replace, move, copy, test]
  - section: 4.1
    title: add
example_patch: |
  [
    { "op": "add", "path": "/a/b/c", "value": "foo" },
    { "op": "remove", "path": "/x" }
  ]
related:
  - "[[optimistic-concurrency]]"
  - "[[cgfs-index]]"
---

# RFC 6902 — JSON Patch

Used to record document history between versions of [[content-addressed-storage|CAS]] snapshots.

- The diff engine for [[cgfs-index]] version history.
- Text at [datatracker](https://datatracker.ietf.org/doc/html/rfc6902).
- Pairs nicely with [[optimistic-concurrency]].
- Local copy of norms: [assets/rfc6902.txt](assets/rfc6902.txt).