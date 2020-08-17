/*
Package ebnf allows to construct a set of EBNF rules and provides a reader and struct/methods to
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
	"log"
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

// Transform for base transformer
func (b *BaseTransformer) Transform(m *MatchResult) {
	if b.T != nil {
		b.T(m)
	}
}

// TerminalString pattern
type TerminalString struct {
	BaseTransformer
	String string
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

// Match a terminal string
func (s *TerminalString) Match(r *Reader) (*MatchResult, error) {
	r.PushState()

	result := &MatchResult{Match: false}

	for _, rn1 := range []rune(s.String) {
		rn2, err := r.Read()
		log.Printf("rn2 %v\n", rn2)
		if err != nil {
			r.RestoreState()

			if err == io.EOF {
				return result, nil
			}

			return nil, err
		}

		if rn1 != rn2 {
			r.RestoreState()
			return result, nil
		}
	}

	result.Match = true
	result.Result = r.String()

	s.Transform(result)

	r.PopState()

	return result, nil
}

// CharacterGroup pattern, test membership of a group, for instance whitespace group
type CharacterGroup struct {
	BaseTransformer
	Group   CharacterGroupFunction
	Outside bool
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

// Match a character from a group
func (g *CharacterGroup) Match(r *Reader) (*MatchResult, error) {
	r.PushState()

	result := &MatchResult{Match: false}

	rn, err := r.Read()
	if err == io.EOF {
		r.RestoreState()
		return result, nil
	}

	if err != nil {
		r.RestoreState()
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
		r.PopState()
	} else {
		r.RestoreState()
	}

	return result, nil
}

// Alternation pattern
type Alternation struct {
	BaseTransformer
	Patterns []Pattern
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

// Match alternation pattern, matches if one of the alternating patterns matches
func (a *Alternation) Match(r *Reader) (*MatchResult, error) {
	for _, p := range a.Patterns {
		finished := r.Finished()

		if finished {
			break
		}

		r.PushState()

		result, err := p.Match(r)
		if err != nil {
			r.RestoreState()
			return nil, err
		}

		if result.Match {
			a.Transform(result)
			r.PopState()
			return result, nil
		}

		r.RestoreState()
	}

	return &MatchResult{
		Match: false,
	}, nil
}

// Concatenation pattern
type Concatenation struct {
	BaseTransformer
	Patterns []Pattern
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

// Match concatenation pattern
func (c *Concatenation) Match(r *Reader) (*MatchResult, error) {
	matches := []*MatchResult{}

	r.PushState()

	for _, p := range c.Patterns {
		result, err := p.Match(r)
		if err != nil {
			r.RestoreState()
			return nil, err
		}

		if !result.Match {
			r.RestoreState()
			return &MatchResult{
				Match: false,
			}, nil
		}

		matches = append(matches, result)
	}

	r.PopState()

	result := &MatchResult{
		Match:  true,
		Result: matches,
	}

	c.Transform(result)

	return result, nil
}

// Repetition pattern
type Repetition struct {
	BaseTransformer
	Min     int
	Max     int
	Pattern Pattern
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

// Match repetition pattern
func (rep *Repetition) Match(r *Reader) (*MatchResult, error) {
	matches := []*MatchResult{}

	r.PushState()

	for {
		finished := r.Finished()
		if finished {
			break
		}

		result, err := rep.Pattern.Match(r)
		if err != nil {
			r.RestoreState()
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
		r.RestoreState()
		return &MatchResult{
			Match: false,
		}, nil
	}

	r.PopState()

	result := &MatchResult{
		Match:  true,
		Result: matches,
	}

	rep.Transform(result)

	return result, nil
}

// Exception pattern
type Exception struct {
	BaseTransformer
	MustMatch Pattern
	Except    Pattern
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

// Match exception pattern
func (e *Exception) Match(r *Reader) (result *MatchResult, err error) {
	r.PushState()

	result, err = e.Except.Match(r)
	if err != nil {
		return
	}

	if result.Match {
		r.RestoreState()
		result.Match = false
		result.Result = nil
		return
	}

	r.PopState()

	result, err = e.MustMatch.Match(r)

	e.Transform(result)

	return
}

// EOF pattern
type EOF struct {
	BaseTransformer
}

// NewEOF creates a new end of file
func NewEOF() *EOF {
	return &EOF{}
}

// NewEOFT creates a new end of file with custom transform function
func NewEOFT(t TransformFunction) *EOF {
	return &EOF{
		BaseTransformer: BaseTransformer{
			T: t,
		},
	}
}

// Match end of file pattern
func (e *EOF) Match(r *Reader) (result *MatchResult, err error) {
	match := r.Finished()

	result = &MatchResult{
		Match: match,
	}

	return
}

// EBNF pattern
type EBNF struct {
	RootRule string
	Rules    map[string]Pattern
}

// NewEBNF creates a new EBNF parser
func NewEBNF(rootrule string, rules map[string]Pattern) *EBNF {
	return &EBNF{
		RootRule: rootrule,
		Rules:    rules,
	}
}

// Match EBNF pattern
func (e *EBNF) Match(r *Reader) (*MatchResult, error) {
	return e.Rules[e.RootRule].Match(r)
}
