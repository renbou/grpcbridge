package httprule

import "strings"

type tstate int

const (
	tsegment tstate = iota
	tvariable
	tnested
)

var tnext = map[tstate]string{
	tsegment:  "/{",
	tvariable: ".=}",
	tnested:   "/}",
}

func tokenize(tmpl string) []string {
	state := tsegment

	// approximate number of tokens
	tokens := make([]string, 0, 2*(strings.Count(tmpl, "/"))+1)

	for idx := 0; tmpl != ""; tmpl = tmpl[idx+1:] {
		if idx = strings.IndexAny(tmpl, tnext[state]); idx < 0 {
			tokens = append(tokens, tmpl)
			break
		}

		switch r := tmpl[idx]; r {
		case '/', '.':
		case '{':
			state = tvariable
		case '=':
			state = tnested
		case '}':
			state = tsegment
		}

		if idx > 0 {
			tokens = append(tokens, tmpl[:idx], tmpl[idx:idx+1])
		} else {
			tokens = append(tokens, tmpl[:1])
		}
	}

	// append eof to ease parsing by avoiding length checks
	tokens = append(tokens, eof)

	return tokens
}
