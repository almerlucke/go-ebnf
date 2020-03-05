/*
Package ebnf allows to construct a set of EBNF rules and provides a reader to
define, parse and validate any type of context-free grammar. ebfn_test.go shows
an example of how to describe a simple pascal like syntax as shown in
https://en.wikipedia.org/wiki/Extended_Backus%E2%80%93Naur_form. For each pattern
you can specify a transform function which can be used to transform the default
matched result output to a custom domain, for instance to construct language
source trees
*/
package ebnf

import (
	"io"
)

// MatchResult contains the result of a match
type MatchResult struct {
	Match  bool
	Result interface{}
}

// TransformFunction for match result
type TransformFunction func(r *MatchResult)

// CharacterGroupFunction check if rune is part of group
type CharacterGroupFunction func(r rune) bool

// Pattern to match
type Pattern interface {
	Match(r *Reader) (*MatchResult, error)
}

// Transformer for match result
type Transformer interface {
	Transform(m *MatchResult)
}

// BaseTransformer implements transformer interface
type BaseTransformer struct {
	T TransformFunction
}

// TerminalString pattern
type TerminalString struct {
	BaseTransformer
	String string
}

// CharacterGroup pattern, for instance whitespace group
type CharacterGroup struct {
	BaseTransformer
	Group   CharacterGroupFunction
	Outside bool
}

// Alternation pattern
type Alternation struct {
	BaseTransformer
	Patterns []Pattern
}

// Concatenation pattern
type Concatenation struct {
	BaseTransformer
	Patterns []Pattern
}

// Repetition pattern
type Repetition struct {
	BaseTransformer
	Min     int
	Max     int
	Pattern Pattern
}

// Exception pattern
type Exception struct {
	BaseTransformer
	MustMatch Pattern
	Except    Pattern
}

// EBNF pattern
type EBNF struct {
	RootRule string
	Rules    map[string]Pattern
}

// NewEBNF creates a new EBNF parser
func NewEBNF() *EBNF {
	return &EBNF{
		RootRule: "",
		Rules:    map[string]Pattern{},
	}
}

// Match EBNF pattern
func (e *EBNF) Match(r *Reader) (*MatchResult, error) {
	return e.Rules[e.RootRule].Match(r)
}

// Transform for base transformer
func (b *BaseTransformer) Transform(m *MatchResult) {
	if b.T != nil {
		b.T(m)
	}
}

// NewTerminalString creates a new terminal string
func NewTerminalString(s string) *TerminalString {
	return &TerminalString{
		String: s,
	}
}

// NewTerminalStringT creates a new terminal string with custom transform function
func NewTerminalStringT(t TransformFunction, s string) *TerminalString {
	return &TerminalString{
		BaseTransformer: BaseTransformer{
			T: t,
		},
		String: s,
	}
}

// NewCharacterGroup creates a new character group
func NewCharacterGroup(f CharacterGroupFunction) *CharacterGroup {
	return &CharacterGroup{
		Group: f,
	}
}

// NewCharacterGroupT creates a new character group with custom transform function
func NewCharacterGroupT(t TransformFunction, f CharacterGroupFunction) *CharacterGroup {
	return &CharacterGroup{
		BaseTransformer: BaseTransformer{
			T: t,
		},
		Group: f,
	}
}

// NewAlternation creates a new alternation pattern
func NewAlternation(patterns ...Pattern) *Alternation {
	return &Alternation{
		Patterns: patterns,
	}
}

// NewAlternationT creates a new alternation pattern with custom transform function
func NewAlternationT(t TransformFunction, patterns ...Pattern) *Alternation {
	return &Alternation{
		Patterns: patterns,
		BaseTransformer: BaseTransformer{
			T: t,
		},
	}
}

// NewConcatenation creates a new concatenation pattern
func NewConcatenation(patterns ...Pattern) *Concatenation {
	return &Concatenation{
		Patterns: patterns,
	}
}

// NewConcatenationT creates a new concatenation pattern with custom transform function
func NewConcatenationT(t TransformFunction, patterns ...Pattern) *Concatenation {
	return &Concatenation{
		Patterns: patterns,
		BaseTransformer: BaseTransformer{
			T: t,
		},
	}
}

// NewRepetition creates a new repetition pattern
func NewRepetition(min int, max int, p Pattern) *Repetition {
	return &Repetition{
		Min:     min,
		Max:     max,
		Pattern: p,
	}
}

// NewRepetitionT creates a new repetition pattern with custom transform function
func NewRepetitionT(t TransformFunction, min int, max int, p Pattern) *Repetition {
	return &Repetition{
		BaseTransformer: BaseTransformer{
			T: t,
		},
		Min:     min,
		Max:     max,
		Pattern: p,
	}
}

// NewException creates a new exception
func NewException(mustMatch Pattern, except Pattern) *Exception {
	return &Exception{
		MustMatch: mustMatch,
		Except:    except,
	}
}

// NewExceptionT creates a new exception with custom transform function
func NewExceptionT(t TransformFunction, mustMatch Pattern, except Pattern) *Exception {
	return &Exception{
		BaseTransformer: BaseTransformer{
			T: t,
		},
		MustMatch: mustMatch,
		Except:    except,
	}
}

// Match a terminal string
func (s *TerminalString) Match(r *Reader) (*MatchResult, error) {
	r.SavePos()

	result := &MatchResult{Match: false}

	for _, rn1 := range []rune(s.String) {
		rn2, err := r.Read()
		if err != nil {
			r.RestorePos()

			if err == io.EOF {
				return result, nil
			}

			return nil, err
		}

		if rn1 != rn2 {
			r.RestorePos()
			return result, nil
		}
	}

	result.Match = true
	result.Result = r.String()

	s.Transform(result)

	r.PopPos()

	return result, nil
}

// Match a character from a group
func (g *CharacterGroup) Match(r *Reader) (*MatchResult, error) {
	r.SavePos()

	result := &MatchResult{Match: false}

	rn, err := r.Read()
	if err == io.EOF {
		r.RestorePos()
		return result, nil
	}

	if err != nil {
		r.RestorePos()
		return nil, err
	}

	if g.Outside {
		result.Match = !g.Group(rn)
	} else {
		result.Match = g.Group(rn)
	}

	if result.Match {
		result.Result = r.String()
		g.Transform(result)
		r.PopPos()
	} else {
		r.RestorePos()
	}

	return result, nil
}

// Match alternation pattern, matches if one of the alternating patterns matches
func (a *Alternation) Match(r *Reader) (*MatchResult, error) {
	for _, p := range a.Patterns {
		finished, err := r.Finished()
		if err != nil {
			return nil, err
		}

		if finished {
			break
		}

		r.SavePos()

		result, err := p.Match(r)
		if err != nil {
			r.RestorePos()
			return nil, err
		}

		if result.Match {
			a.Transform(result)
			r.PopPos()
			return result, nil
		}

		r.RestorePos()
	}

	return &MatchResult{
		Match: false,
	}, nil
}

// Match concatenation pattern
func (c *Concatenation) Match(r *Reader) (*MatchResult, error) {
	matches := []*MatchResult{}

	r.SavePos()

	for _, p := range c.Patterns {
		result, err := p.Match(r)
		if err != nil {
			r.RestorePos()
			return nil, err
		}

		if !result.Match {
			r.RestorePos()
			return &MatchResult{
				Match: false,
			}, nil
		}

		matches = append(matches, result)
	}

	r.PopPos()

	result := &MatchResult{
		Match:  true,
		Result: matches,
	}

	c.Transform(result)

	return result, nil
}

// Match exception pattern
func (e *Exception) Match(r *Reader) (result *MatchResult, err error) {
	r.SavePos()

	result, err = e.Except.Match(r)
	if err != nil {
		return
	}

	if result.Match {
		r.RestorePos()
		result.Match = false
		result.Result = nil
		return
	}

	r.PopPos()

	result, err = e.MustMatch.Match(r)

	e.Transform(result)

	return
}

// Match repetition pattern
func (rep *Repetition) Match(r *Reader) (*MatchResult, error) {
	matches := []*MatchResult{}

	r.SavePos()

	for {
		finished, err := r.Finished()
		if err != nil {
			r.RestorePos()
			return nil, err
		}

		if finished {
			break
		}

		result, err := rep.Pattern.Match(r)
		if err != nil {
			r.RestorePos()
			return nil, err
		}

		if result.Match {
			matches = append(matches, result)

			if rep.Max != 0 && len(matches) == rep.Max {
				break
			}
		} else {
			break
		}
	}

	if len(matches) < rep.Min {
		r.RestorePos()
		return &MatchResult{
			Match: false,
		}, nil
	}

	r.PopPos()

	result := &MatchResult{
		Match:  true,
		Result: matches,
	}

	rep.Transform(result)

	return result, nil
}
