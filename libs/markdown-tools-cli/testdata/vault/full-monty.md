---
uuid: 70f3d0e2-9a1c-4b7f-8e6a-2c4d8a9b1f30
title: The Full Monty
aliases:
  - "FM"
  - "Rich Frontmatter Sample"
tags:
  - test
  - fixture
  - "full-monty"
  - frontmatter
  - all-the-things
cssclass: report
created: 2026-09-10
modified: 2026-09-14T18:22:11
published: false
draft: false
status: collecting
author:
  name: Derek Logan
  email: derek@localhost
  npub: npub1derekloganovauthor0000000000
category: meta
priority: 1
age: 37
pi: 3.14159
small: 1e-09
negative: -42
truthy: true
falsey: false
nothing: null
weather:
  temp: 19.5
  condition: overcast
  feels_like: 18.0
metrics:
  scans: 142
  uptime: 99.97
  precision: 0.9812
repositories:
  - name: markdown-indexer
    url: https://github.com/cgfs/markdown-indexer
    language: go
    stars: 12
  - name: cfs-on-nostr
    url: https://github.com/cgfs/cfs-on-nostr
    language: typescript
    stars: 41
milestones:
  - 2026-06-01:
      event: first commit
      done: true
  - 2026-07-15:
      event: schema freeze
      done: true
  - 2026-09-30:
      event: public beta
      done: false
quote: |
  Every line of a note is a content-addressed atom,
  and every atom is worth remembering.
summary: >
  A note that exercises every YAML feature we care about:
  scalars, sequences, mappings, folded and literal blocks,
  booleans, nulls, and a few surprises.
camelCase: testCamel
snake_case: test_snake
kebab-case: test-kebab
"quoted key with spaces": works
"colon:inside": also-works
unicode: "héllo → wörld 🧪"
tags-from-template:
  - "#experiment"
  - "#cgfs"
links:
  - "[[content-addressed-storage]]"
  - "[[optimistic-concurrency]]"
  - "[[obsidian-link-syntax]]"
cover: "![[banner.png]]"
ingredients:
  - flour
  - "water, filterered"
  - 12
  - 3.5
  - true
---

# The Full Monty

A deliberately overloaded document. It references [[content-addressed-storage]], embeds a diagram with ![[mememap.png]], and links out to [DAG-CBOR spec](https://ipld.io/specs/codecs/dag-cbor/).

Some raw markdown image embed: ![](assets/graph-summary.png). A relative markdown link that isn't a wiki link: [local](reference/nostr-nip-01.md).

That stray `![[note-embed-test]]` should parse as an asset embed even though no such note exists.