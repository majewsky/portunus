// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package ldapproto

import (
	"fmt"

	"github.com/majewsky/portunus/internal/ber"
)

// SearchRequest represents the SearchRequest type defined in [RFC 4511, 4.2].
// It is a choice for type [Operation].
type SearchRequest struct {
	BaseObject   DN
	Scope        SearchScope
	DerefAliases DerefAliasesStrategy
	SizeLimit    uint32
	TimeLimit    uint32
	TypesOnly    bool
	Filter       ber.ChoiceOf[Filter]
	Attributes   []String
}

// SearchScope is an enum appearing in type [SearchRequest].
type SearchScope int

var _ ber.Enum = SearchScopeBaseObject

const (
	// SearchScopeBaseObject constrains the search to the entry named by BaseObject.
	SearchScopeBaseObject SearchScope = 0
	// SearchScopeSingleLevel constrains the search to the immediate subordinates of the entry named by BaseObject.
	SearchScopeSingleLevel SearchScope = 1
	// SearchScopeWholeSubtree constrains the search to the entry named by BaseObject and to all its subordinates.
	SearchScopeWholeSubtree SearchScope = 2
)

// IsEnum implements the [ber.Enum] interface.
func (SearchScope) IsEnum() bool { return true }

// Validate implements the [ber.Enum] interface.
func (s SearchScope) Validate() error {
	switch s {
	case SearchScopeBaseObject, SearchScopeSingleLevel, SearchScopeWholeSubtree:
		return nil
	default:
		return fmt.Errorf("unacceptable value for SearchScope: %d", s)
	}
}

// DerefAliasesStrategy is an enum appearing in type [SearchRequest].
type DerefAliasesStrategy int

var _ ber.Enum = NeverDerefAliases

const (
	// NeverDerefAliases forbids dereferencing aliases both in searching or in locating the base object of the search.
	NeverDerefAliases DerefAliasesStrategy = 0
	// DerefAliasesInSearching allows dereferencing aliases only during searching, but not while locating the base object of the search.
	DerefAliasesInSearching DerefAliasesStrategy = 1
	// DerefAliasesFindingBaseObject allows dereferencing aliases while locating the base object of the search, but not while searching.
	DerefAliasesFindingBaseObject DerefAliasesStrategy = 2
	// DerefAliasesAlways allows dereferencing aliases both in searching or in locating the base object of the search.
	DerefAliasesAlways DerefAliasesStrategy = 3
)

// IsEnum implements the [ber.Enum] interface.
func (DerefAliasesStrategy) IsEnum() bool { return true }

// Validate implements the [ber.Enum] interface.
func (s DerefAliasesStrategy) Validate() error {
	switch s {
	case NeverDerefAliases, DerefAliasesInSearching, DerefAliasesFindingBaseObject, DerefAliasesAlways:
		return nil
	default:
		return fmt.Errorf("unacceptable value for DerefAliasesStrategy: %d", s)
	}
}

// SearchResultEntry represents the SearchResultEntry type defined in [RFC 4511, 4.2].
// It is a choice for type [Operation].
type SearchResultEntry struct {
	ObjectName DN
	Attributes []Attribute
}

// Attribute appears in type [SearchResultEntry] (TODO: and more).
type Attribute struct {
	Type   String
	Values ber.SetOf[string]
}

// SearchResultDone represents the SearchResultDone type defined in [RFC 4511, 4.2].
// It is a choice for type [Operation].
type SearchResultDone Result

// SearchResultReference represents the SearchResultReference type defined in [RFC 4511, 4.2].
// It is a choice for type [Operation].
type SearchResultReference []URI

////////////////////////////////////////////////////////////////////////////////
// type Filter and its subtypes

// Filter appears in type [SearchRequest].
type Filter struct {
	AndFilter             `ber:"context-specific:0"`
	OrFilter              `ber:"context-specific:1"`
	NotFilter             `ber:"context-specific:2"` // TODO: how the fuck is this encoded?
	EqualityMatchFilter   `ber:"context-specific:3"`
	SubstringsFilter      `ber:"context-specific:4"`
	GreaterOrEqualFilter  `ber:"context-specific:5"`
	LessOrEqualFilter     `ber:"context-specific:6"`
	PresentFilter         `ber:"context-specific:7"`
	ApproxMatchFilter     `ber:"context-specific:8"`
	ExtensibleMatchFilter `ber:"context-specific:9"`
}

var (
	_ ber.ChoiceOf[Filter] = AndFilter{}
	_ ber.ChoiceOf[Filter] = OrFilter{}
	_ ber.ChoiceOf[Filter] = NotFilter{}
	_ ber.ChoiceOf[Filter] = EqualityMatchFilter{}
	_ ber.ChoiceOf[Filter] = SubstringsFilter{}
	_ ber.ChoiceOf[Filter] = GreaterOrEqualFilter{}
	_ ber.ChoiceOf[Filter] = LessOrEqualFilter{}
	_ ber.ChoiceOf[Filter] = PresentFilter("")
	_ ber.ChoiceOf[Filter] = ApproxMatchFilter{}
	_ ber.ChoiceOf[Filter] = ExtensibleMatchFilter{}
)

// Into implements the [ber.ChoiceOf] interface.
func (f AndFilter) Into() Filter { return Filter{AndFilter: f} }

// Into implements the [ber.ChoiceOf] interface.
func (f OrFilter) Into() Filter { return Filter{OrFilter: f} }

// Into implements the [ber.ChoiceOf] interface.
func (f NotFilter) Into() Filter { return Filter{NotFilter: f} }

// Into implements the [ber.ChoiceOf] interface.
func (f EqualityMatchFilter) Into() Filter { return Filter{EqualityMatchFilter: f} }

// Into implements the [ber.ChoiceOf] interface.
func (f SubstringsFilter) Into() Filter { return Filter{SubstringsFilter: f} }

// Into implements the [ber.ChoiceOf] interface.
func (f GreaterOrEqualFilter) Into() Filter { return Filter{GreaterOrEqualFilter: f} }

// Into implements the [ber.ChoiceOf] interface.
func (f LessOrEqualFilter) Into() Filter { return Filter{LessOrEqualFilter: f} }

// Into implements the [ber.ChoiceOf] interface.
func (f PresentFilter) Into() Filter { return Filter{PresentFilter: f} }

// Into implements the [ber.ChoiceOf] interface.
func (f ApproxMatchFilter) Into() Filter { return Filter{ApproxMatchFilter: f} }

// Into implements the [ber.ChoiceOf] interface.
func (f ExtensibleMatchFilter) Into() Filter { return Filter{ExtensibleMatchFilter: f} }

// AndFilter is a choice for type [Filter].
type AndFilter []ber.SetOf[ber.ChoiceOf[Filter]]

// OrFilter is a choice for type [Filter].
// OrFilter is a choice for type [Filter].
type OrFilter []ber.SetOf[ber.ChoiceOf[Filter]]

// NotFilter is a choice for type [Filter].
type NotFilter struct {
	Inner ber.ChoiceOf[Filter]
	// NOTE: This struct represents the fact that the chosen type `[2] Filter` requires explicit tagging according to [X.680, 31.2.7c].
}

// EqualityMatchFilter is a choice for type [Filter].
type EqualityMatchFilter AttributeValueAssertion

// SubstringsFilter is a choice for type [Filter].
type SubstringsFilter []SubstringFilter

// GreaterOrEqualFilter is a choice for type [Filter].
type GreaterOrEqualFilter AttributeValueAssertion

// LessOrEqualFilter is a choice for type [Filter].
type LessOrEqualFilter AttributeValueAssertion

// PresentFilter is a choice for type [Filter].
type PresentFilter AttributeDescription

// ApproxMatchFilter is a choice for type [Filter].
type ApproxMatchFilter AttributeValueAssertion

// ExtensibleMatchFilter is a choice for type [Filter].
type ExtensibleMatchFilter MatchingRuleAssertion

// SubstringFilter appears in type [SubstringsFilter].
type SubstringFilter struct {
	Type       AttributeDescription
	Substrings []ber.ChoiceOf[Substring]
}

// Substring appears in type [SubstringFilter].
type Substring struct {
	InitialSubstring `ber:"context-specific:0"`
	AnySubstring     `ber:"context-specific:1"`
	FinalSubstring   `ber:"context-specific:2"`
}

var (
	_ ber.ChoiceOf[Substring] = InitialSubstring("")
	_ ber.ChoiceOf[Substring] = AnySubstring("")
	_ ber.ChoiceOf[Substring] = FinalSubstring("")
)

// Into implements the [ber.ChoiceOf] interface.
func (s InitialSubstring) Into() Substring { return Substring{InitialSubstring: s} }

// Into implements the [ber.ChoiceOf] interface.
func (s AnySubstring) Into() Substring { return Substring{AnySubstring: s} }

// Into implements the [ber.ChoiceOf] interface.
func (s FinalSubstring) Into() Substring { return Substring{FinalSubstring: s} }

// InitialSubstring is a choice for type [Substring].
type InitialSubstring AssertionValue

// AnySubstring is a choice for type [Substring].
type AnySubstring AssertionValue

// FinalSubstring is a choice for type [Substring].
type FinalSubstring AssertionValue

// MatchingRuleAssertion appears in type [ExtensibleMatchFilter].
type MatchingRuleAssertion struct {
	MatchingRule MatchingRuleID       `ber:"context-specific:1,optional"`
	Type         AttributeDescription `ber:"context-specific:2,optional"`
	MatchValue   AssertionValue       `ber:"context-specific:3"`
	DNAttributes bool                 `ber:"context-specific:4,optional"` // NOTE: actually declared with "DEFAULT FALSE"
}
