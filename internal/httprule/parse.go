package httprule

import (
	"fmt"
	"strings"
)

const eof = "\u0000"

type lexeme string

const (
	lexemeIdent   lexeme = "i"
	lexemeLiteral lexeme = "l"
	lexemeVerb    lexeme = "v"
	lexemeEof     lexeme = "e"
)

type segment struct {
	typ      segmentType
	literal  string
	variable variable
}

type segmentType int

const (
	segmentWildcard segmentType = iota
	segmentMultiWildcard
	segmentVariable
	segmentLiteral
)

type variable struct {
	fieldPath []string
	segments  []segment
}

// Template represents a parsed HttpRule path template.
type Template struct {
	tmpl     string
	segments []segment
	verb     string
}

// Parse parses an HttpRule path template according to the rules specified in google/api/http.proto
// and formalized as ABNF in the file httprule.bnf in this directory.
func Parse(tmpl string) (*Template, error) {
	if !strings.HasPrefix(tmpl, "/") {
		return nil, fmt.Errorf("template %q does not contain leading /", tmpl)
	}

	tokens := tokenize(tmpl[1:])
	p := &parser{
		accepted: tokens[:0],
		left:     tokens,
	}

	res, err := p.template()
	if err != nil {
		return nil, err
	}

	res.tmpl = tmpl

	return res, nil
}

type parser struct {
	accepted []string
	left     []string
}

func (p *parser) template() (*Template, error) {
	if _, err := p.accept(lexemeEof); err == nil {
		return &Template{segments: []segment{{typ: segmentLiteral, literal: ""}}}, nil
	}

	segments, _, err := p.segments()
	if err != nil {
		return nil, p.error()
	}

	tmpl := &Template{segments: segments}
	last := &segments[len(segments)-1]

	if last.typ == segmentLiteral {
		// split literal into literal + verb
		if verbIdx := strings.LastIndex(last.literal, ":"); verbIdx != -1 {
			tmpl.verb = last.literal[verbIdx+1:]
			last.literal = last.literal[:verbIdx]
		}
	} else if last.typ == segmentVariable && p.left[0] != eof {
		// additionally allow a verb
		if tmpl.verb, err = p.accept(lexemeVerb); err != nil {
			return nil, p.error()
		}
	}

	if _, err := p.accept(lexemeEof); err != nil {
		return nil, p.error()
	}

	return tmpl, nil
}

func (p *parser) error() error {
	return fmt.Errorf("unexpected token %q after segments %q", p.left[0], "/"+strings.Join(p.accepted, ""))
}

func (p *parser) segments() ([]segment, bool, error) {
	segments := make([]segment, 0, 1)
	multi := false

	for {
		s, ms, err := p.segment()
		if err != nil {
			return nil, false, err
		}

		segments = append(segments, s)

		if ms {
			// MultiSegment, MultiSegVariable
			multi = true
			break
		}

		if _, err := p.accept("/"); err != nil {
			break
		}
	}

	return segments, multi, nil
}

func (p *parser) segment() (segment, bool, error) {
	// wildcards
	if _, err := p.accept("*"); err == nil {
		return segment{typ: segmentWildcard}, false, nil
	} else if _, err := p.accept("**"); err == nil {
		return segment{typ: segmentMultiWildcard}, true, nil
	}

	// literal
	if l, err := p.accept(lexemeLiteral); err == nil {
		return segment{typ: segmentLiteral, literal: l}, false, nil
	}

	// variable
	if v, multi, err := p.variable(); err == nil {
		return segment{typ: segmentVariable, variable: v}, multi, nil
	}

	return segment{}, false, fmt.Errorf("invalid segment: not a wildcard, literal, or variable")
}

func (p *parser) variable() (variable, bool, error) {
	if _, err := p.accept("{"); err != nil {
		return variable{}, false, err
	}

	path, err := p.fieldPath()
	if err != nil {
		return variable{}, false, fmt.Errorf("invalid field path in variable: %w", err)
	}

	var segments []segment
	multi := false

	if _, err := p.accept("="); err == nil {
		// note that variable inside variable is not possible thanks to tokenize
		if segments, multi, err = p.segments(); err != nil {
			return variable{}, false, fmt.Errorf("invalid segment in variable: %w", err)
		}
	} else {
		segments = []segment{{typ: segmentWildcard}}
	}

	if _, err := p.accept("}"); err != nil {
		return variable{}, false, fmt.Errorf("unterminated variable: %w", err)
	}

	return variable{
		fieldPath: path,
		segments:  segments,
	}, multi, nil
}

func (p *parser) fieldPath() ([]string, error) {
	component, err := p.accept(lexemeIdent)
	if err != nil {
		return nil, err
	}

	path := []string{component}

	for {
		if _, err := p.accept("."); err != nil {
			break
		}

		component, err := p.accept(lexemeIdent)
		if err != nil {
			return nil, err
		}

		path = append(path, component)
	}

	return path, nil
}

func (p *parser) accept(want lexeme) (string, error) {
	got := p.left[0]

	switch want {
	case lexemeEof:
		if got != eof {
			return "", fmt.Errorf("expected EOF, got %q", got)
		}
	case lexemeIdent:
		if err := checkIdent(got); err != nil {
			return "", err
		}
	case lexemeLiteral:
		if err := checkLiteral(got); err != nil {
			return "", err
		}
	case lexemeVerb:
		if err := checkLiteral(got); err != nil {
			return "", err
		}

		if !strings.HasPrefix(got, ":") {
			return "", fmt.Errorf("verb %q does not start with colon", got)
		}

		got = got[1:]
	case "/", "*", "**", ".", "=", "{", "}", ":":
		if got != string(want) {
			return "", fmt.Errorf("expected %q but got %q", want, got)
		}
	}

	p.left = p.left[1:]
	p.accepted = p.accepted[:len(p.accepted)+1]

	return got, nil
}

// IDENT = (ALPHA / "_") *(ALPHA / DIGIT / "_")
func checkIdent(s string) error {
	// empty identifier cannot occur here thanks to tokenize

	for i, c := range s {
		switch {
		case '0' <= c && c <= '9':
			if i == 0 {
				return fmt.Errorf("identifier %q starts with digit", s)
			}
			continue
		case 'A' <= c && c <= 'Z':
			continue
		case 'a' <= c && c <= 'z':
			continue
		case c == '_':
			continue
		default:
			return fmt.Errorf("invalid character %q in identifier %q", c, s)
		}
	}

	return nil
}

// LITERAL = *UriPchar
func checkLiteral(s string) error {
	original := s

	for s != "" {
		var err error
		s, err = consumePchar(original, s)
		if err != nil {
			return err
		}
	}

	return nil
}

// UriPchar = UriUnreserved / UriPctEncoded / UriSubDelims / ":" / "@" ; RFC 3986, URI
// UriUnreserved = ALPHA / DIGIT / "-" / "." / "_" / "~" ; RFC 3986, URI
// UriSubDelims = "!" / "$" / "&" / "’" / "(" / ")" / "*" / "+" / "," / ";" / "=" ; RFC 3986, URI
// UriPctEncoded = "%" HEXDIG HEXDIG ; RFC 3986, URI
func consumePchar(whole, left string) (string, error) {
	c, left := left[0], left[1:]

	// Unreserved
	switch {
	case '0' <= c && c <= '9':
		fallthrough
	case 'A' <= c && c <= 'Z':
		fallthrough
	case 'a' <= c && c <= 'z':
		return left, nil
	}

	switch c {
	case '-', '.', '_', '~':
		// Unreserved
		fallthrough
	case '!', '$', '&', '\'', '(', ')', '*', '+', ',', ';', '=':
		// Subdelims
		fallthrough
	case ':', '@':
		// Other pchar
		return left, nil
	}

	if c != '%' {
		return "", fmt.Errorf("invalid character %q in path segment", c)
	} else if len(left) < 2 || !isHex(left[0]) || !isHex(left[1]) {
		return "", fmt.Errorf("invalid percent-encoding in %q", whole)
	}

	return left[2:], nil
}

func isHex(c byte) bool {
	switch {
	case '0' <= c && c <= '9':
		return true
	case 'A' <= c && c <= 'F':
		return true
	case 'a' <= c && c <= 'f':
		return true
	}
	return false
}
