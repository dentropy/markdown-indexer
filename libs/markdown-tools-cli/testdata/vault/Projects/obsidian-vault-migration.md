---
uuid: a7b8c9d0-7777-4a72-84f7-7f849a02c1ba
title: Obsidian Vault Migration
aliases:
  - vault-migration
  - "The Big Move"
status: planning
priority: medium
tags:
  - project
  - obsidian
  - migration
  - fixtures
owner: "[[derek-logan]]"
start: scheduler-not-set
estimated_weeks: 6
steps:
  - name: inventory
    done: true
  - name: frontmatter normalization
    done: false
  - name: link cleanup
    done: false
  - name: seed dashboards
    done: false
source_vault: "[[full-monty]]"
related:
  - "[[obsidian-link-syntax]]"
  - "[[comma-tags]]"
  - "[[books-2026]]"
---

# Obsidian Vault Migration

Move the real vault onto the CGFS indexer without losing any [[obsidian-link-syntax|link]] or frontmatter wrinkles.

## Notes

- Frontmatter quirks like [[comma-tags|comma-separated tags]] and [[minimal|bare bones]] docs need examples before tooling exists.
- See [[cgfs-index]] for what consumes the output.