// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package grammars

/*

import (
	"fmt"
	"io"
	"strings"

	. "go.xyrillian.de/gg/option"
)

// Parser is the base type for all parser combinators defined in this file.
//
// Because [strings.Reader] can only unread one rune, this type can only accept LL(1) grammars.
type Parser[T any] func(*strings.Reader) ParseResult[T]

// ParseResult is the return type of [Parser].
type ParseResult[T any] struct {
	Value    T     // only filled on success (but may be zero-valued even then depending on T)
	Error    error // only filled on error
	Consumed int   // on error, indicates how many runes were irrevocably consumed
}

type parseError struct {
	Offset   int64
	Expected string       // character class that would have been allowed, e.g. `[a-zA-Z0-9_-]`, or multiple thereof separated with ` or `
	Actual   Option[rune] // None for EOF
}

func unexpectedRune[T any](s *strings.Reader, initialLen int, expected string, actual rune) ParseResult[T] {
	_ = s.UnreadRune()
	currentLen := s.Len()
	return ParseResult[T]{
		Error:    parseError{s.Size() - int64(currentLen), expected, actual},
		Consumed: initialLen - currentLen,
	}
}

// Error implements the [error] interface.
func (e parseError) Error() string {
	return fmt.Sprintf("at offset %d: expected [%s], but got %q", e.Offset, e.Expected, e.Actual)
}

////////////////////////////////////////////////////////////////////////////////
// parser combinators

// Rune is a parser that accepts exactly the provided rune and nothing else.
func Rune(expected rune) Parser[struct{}] {
	repr := fmt.Sprintf("[%c]", expected)
	return func(s *strings.Reader) ParseResult[struct{}] {
		initialLen := s.Len()
		r, _, err := s.ReadRune()
		switch {
		case err != nil:
			return ParseResult[struct{}]{Error: err}
		case r == expected:
			return ParseResult[struct{}]{}
		default:
			return unexpectedRune[struct{}](s, initialLen, repr, r)
		}
	}
}

// NonEmptyListWithSeparator is a parser that accepts non-empty lists of elements,
// with each element separated by the specified rune (without any extra whitespace).
func NonEmptyListWithSeparator[T ~[]E, E any](parseElement Parser[E], separator rune) Parser[T] {
	repr := fmt.Sprintf("[%c]", separator)
	return func(s *strings.Reader) ParseResult[T] {
		var result T
		for {
			elem := parseElement(s)
			if elem.Error != nil {
				return nil, err
			}
			result = append(result, elem)

			r, _, err := s.ReadRune()
			switch {
			case err == io.EOF:
				return result, nil
			case err != nil:
				_ = s.UnreadRune()
				return nil, err // defense in depth: strings.Reader.ReadRune() should never return non-EOF errors
			case r == separator:
				continue
			default:
				return nil, newParseError(s, repr, Some(r))
			}
		}
	}
}

// SequenceOfTwo is a parser that invokes two parsers in order, and then fuses both results into one.
func SequenceOfTwo[T, A, B any](parseA Parser[A], parseB Parser[B], fuse func(A, B) T) Parser[T] {
	return func(s *strings.Reader) (_ T, err error) {
		var (
			a A
			b B
		)
		a, err = parseA(s)
		if err != nil {
			return
		}
		b, err = parseB(s)
		if err != nil {
			return
		}
		return fuse(a, b), nil
	}
}

// SequenceOfThree is a parser that invokes three parsers in order, and then fuses all their results into one.
func SequenceOfThree[T, A, B, C any](parseA Parser[A], parseB Parser[B], parseC Parser[C], fuse func(A, B, C) T) Parser[T] {
	return func(s *strings.Reader) (_ T, err error) {
		var (
			a A
			b B
			c C
		)
		a, err = parseA(s)
		if err != nil {
			return
		}
		b, err = parseB(s)
		if err != nil {
			return
		}
		c, err = parseC(s)
		if err != nil {
			return
		}
		return fuse(a, b, c), nil
	}
}

*/
