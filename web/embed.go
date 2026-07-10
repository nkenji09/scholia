// Package webdist embeds the built viewer SPA (web/dist, produced by
// `npm run build`) so internal/viewer can serve it without a node/npm
// runtime dependency (DESIGN §9/§10).
package webdist

import "embed"

//go:embed all:dist
var FS embed.FS
