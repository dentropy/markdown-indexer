---
uuid: f6a7b8c9-6666-4f61-73e6-6e738a910baf
title: nostr dot local deployment
aliases:
  - "Nostr Local Deployment"
  - "npm run nostr"
status: shipped
archived: false
tags:
  - project
  - nostr
  - deployment
  - local
version: 2.4.1
registry: npm
packageName: "nostr-dot-local"
maintainers:
  - "[[derek-logan]]"
docs:
  readme: https://github.com/cgfs/nostr-dot-local#readme
  spec: https://github.com/cgfs/nostr-dot-local/blob/main/docs/SPEC.md
ports:
  http: 8080
  relay: 7000
  p2p: 7001
related:
  - "[[derek-logan]]"
  - "[[nostr-nip-01]]"
  - "[[cgfs-index]]"
tags_extra: [deploy, "relay", self-hosted]
---

# nostr dot local

Spin up a full [[nostr-nip-01]] stack on localhost with a single command: `npx nostr-dot-local`.

- Talks to local relays seeded from the experiment docs in [[cgfs-index]].
- Full dependency picture in [the spec](https://github.com/cgfs/nostr-dot-local/blob/main/docs/SPEC.md).
- People: [[derek-logan]], [[tarek-ziyad]].