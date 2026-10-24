// Package markdowntools writes to a markdown vault: it adds missing front
// matter, sets the share permission, and repairs duplicate UUIDs. It reads the
// vault through the markdownindexer package, so scanning and indexing stay
// read-only there while every write lives here.
package markdowntools
