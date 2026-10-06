---
uuid: f2a3b4c5-cccc-4fc7-d9dc-ce940f57b60f
title: "Nostr NIP-01"
aliases:
  - "NIP-01"
  - "nip01"
doctype: spec
status: draft
source: https://github.com/nostr-protocol/nips/blob/master/01.md
authors:
  - fiatjaf
tags:
  - reference
  - nostr
  - spec
kind: 1
content_types: [text/note, memo]
event_id_example: "0e02b9e10d1b6e639429bc9a2b7f5bdc2a8a6e5a1f4f40d0c5e55d5f9cbaa1b0"
relays_wanted:
  - wss://relay.damus.io
  - wss://nos.lol
bip340: true
related:
  - "[[nostr-dot-local-deployment]]"
  - "[[cgfs-index]]"
  - "[[tarek-ziyad]]"
---

# Nostr NIP-01

The event envelope that CGFS mirrors: kind, pubkey, sig, id, and signed content.

- Implemented locally by [[nostr-dot-local-deployment]].
- Consumed by [[cgfs-index]] as its event stream foundation.
- On-chain weirdness lives in [[holochain-dht]].