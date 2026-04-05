//go:build embedui

package main

import "embed"

//go:embed all:ui
var uiContent embed.FS
