package combi

import (
	"fmt"
	"hash/maphash"
	"strings"
	"unicode"
	utf8 "unicode/utf8"
)

type Error struct {
	message string
}

func (e *Error) Error() string {
	if e != nil {
		return e.message
	} else {
		return ""
	}
}

type State struct {
	content     string
	contentHash uint64
	pos         int
}

func NewState(content string) State {
	// Calculate hash of content to be used for memoization purposes
	seed := maphash.MakeSeed()
	contentHash := maphash.String(seed, content)

	return State{
		content:     content,
		contentHash: contentHash,
		pos:         0,
	}
}

type StateCacheKey struct {
	contentHash uint64
	pos         int
}

func (s State) CacheKey() StateCacheKey {
	return StateCacheKey{
		contentHash: s.contentHash,
		pos:         s.pos,
	}
}

func (s State) Rest() string {
	return s.content[s.pos:]
}

func (s State) Eof() bool {
	return len(s.Rest()) == 0
}

func (s State) NextRune() (rune, int) {
	return utf8.DecodeRuneInString(s.Rest())
}

func (s State) ConsumeSize(size int) State {
	return State{
		content:     s.content,
		contentHash: s.contentHash,
		pos:         s.pos + size,
	}
}

type Result[T any] struct {
	value T
	state State
	error *Error
}

func Parsed[T any](result Result[T]) (T, *Error) {
	if result.error != nil {
		return result.value, result.error
	}
	if result.state.Rest() != "" {
		error := NewError(
			fmt.Sprintf(
				"input not fully consumed at pos %d: %#v",
				result.state.pos,
				result.state.Rest(),
			),
		)
		return result.value, error
	}
	return result.value, nil
}

func Parse[T any](parser ParseFn[T], content string) (T, *Error) {
	state := NewState(content)
	result := parser(state)
	return Parsed(result)
}

func Failed[T any](error *Error, state State) Result[T] {
	return Result[T]{
		state: state,
		error: error,
	}
}

func Ok[T any](value T, state State) Result[T] {
	return Result[T]{
		value: value,
		state: state,
	}
}

type ParseFn[T any] = func(State) Result[T]

func NewError(msg string) *Error {
	return &Error{msg}
}

func Token(word string) ParseFn[string] {
	unmatchedError := NewError(fmt.Sprintf("content does not start with word %#v", word))
	return func(state State) Result[string] {
		if strings.HasPrefix(state.Rest(), word) {
			return Ok(word, state.ConsumeSize(len(word)))
		}
		return Failed[string](unmatchedError, state)
	}
}

func Whitespace() ParseFn[rune] {
	return MapErrorValue(
		SatisfyChar(
			func(rune rune) bool {
				return unicode.IsSpace(rune)
			},
		),
		NewError("not whitespace"),
	)
}

func SatisfyChar(fn func(rune rune) bool) ParseFn[rune] {
	error := NewError("char predicate not satisfied")
	return func(state State) Result[rune] {
		if rune, size := state.NextRune(); size > 0 && fn(rune) {
			return Ok(rune, state.ConsumeSize(size))
		}
		return Failed[rune](error, state)
	}
}

func OneOfChar(chars string) ParseFn[rune] {
	return MapErrorValue(
		SatisfyChar(
			func(rune rune) bool {
				return strings.ContainsRune(chars, rune)
			},
		),
		NewError(fmt.Sprintf("content does not start with one of chars %#v", chars)),
	)
}

var Digit = OneOfChar("0123456789")

func NoneOfChar(chars string) ParseFn[rune] {
	return MapErrorValue(
		SatisfyChar(
			func(rune rune) bool {
				return !strings.ContainsRune(chars, rune)
			},
		),
		NewError(fmt.Sprintf("content starts with one of chars %#v", chars)),
	)
}

func Word(chars string) ParseFn[string] {
	return Map(
		Many1(OneOfChar(chars)),
		func(runes []rune) string {
			return string(runes)
		},
	)
}

// Convenience combinator to go from runes to string
func StringFromRunes(p ParseFn[[]rune]) ParseFn[string] {
	return Map(p, func(runes []rune) string {
		return string(runes)
	})
}

type Pair[T any, S any] struct {
	Left  T
	Right S
}

func ForwardRef[T any](fn func() ParseFn[T]) ParseFn[T] {
	return func(s State) Result[T] {
		p := fn()
		return p(s)
	}
}

func Compose[T any, S any](p1 ParseFn[T], p2 ParseFn[S]) ParseFn[Pair[T, S]] {
	return func(state State) Result[Pair[T, S]] {
		result1 := p1(state)
		if result1.error != nil {
			return Failed[Pair[T, S]](result1.error, state)
		}
		result2 := p2(result1.state)
		if result2.error != nil {
			return Failed[Pair[T, S]](result2.error, state)
		}
		return Ok(
			Pair[T, S]{
				Left:  result1.value,
				Right: result2.value,
			},
			result2.state,
		)
	}
}

func MapPair[T any, S any, R any](p ParseFn[Pair[T, S]], fn func(T, S) R) ParseFn[R] {
	return Map(
		p,
		func(pair Pair[T, S]) R {
			return fn(pair.Left, pair.Right)
		},
	)
}

func Map2[T any, S any, R any](p1 ParseFn[T], p2 ParseFn[S], fn func(T, S) R) ParseFn[R] {
	return MapPair(
		Compose(p1, p2),
		fn,
	)
}

func Map3[T any, S any, U any, R any](p1 ParseFn[T], p2 ParseFn[S], p3 ParseFn[U], fn func(T, S, U) R) ParseFn[R] {
	return Map(
		Compose(p1, Compose(p2, p3)),
		func(pair Pair[T, Pair[S, U]]) R {
			return fn(pair.Left, pair.Right.Left, pair.Right.Right)
		},
	)
}

func Map4[T any, S any, U any, V any, R any](p1 ParseFn[T], p2 ParseFn[S], p3 ParseFn[U], p4 ParseFn[V], fn func(T, S, U, V) R) ParseFn[R] {
	return Map(
		Compose(p1, Compose(p2, Compose(p3, p4))),
		func(pair Pair[T, Pair[S, Pair[U, V]]]) R {
			return fn(pair.Left, pair.Right.Left, pair.Right.Right.Left, pair.Right.Right.Right)
		},
	)
}

// Return first result matching one of the given parsers or error
// if none of the parsers match.
func Choice[T any](ps ...ParseFn[T]) ParseFn[T] {
	return func(state State) Result[T] {
		var errors []string
		for _, p := range ps {
			result := p(state)
			if result.error != nil {
				errors = append(errors, result.error.Error())
				continue
			}
			return Ok(result.value, result.state)
		}
		error := NewError(fmt.Sprint("no parser matched: ", strings.Join(errors, "; ")))
		return Failed[T](error, state)
	}
}

// Parse one of zero matches of the given parser.
// Returns the type's zero value on zero.
func OneOrZero[T any](p ParseFn[T]) ParseFn[T] {
	return func(state State) Result[T] {
		result := p(state)
		if result.error != nil {
			var zero T
			return Ok(zero, state)
		}
		return Ok(result.value, result.state)
	}
}

// Map parser of value to a parser of its pointer type
func Pointer[T any](p ParseFn[T]) ParseFn[*T] {
	return Map(
		p,
		func(value T) *T {
			return &value
		},
	)
}

func Map[T any, R any](p ParseFn[T], fn func(T) R) ParseFn[R] {
	return func(state State) Result[R] {
		result := p(state)
		if result.error != nil {
			return Failed[R](result.error, state)
		}
		return Ok(
			fn(result.value),
			result.state,
		)
	}
}

func MapValue[T any, R any](p ParseFn[T], value R) ParseFn[R] {
	return Map(
		p,
		func(v T) R {
			return value
		},
	)
}

func MapError[T any](p ParseFn[T], fn func(*Error, State) *Error) ParseFn[T] {
	return func(state State) Result[T] {
		result := p(state)
		if result.error != nil {
			error := fn(result.error, state)
			return Failed[T](error, state)
		}
		return result
	}
}

func MapErrorValue[T any](p ParseFn[T], error *Error) ParseFn[T] {
	return MapError(
		p,
		func(pe *Error, s State) *Error {
			return error
		},
	)
}

func Left[T any, S any](p ParseFn[Pair[T, S]]) ParseFn[T] {
	return MapPair(
		p,
		func(left T, right S) T {
			return left
		},
	)
}

func Right[T any, S any](p ParseFn[Pair[T, S]]) ParseFn[S] {
	return MapPair(
		p,
		func(left T, right S) S {
			return right
		},
	)
}

func DiscardLeft[T any, S any](p1 ParseFn[T], p2 ParseFn[S]) ParseFn[S] {
	p := Compose(p1, p2)
	return Right(p)
}

func DiscardRight[T any, S any](p1 ParseFn[T], p2 ParseFn[S]) ParseFn[T] {
	p := Compose(p1, p2)
	return Left(p)
}

func DiscardMiddle[T any, S any, U any](opening ParseFn[T], p ParseFn[S], closing ParseFn[U]) ParseFn[Pair[T, U]] {
	return Compose(
		opening,
		DiscardLeft(
			p,
			closing,
		),
	)
}

func Between[T any, S any, U any](opening ParseFn[T], p ParseFn[S], closing ParseFn[U]) ParseFn[S] {
	return DiscardRight(
		DiscardLeft(
			opening,
			p,
		),
		closing,
	)
}

func Many[T any](p ParseFn[T]) ParseFn[[]T] {
	return func(state State) Result[[]T] {
		var results []T
		for len(state.Rest()) > 0 {
			result := p(state)
			state = result.state
			if result.error != nil {
				break
			}
			results = append(results, result.value)
		}
		return Ok(results, state)
	}
}

// Construct list from list head and existing list
func Cons[T any](left ParseFn[T], right ParseFn[[]T]) ParseFn[[]T] {
	return Map2(
		left,
		right,
		func(head T, tail []T) []T {
			values := make([]T, 0, 1+len(tail))
			values = append(values, head)
			values = append(values, tail...)
			return values
		},
	)
}

type ReduceFn[T any] = func(T, T) T

func Reduce[T any](fn ReduceFn[T], ps ...ParseFn[T]) ParseFn[T] {
	return func(state State) Result[T] {
		var acc T
		nextState := state
		for _, p := range ps {
			result := p(nextState)
			if result.error != nil {
				return Failed[T](result.error, state)
			}
			acc = fn(acc, result.value)
			nextState = result.state
		}
		return Ok(acc, nextState)
	}
}

func Concat[T any](p ParseFn[Pair[[]T, []T]]) ParseFn[[]T] {
	return Map(
		p,
		func(value Pair[[]T, []T]) []T {
			values := make([]T, 0, len(value.Left)+len(value.Right))
			values = append(values, value.Left...)
			values = append(values, value.Right...)
			return values
		},
	)
}

func Many1[T any](p ParseFn[T]) ParseFn[[]T] {
	return Cons(p, Many(p))
}

func Count[T any](p ParseFn[T], count int) ParseFn[[]T] {
	return func(state State) Result[[]T] {
		var results []T
		nextState := state
		for !nextState.Eof() && len(results) < count {
			result := p(nextState)
			nextState = result.state
			if result.error != nil {
				break
			}
			results = append(results, result.value)
		}
		if len(results) < count {
			return Failed[[]T](NewError(fmt.Sprintf("parsed %d < %d results", len(results), count)), state)
		}
		return Ok(results, nextState)
	}
}

// Memoize the result of the given parser by position
func Memoize[T any](p ParseFn[T]) ParseFn[T] {
	cache := make(map[StateCacheKey]Result[T])

	return func(state State) Result[T] {
		// Return cached result
		cacheKey := state.CacheKey()
		if result, ok := cache[cacheKey]; ok {
			return result
		}

		// Run parser and cache result
		result := p(state)
		cache[cacheKey] = result
		return result
	}
}
