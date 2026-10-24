#### Determinism

The index is content addressed, so two runs over the same vault must produce
byte-identical output. This is the property most worth checking by hand.

``` bash
./markdown-tools-cli -dir testdata -force -all > /tmp/run1.json
./markdown-tools-cli -dir testdata -force -all > /tmp/run2.json
diff /tmp/run1.json /tmp/run2.json && echo "documents: stable"

./markdown-tools-cli -dir testdata -force -wikilinks -all > /tmp/wl1.json
./markdown-tools-cli -dir testdata -force -wikilinks -all > /tmp/wl2.json
diff /tmp/wl1.json /tmp/wl2.json && echo "wikilinks: stable"
```

#### Share filtering

Every operation loads only documents whose front matter sets `share: true`
unless `-all` is passed. Note `-force` throughout: `testdata/vault/` holds a
deliberately broken file, and without `-force` it aborts the run.

``` bash
# The shared half only.
./markdown-tools-cli -dir testdata -force | jq -r 'keys[]'

# The whole vault.
./markdown-tools-cli -dir testdata -force -all | jq -r 'keys[]'
```

`testdata/no-frontmatter.md` has no front matter at all, so it appears only under
`-all`, keyed by the empty string.

#### Wikilink graph

``` bash
./markdown-tools-cli -dir testdata -force -wikilinks -all | jq -r 'to_entries[] | "\(.value.label)\t\(.value.title)\t\(.value.to_document_id)"'
```

Edges are keyed by content address, so identical `[[links]]` collapse to one
entry. Unresolved targets keep their raw text as `to_document_id`.

#### Vault check

Reports structural problems and exits non-zero, which is what makes it usable as
a CI gate.

``` bash
./markdown-tools-cli -dir testdata/vault -vaultcheck
echo "exit=$?"
```

Expect two problems: broken front matter in `broken-frontmatter.md`, and the
UUID shared by `duplicate-a.md` and `duplicate-b.md`. A clean vault prints
`vault check: ok` and exits zero.

#### Duplicate detection and repair

Both need `-force` on this fixture, for the same reason as above.

``` bash
# Report duplicates, still writing the index.
./markdown-tools-cli -dir testdata/vault -checkdups -force -all
echo "exit=$?"

# Assign a fresh UUID to every document sharing one. Answers anything but
# y/yes abort without touching a file.
```

`testdata/vault` is checked into git, so run `-stripdups` on a copy:

``` bash
cp -r testdata/vault /tmp/strip-scratch
echo y | ./markdown-tools-cli -dir /tmp/strip-scratch -stripdups -force
./markdown-tools-cli -dir /tmp/strip-scratch -vaultcheck
```

The duplicate is gone, but the command still exits 1: `broken-frontmatter.md` is
still broken. Check that the duplicate line specifically has disappeared.

#### Writing front matter

Both write modes modify files in place, so use a scratch copy. `-initfrontmatter`
fills gaps without touching existing values; `-setshare` forces the permission
and still adds missing UUIDs.

``` bash
cp -r testdata/vault /tmp/frontmatter-scratch

# Add missing uuid/share; only files with front matter can be read without -force.
./markdown-tools-cli -dir /tmp/frontmatter-scratch -initfrontmatter -force -q > /dev/null

# Force every matched document private, recursively.
./markdown-tools-cli -dir /tmp/frontmatter-scratch -setshare -share=false -force -q > /dev/null

# Every document is private now, so the default share filter loads nothing.
./markdown-tools-cli -dir /tmp/frontmatter-scratch -force | jq 'length'
```

Repeat `-setshare` and confirm the files are byte-identical (the command skips
unchanged files):

``` bash
cp -r /tmp/frontmatter-scratch /tmp/frontmatter-before
./markdown-tools-cli -dir /tmp/frontmatter-scratch -setshare -share=false -force -q > /dev/null
diff -r /tmp/frontmatter-before /tmp/frontmatter-scratch && echo "front matter: stable"
```

#### Broken front matter

By default the first broken file aborts the run. `-force` skips it and logs a
line per file.

``` bash
# Aborts.
./markdown-tools-cli -dir testdata/vault; echo "exit=$?"

# Skips and carries on.
./markdown-tools-cli -dir testdata/vault -force -all | jq -r 'keys[]'
```

#### Quiet output

Each skipped file is reported on stderr, so stdout is clean JSON without any
extra flags. `-quiet` (`-q`) silences the notices.

``` bash
# The notice is on stderr, so this already works.
./markdown-tools-cli -dir testdata -force -all | jq -r 'keys | length'

# Same result, no notice.
./markdown-tools-cli -dir testdata -force -all -q | jq -r 'keys | length'

# Nothing at all on stderr under -q.
./markdown-tools-cli -dir testdata -force -all -q 2>&1 >/dev/null | wc -c
```

Check the three cases that `-q` must not break. A fatal error still fails, with
the file named:

``` bash
./markdown-tools-cli -dir testdata/vault -q; echo "exit=$?"
```

`-vaultcheck` still reports, because its report is the output:

``` bash
./markdown-tools-cli -dir testdata/vault -vaultcheck -q; echo "exit=$?"
```

#### Memory estimate

``` bash
./markdown-tools-cli -dir testdata -all -memusage > /dev/null
```