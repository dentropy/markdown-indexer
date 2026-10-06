#### Determinism

The index is content addressed, so two runs over the same vault must produce
byte-identical output. This is the property most worth checking by hand.

``` bash
./markdown-indexer-cli -dir testdata -force -all > /tmp/run1.json
./markdown-indexer-cli -dir testdata -force -all > /tmp/run2.json
diff /tmp/run1.json /tmp/run2.json && echo "documents: stable"

./markdown-indexer-cli -dir testdata -force -wikilinks -all > /tmp/wl1.json
./markdown-indexer-cli -dir testdata -force -wikilinks -all > /tmp/wl2.json
diff /tmp/wl1.json /tmp/wl2.json && echo "wikilinks: stable"
```

#### Share filtering

Every operation loads only documents whose front matter sets `share: true`
unless `-all` is passed. Note `-force` throughout: `testdata/vault/` holds a
deliberately broken file, and without `-force` it aborts the run.

``` bash
# The shared half only.
./markdown-indexer-cli -dir testdata -force | jq -r 'keys[]'

# The whole vault.
./markdown-indexer-cli -dir testdata -force -all | jq -r 'keys[]'
```

`testdata/no-frontmatter.md` has no front matter at all, so it appears only under
`-all`, keyed by the empty string.

#### Wikilink graph

``` bash
./markdown-indexer-cli -dir testdata -force -wikilinks -all | jq -r 'to_entries[] | "\(.value.label)\t\(.value.title)\t\(.value.to_document_id)"'
```

Edges are keyed by content address, so identical `[[links]]` collapse to one
entry. Unresolved targets keep their raw text as `to_document_id`.

#### Vault check

Reports structural problems and exits non-zero, which is what makes it usable as
a CI gate.

``` bash
./markdown-indexer-cli -dir testdata/vault -vaultcheck
echo "exit=$?"
```

Expect two problems: broken front matter in `broken-frontmatter.md`, and the
UUID shared by `duplicate-a.md` and `duplicate-b.md`. A clean vault prints
`vault check: ok` and exits zero.

#### Duplicate detection and repair

Both need `-force` on this fixture, for the same reason as above.

``` bash
# Report duplicates, still writing the index.
./markdown-indexer-cli -dir testdata/vault -checkdups -force -all
echo "exit=$?"

# Assign a fresh UUID to every document sharing one. Answers anything but
# y/yes abort without touching a file.
```

`testdata/vault` is checked into git, so run `-stripdups` on a copy:

``` bash
cp -r testdata/vault /tmp/strip-scratch
echo y | ./markdown-indexer-cli -dir /tmp/strip-scratch -stripdups -force
./markdown-indexer-cli -dir /tmp/strip-scratch -vaultcheck
```

The duplicate is gone, but the command still exits 1: `broken-frontmatter.md` is
still broken. Check that the duplicate line specifically has disappeared.

#### Broken front matter

By default the first broken file aborts the run. `-force` skips it and logs a
line per file.

``` bash
# Aborts.
./markdown-indexer-cli -dir testdata/vault; echo "exit=$?"

# Skips and carries on.
./markdown-indexer-cli -dir testdata/vault -force -all | jq -r 'keys[]'
```

#### Memory estimate

``` bash
./markdown-indexer-cli -dir testdata -all -memusage > /dev/null
```