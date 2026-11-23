// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package export

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// format is the summary output strategy: Discord (Markdown), forum (BBCode) or text.
type format interface {
	name() string
	title(b *strings.Builder, text string)
	field(b *strings.Builder, label, value string)
	list(b *strings.Builder, label string, items []string)
	itemName(item Item) string
	finish(text string) string
}

// formatNamed picks a format by name (with aliases); an empty name means discord.
func formatNamed(name string) (format, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "discord", "markdown", "md":
		return discord{}, true
	case "forum", "bbcode":
		return forum{}, true
	case "text", "plain", "txt":
		return plainText{}, true
	}
	return nil, false
}

// discord is Markdown for Discord.
type discord struct{}

func (discord) name() string { return "discord" }

func (discord) title(b *strings.Builder, text string) { fmt.Fprintf(b, "### %s\n", text) }

func (discord) field(b *strings.Builder, label, value string) {
	fmt.Fprintf(b, "**%s:** %s\n", label, value)
}

func (discord) list(b *strings.Builder, label string, items []string) {
	fmt.Fprintf(b, "\n**%s**\n", label)
	for _, it := range items {
		fmt.Fprintf(b, "- %s\n", it)
	}
}

func (discord) itemName(item Item) string { return fmt.Sprint(item.Name) }

func (discord) finish(text string) string { return text }

// forum is BBCode with item quality colors.
type forum struct{}

var qualityColors = map[int64]string{0: "#9d9d9d", 1: "#ffffff", 2: "#1eff00", 3: "#0070dd", 4: "#a335ee", 5: "#ff8000"}

func (forum) name() string { return "forum" }

func (forum) title(b *strings.Builder, text string) {
	fmt.Fprintf(b, "[size=5][b]%s[/b][/size]\n", text)
}

func (forum) field(b *strings.Builder, label, value string) {
	fmt.Fprintf(b, "[b]%s:[/b] %s\n", label, value)
}

func (forum) list(b *strings.Builder, label string, items []string) {
	fmt.Fprintf(b, "\n[b]%s[/b]\n[list]\n", label)
	for _, it := range items {
		fmt.Fprintf(b, "[*]%s\n", it)
	}
	b.WriteString("[/list]\n")
}

func (forum) itemName(item Item) string {
	name := fmt.Sprint(item.Name)
	if item.HasQuality && qualityColors[item.Quality] != "" {
		return fmt.Sprintf("[color=%s]%s[/color]", qualityColors[item.Quality], name)
	}
	return name
}

func (forum) finish(text string) string { return text }

// plainText is plain text with an underlined title.
type plainText struct{}

func (plainText) name() string { return "text" }

func (plainText) title(b *strings.Builder, text string) {
	fmt.Fprintf(b, "%s\n%s\n", text, strings.Repeat("=", utf8.RuneCountInString(text)))
}

func (plainText) field(b *strings.Builder, label, value string) {
	fmt.Fprintf(b, "%s: %s\n", label, value)
}

func (plainText) list(b *strings.Builder, label string, items []string) {
	fmt.Fprintf(b, "\n%s:\n", label)
	for _, it := range items {
		fmt.Fprintf(b, "  - %s\n", it)
	}
}

func (plainText) itemName(item Item) string { return fmt.Sprint(item.Name) }

func (plainText) finish(text string) string { return text }
