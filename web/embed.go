package web

import "embed"

// SAI-129: embute src/ também — index.html carrega /src/main.js
// (sem bundler); só "public" deixava o JS do dashboard 404ando sempre.
//
//go:embed public src
var Public embed.FS
