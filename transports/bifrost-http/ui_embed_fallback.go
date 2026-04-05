//go:build !embedui

package main

import "embed"

//go:embed all:embedded_ui
var uiContent embed.FS
