// Package viewer serves the embedded SPA and its JSON API over local HTTP
// (pmem view / export --html). It is a thin layer over internal/{store,
// index,lint,diff,render}: no derived-view logic is duplicated here (§7).
package viewer
