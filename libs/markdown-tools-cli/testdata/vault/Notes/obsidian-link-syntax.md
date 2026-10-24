---
uuid: d0e1f2a3-aaaa-4da5-b7ca-acb72d35f4ed
title: Obsidian Link Syntax
aliases:
  - links
  - "wiki link syntax"
created: 2026-09-15
updated: 2026-09-15T13:30:00
tags:
  - note
  - obsidian
  - links
  - fixtures
region: vault
cssclass: link-reference
sections:
  internal: "[[target]]"
  internal_alias: "[[target|display text]]"
  embed_note: "![[note-name]]"
  embed_asset: "![[image.png]]"
  markdown_link: "[text](https://example.com)"
  image_embed: "![](assets/image.png)"
extraction_rules:
  - "[[...]]  -> internal link"
  - "![[...]] -> asset embed"
  - "https links -> website"
  - "![](...)  -> asset"
related:
  - "[[cgfs-index]]"
  - "[[full-monty]]"
---

# Obsidian Link Syntax

The grammar every extractor tests against.

| Style | Example | Parses as |
| --- | --- | --- |
| internal | [[derek-logan]] | internal |
| internal w/ alias | [[cgfs-index\|the index]] | internal |
| asset embed | ![[cas-diagram.png]] | assets |
| note embed | ![[full-monty]] | assets |
| external | [ipld.io](https://ipld.io) | website |
| markdown image | ![](assets/cgfs-architecture.png) | assets |

See [[cgfs-index]] for the consumer and [[full-monty]] for a doc that exercises every style at once.