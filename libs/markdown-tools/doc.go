// Package markdowntools writes derived output: it adds missing front matter,
// sets the share permission, and repairs duplicate UUIDs in a markdown vault,
// and publishes each document's backlinks as a JSON file. It reads the vault
// through the markdownindexer package, so scanning and indexing stay read-only
// there while every write lives here.
package markdowntools
