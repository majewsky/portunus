// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package grammars

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	combi "codeberg.org/cellsons/combi/pkg"

	"github.com/majewsky/portunus/internal/ldapproto"
)

// DN is a structured representation for a Distinguished Name [RFC 4514, section 2.1].
// Its constituent RDNs appear in the slice in serialization order.
type DN []RelativeDN

// RelativeDN is a structured representation for a Relative Distinguished Name [RFC 4514, section 2.2].
// Its constituent attributes appear in the slice in serialization order.
type RelativeDN []Attribute

// Attribute is a single pair of attribute type and value that appears in [RelativeDN].
type Attribute struct {
	Type ldapproto.String
	// EncodedValue contains the string representation of the [ldapproto.AttributeValue],
	// according to whatever string encoding rule is specified for this attribute type,
	// cf. e.g. [RFC 4517, section 3.3] for descriptions of common attribute types.
	EncodedValue string
}

// ParseDN parses the string representation of a [DN].
func ParseDN(input ldapproto.DN) (DN, error) {
	if input == "" {
		// empty DNs are explicitly allowed (used in SearchRequest to query for the root DSE)
		return nil, nil
	}
	if !utf8.ValidString(string(input)) {
		return nil, errors.New("not a valid UTF-8 string")
	}
	if strings.ContainsFunc(string(input), isNotSimpleDNRune) {
		return slowParseDN(string(input))
	} else {
		return fastParseDN(string(input))
	}
}

////////////////////////////////////////////////////////////////////////////////
// fast (as in: allocation-minimizing) DN parser for "boring" DNs like `uid=JohnDoe42,ou=users,dc=example,dc=org`

func isNotSimpleDNRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return false
	case r >= 'A' && r <= 'Z':
		return false
	case r >= '0' && r <= '9':
		return false
	case r == ',' || r == '=':
		return false
	default:
		return true
	}
}

func fastParseDN(input string) (result DN, _ error) {
	for raw := range strings.SplitSeq(input, ",") {
		rdn, err := fastParseRDN(raw)
		if err != nil {
			return nil, fmt.Errorf("malformed relativeDistinguishedName %q: %w", raw, err)
		}
		result = append(result, rdn)
	}
	return
}

func fastParseRDN(input string) (result RelativeDN, _ error) {
	if input == "" {
		return nil, errors.New("RDN may not be empty")
	}
	for raw := range strings.SplitSeq(input, "+") {
		rdn, err := fastParseRDNAttribute(raw)
		if err != nil {
			return nil, fmt.Errorf("malformed attributeTypeAndValue %q: %w", raw, err)
		}
		result = append(result, rdn)
	}
	return
}

func fastParseRDNAttribute(input string) (Attribute, error) {
	left, right, ok := strings.Cut(input, "=")
	if !ok {
		return Attribute{}, errors.New(`not of the form "type=value"`)
	}
	return Attribute{Type: left, EncodedValue: right}, nil
}

////////////////////////////////////////////////////////////////////////////////
// slow (as in: careful, but with lots of allocations) parser for the full DN grammar

func slowParseDN(input string) (result DN, _ error) {
	return combi.Parse(dnParser, input)
}

// productions from RFC 4514
var (
	// distinguishedName = [ relativeDistinguishedName *( COMMA relativeDistinguishedName ) ]
	// NOTE: empty DNs are already handled in ParseDN()
	dnParser = combi.Map(
		combi.Cons(rdnParser, combi.Many(combi.DiscardLeft(combi.Token(","), rdnParser))),
		func(rdns []RelativeDN) DN { return DN(rdns) },
	)

	// relativeDistinguishedName = attributeTypeAndValue *( PLUS attributeTypeAndValue )
	rdnParser = combi.Map(
		combi.Cons(attributeTypeAndValueParser, combi.Many(combi.DiscardLeft(combi.Token("+"), attributeTypeAndValueParser))),
		func(attrs []Attribute) RelativeDN { return RelativeDN(attrs) },
	)

	// attributeTypeAndValue = attributeType EQUALS attributeValue
	attributeTypeAndValueParser = combi.Map2(
		attributeTypeParser, combi.DiscardLeft(combi.Token("="), attributeValueParser),
		func(t, v string) Attribute { return Attribute{t, v} },
	)

	// attributeType = descr / numericoid
	// descr = keystring
	attributeTypeParser = combi.Choice(keystringParser, numericoidParser)

	// attributeValue = string / hexstring
	attributeValueParser = combi.Choice(stringParser, hexstringParser)

	// string = [ ( leadchar / pair ) [ *( stringchar / pair ) ( trailchar / pair ) ] ]
	stringParser = combi.Map(
		combi.OneOrZero(combi.Compose(
			combi.Choice(leadcharParser, pairParser),
			combi.OneOrZero(combi.Compose(
				combi.Many(combi.Choice(stringcharParser, pairParser)),
				combi.Choice(trailcharParser, pairParser),
			)),
		)),
		func(p combi.Pair[string, combi.Pair[[]string, string]]) string {
			return p.Left + strings.Join(p.Right.Left, "") + p.Right.Right
		},
	)

	// leadchar   = LUTF1 / UTFMB
	// LUTF1      = %x01-1F / %x21 / %x24-2A / %x2D-3A / %x3D / %x3F-5B / %x5D-7F
	// trailchar  = TUTF1 / UTFMB
	// TUTF1      = %x01-1F / %x21 / %x23-2A / %x2D-3A / %x3D / %x3F-5B / %x5D-7F
	// stringchar = SUTF1 / UTFMB
	// SUTF1      = %x01-21 / %x23-2A / %x2D-3A / %x3D / %x3F-5B / %x5D-7F
	leadcharParser   = combi.Map(combi.NoneOfChar(` "#+,;<>\`), stringFromRune)
	trailcharParser  = combi.Map(combi.NoneOfChar(` "+,;<>\`), stringFromRune)
	stringcharParser = combi.Map(combi.NoneOfChar(`"+,;<>\`), stringFromRune)
	stringFromRune   = func(r rune) string { return string([]rune{r}) }

	// pair    = ESC ( ESC / special / hexpair )
	// special = escaped / SPACE / SHARP / EQUALS
	// escaped = DQUOTE / PLUS / COMMA / SEMI / LANGLE / RANGLE
	pairParser = combi.DiscardLeft(combi.Token(`\`), combi.Map(
		combi.Choice(combi.OneOfChar(` "#+,;<=>\`), hexpairParser),
		stringFromRune,
	))

	// TODO
	// hexstring = SHARP 1*hexpair
	// hexpair = HEX HEX
	hexstringParser combi.ParseFn[string]
	hexpairParser   combi.ParseFn[rune]
)

// productions from RFC 4512
var (
	// keystring   = leadkeychar *keychar
	// leadkeychar = ALPHA
	// keychar     = ALPHA / DIGIT / HYPHEN
	keystringParser = combi.StringFromRunes(combi.Cons(
		combi.OneOfChar("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMOPQRSTUVWXYZ"),
		combi.Many(combi.OneOfChar("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMOPQRSTUVWXYZ0123456789-")),
	))

	// numericoid = number 1*( DOT number )
	numericoidParser = combi.Map(
		combi.Cons(numberParser, combi.Many1(combi.DiscardLeft(combi.Token("."), numberParser))),
		func(numbers []string) string { return strings.Join(numbers, ".") },
	)

	// number  = DIGIT / ( LDIGIT 1*DIGIT )
	// DIGIT   = %x30 / LDIGIT       ; "0"-"9"
	// LDIGIT  = %x31-39             ; "1"-"9"
	//
	// rephrased here as: number = %30 / ( LDIGIT *DIGIT )
	numberParser = combi.Choice(
		combi.Token("0"),
		combi.StringFromRunes(combi.Cons(
			combi.OneOfChar("123456789"),
			combi.Many(combi.OneOfChar("0123456789")),
		)),
	)
)

/*

func slowParseDN(input string) (result DN, _ error) {
	return dnParser(strings.NewReader(input))
}

var (
	attributeTypeParser  = func(*strings.Reader) (string, error) { return "", errors.New("beep") }
	attributeValueParser = func(*strings.Reader) (string, error) { return "", errors.New("boop") }
	rdnAttributeParser   = SequenceOfThree(attributeTypeParser, Rune('='), attributeValueParser, fuseRDNAttribute)
	rdnParser            = NonEmptyListWithSeparator[RelativeDN](rdnAttributeParser, '+')
	dnParser             = NonEmptyListWithSeparator[DN](rdnParser, ',')
)

func fuseRDNAttribute(attributeType string, _ struct{}, encodedValue string) Attribute {
	return Attribute{attributeType, encodedValue}
}

*/
