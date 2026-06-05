// SPDX-FileCopyrightText: 2022 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package static

import "embed"

//go:embed css/portunus.css fonts/* img/*
var FS embed.FS
