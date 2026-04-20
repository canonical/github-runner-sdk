package main

import (
	"io"
	"os"
	"slices"
	"strings"

	"golang.org/x/term"
)

type symbols struct {
	lock string
	tick string
}

type escapes struct {
	green     string
	end       string
	link      string
	linkEnd   string
	keypadEnd string
}

func newSymbols(out io.Writer) symbols {
	if supportsUnicode(out) {
		// Deliberately use a root symbol instead of a tick, to match actions/runner.
		return symbols{lock: "🔐", tick: "√"}
	} else {
		return symbols{lock: "--", tick: "-"}
	}
}

func supportsUnicode(out io.Writer) bool {
	if !isTerminal(out) {
		return false
	}

	for _, k := range []string{"LC_MESSAGES", "LC_ALL", "LANG"} {
		lang := strings.ToUpper(os.Getenv(k))
		if strings.Contains(lang, "UTF-8") || strings.Contains(lang, "UTF8") {
			return true
		}
		if lang != "" {
			break
		}
	}
	return false
}

func newEscapes(out io.Writer) escapes {
	if !supportsColor(out) {
		return escapes{}
	}
	return escapes{
		green:     "\033[92m",
		end:       "\033[0m",
		link:      "\033]8;;",
		linkEnd:   "\033\\",
		keypadEnd: "\033[?1l\033>",
	}
}

func (e *escapes) linkURL(url string) string {
	return e.linkText(url, url)
}

func (e *escapes) linkText(text, url string) string {
	if e.link == "" {
		return url
	}
	return e.link + url + e.linkEnd + text + e.link + e.linkEnd
}

func supportsColor(out io.Writer) bool {
	if !isTerminal(out) {
		return false
	}

	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}

	if term, ok := os.LookupEnv("TERM"); !ok || slices.Contains([]string{"dumb", "linux-m", "xterm-mono"}, term) {
		return false
	}

	return true
}

func isTerminal(out io.Writer) bool {
	type hasFd interface {
		Fd() uintptr
	}

	file, ok := out.(hasFd)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}
