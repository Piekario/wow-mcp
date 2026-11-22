// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package savedvars

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// Parser of WoW SavedVariables files.
//
// The game client saves addon variables as plain Lua code made of top-level
// assignments:
//
//	WowMcpDB = {
//	["last"] = "Piekasz",
//	["characters"] = {
//	{
//	["name"] = "Linen Cloth",
//	}, -- [1]
//	},
//	}
//
// We do not need the whole language, only literals: tables, strings, numbers,
// true/false/nil and comments. The result uses types known from encoding/json:
// map[string]any, []any, string, int64, float64, bool and nil.

// Parse returns a map: global variable name -> its value.
func Parse(src []byte) (map[string]any, error) {
	p := &luaParser{src: src}
	vars := map[string]any{}
	for {
		p.skipSpace()
		if p.eof() {
			return vars, nil
		}
		name := p.identifier()
		if name == "" {
			return nil, p.errorf("expected a variable name")
		}
		p.skipSpace()
		if !p.consume('=') {
			return nil, p.errorf("expected '=' after name %q", name)
		}
		value, err := p.value()
		if err != nil {
			return nil, err
		}
		vars[name] = value
		p.skipSpace()
		p.consume(';')
	}
}

// maxDepth limits table nesting; real dumps have a dozen or so levels,
// and without a limit "{{{{..." overflows the stack (a fatal error that cannot be recovered).
const maxDepth = 200

type luaParser struct {
	src   []byte
	pos   int
	depth int // current table nesting
}

func (p *luaParser) eof() bool { return p.pos >= len(p.src) }

func (p *luaParser) peek() byte {
	if p.eof() {
		return 0
	}
	return p.src[p.pos]
}

func (p *luaParser) consume(c byte) bool {
	if p.peek() == c && !p.eof() {
		p.pos++
		return true
	}
	return false
}

func (p *luaParser) errorf(format string, args ...any) error {
	line := 1 + bytes.Count(p.src[:p.pos], []byte("\n"))
	return fmt.Errorf("SavedVariables, line %d: %s", line, fmt.Sprintf(format, args...))
}

// skipSpace skips whitespace and comments: "-- to end of line" and "--[[ block ]]".
func (p *luaParser) skipSpace() {
	for !p.eof() {
		c := p.src[p.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			p.pos++
		case c == 0xEF && p.pos+2 < len(p.src) && p.src[p.pos+1] == 0xBB && p.src[p.pos+2] == 0xBF:
			p.pos += 3 // UTF-8 BOM
		case c == '-' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '-':
			p.pos += 2
			if bytes.HasPrefix(p.src[p.pos:], []byte("[[")) {
				end := bytes.Index(p.src[p.pos:], []byte("]]"))
				if end < 0 {
					p.pos = len(p.src)
				} else {
					p.pos += end + 2
				}
				continue
			}
			for !p.eof() && p.src[p.pos] != '\n' {
				p.pos++
			}
		default:
			return
		}
	}
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func (p *luaParser) identifier() string {
	start := p.pos
	if p.eof() || !isIdentStart(p.src[p.pos]) {
		return ""
	}
	for !p.eof() && (isIdentStart(p.src[p.pos]) || isDigit(p.src[p.pos])) {
		p.pos++
	}
	return string(p.src[start:p.pos])
}

func (p *luaParser) value() (any, error) {
	p.skipSpace()
	if p.eof() {
		return nil, p.errorf("unexpected end of file")
	}
	c := p.peek()
	switch {
	case c == '{':
		return p.table()
	case c == '"' || c == '\'':
		return p.str()
	case isDigit(c) || c == '-' || c == '.':
		return p.number()
	case isIdentStart(c):
		word := p.identifier()
		switch word {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "nil", "inf", "nan":
			return nil, nil
		}
		return nil, p.errorf("unknown word %q", word)
	}
	return nil, p.errorf("unexpected character %q", string(c))
}

func (p *luaParser) number() (any, error) {
	start := p.pos
	p.consume('-')
	p.skipSpace()
	if isIdentStart(p.peek()) { // "-inf", "-nan"
		p.identifier()
		return nil, nil
	}
	if p.peek() == '0' && p.pos+1 < len(p.src) && (p.src[p.pos+1] == 'x' || p.src[p.pos+1] == 'X') {
		p.pos += 2
		for !p.eof() && strings.IndexByte("0123456789abcdefABCDEF", p.src[p.pos]) >= 0 {
			p.pos++
		}
		text := strings.ReplaceAll(string(p.src[start:p.pos]), " ", "")
		n, err := strconv.ParseInt(text, 0, 64)
		if err != nil {
			return nil, p.errorf("invalid number %q", text)
		}
		return n, nil
	}
	for !p.eof() {
		c := p.src[p.pos]
		if isDigit(c) || c == '.' {
			p.pos++
		} else if c == 'e' || c == 'E' {
			p.pos++
			if p.peek() == '+' || p.peek() == '-' {
				p.pos++
			}
		} else {
			break
		}
	}
	text := strings.ReplaceAll(string(p.src[start:p.pos]), " ", "")
	if n, err := strconv.ParseInt(text, 10, 64); err == nil {
		return n, nil
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil, p.errorf("invalid number %q", text)
	}
	return f, nil
}

func (p *luaParser) str() (string, error) {
	quote := p.src[p.pos]
	p.pos++
	var b strings.Builder
	for {
		if p.eof() {
			return "", p.errorf("unterminated string")
		}
		c := p.src[p.pos]
		p.pos++
		if c == quote {
			return b.String(), nil
		}
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		if p.eof() {
			return "", p.errorf("unterminated string")
		}
		e := p.src[p.pos]
		p.pos++
		switch e {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case '\n':
			b.WriteByte('\n')
		default:
			if isDigit(e) { // \ddd: byte code in decimal, up to three digits
				n := int(e - '0')
				for i := 0; i < 2 && !p.eof() && isDigit(p.src[p.pos]); i++ {
					n = n*10 + int(p.src[p.pos]-'0')
					p.pos++
				}
				if n > 255 { // Lua rejects it too ("decimal escape too large"); byte(n) would silently wrap
					return "", p.errorf("decimal escape \\%d is larger than 255", n)
				}
				b.WriteByte(byte(n))
			} else {
				b.WriteByte(e) // \\ \" \' and the rest: the character literally
			}
		}
	}
}

type tableEntry struct {
	key   any // string, int64, float64 or nil for a positional element
	value any
}

func (p *luaParser) table() (any, error) {
	if p.depth >= maxDepth {
		return nil, p.errorf("tables nested deeper than %d levels", maxDepth)
	}
	p.depth++
	defer func() { p.depth-- }()
	p.pos++ // '{'
	var entries []tableEntry
	for {
		p.skipSpace()
		if p.eof() {
			return nil, p.errorf("unterminated table")
		}
		if p.consume('}') {
			return buildTable(entries), nil
		}

		var entry tableEntry
		switch {
		case p.peek() == '[':
			p.pos++
			key, err := p.value()
			if err != nil {
				return nil, err
			}
			p.skipSpace()
			if !p.consume(']') {
				return nil, p.errorf("expected ']' after key")
			}
			p.skipSpace()
			if !p.consume('=') {
				return nil, p.errorf("expected '=' after key")
			}
			entry.key = key
		case isIdentStart(p.peek()):
			// "name = value" or just a value (true/false/nil).
			save := p.pos
			word := p.identifier()
			p.skipSpace()
			if p.peek() == '=' {
				p.pos++
				entry.key = word
			} else {
				p.pos = save
			}
		}

		value, err := p.value()
		if err != nil {
			return nil, err
		}
		entry.value = value
		entries = append(entries, entry)

		p.skipSpace()
		if !p.consume(',') {
			p.consume(';')
		}
	}
}

// buildTable converts a Lua table to []any (list 1..n) or map[string]any.
func buildTable(entries []tableEntry) any {
	if len(entries) == 0 {
		return []any{}
	}

	indexed := map[int64]any{}
	named := map[string]any{}
	next := int64(1)
	for _, e := range entries {
		switch k := e.key.(type) {
		case nil:
			indexed[next] = e.value
			next++
		case int64:
			indexed[k] = e.value
		case float64:
			named[strconv.FormatFloat(k, 'g', -1, 64)] = e.value
		case string:
			named[k] = e.value
		case bool:
			named[strconv.FormatBool(k)] = e.value
		}
	}

	if len(named) == 0 {
		list := make([]any, 0, len(indexed))
		for i := int64(1); i <= int64(len(indexed)); i++ {
			v, ok := indexed[i]
			if !ok {
				list = nil
				break
			}
			list = append(list, v)
		}
		if list != nil {
			return list
		}
	}

	for k, v := range indexed {
		named[strconv.FormatInt(k, 10)] = v
	}
	return named
}
