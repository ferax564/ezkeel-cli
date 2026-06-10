package main

import (
	"fmt"
	"strings"
	"unicode"
)

// splitShellWords splits s into words following POSIX-like shell quoting
// rules, without performing any expansion:
//
//   - unquoted whitespace separates words
//   - single quotes preserve everything literally until the next single quote
//   - double quotes preserve everything except `\"` and `\\`, which
//     escape to a literal quote/backslash
//   - an unquoted backslash escapes the next character
//
// It exists so user-supplied commands like
//
//	bundle exec rails runner "User.where(name: 'x').count"
//
// keep their quoted arguments intact instead of being torn apart on
// every space (the old strings.Fields behaviour). Unterminated quotes
// or a trailing backslash are reported as errors.
func splitShellWords(s string) ([]string, error) {
	var (
		words   []string
		cur     strings.Builder
		inWord  bool
		quote   rune // 0 = unquoted, '\'' or '"'
		escaped bool
	)

	for _, r := range s {
		switch {
		case escaped:
			// Inside double quotes, backslash only escapes the
			// characters that are special there; before anything else
			// it stays literal (`"\n"` is backslash-n, not n).
			if quote == '"' && r != '"' && r != '\\' && r != '$' && r != '`' {
				cur.WriteRune('\\')
			}
			cur.WriteRune(r)
			escaped = false
		case quote == '\'':
			if r == '\'' {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case quote == '"':
			switch r {
			case '"':
				quote = 0
			case '\\':
				escaped = true
			default:
				cur.WriteRune(r)
			}
		case r == '\\':
			escaped = true
			inWord = true
		case r == '\'' || r == '"':
			quote = r
			inWord = true
		case unicode.IsSpace(r):
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}

	if escaped {
		return nil, fmt.Errorf("trailing backslash")
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated %c quote", quote)
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words, nil
}
