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
	"strings"
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

// TerminalString pattern
type TerminalString struct {
	T      TransformFunction
	String string
}

// CharacterGroup pattern, for instance whitespace group
type CharacterGroup struct {
	T     TransformFunction
	Group CharacterGroupFunction
}

// CharacterRange pattern
type CharacterRange struct {
	T       TransformFunction
	Low     rune
	High    rune
	Outside bool
}

// CharacterEnum pattern
type CharacterEnum struct {
	T       TransformFunction
	Enum    string
	Outside bool
}

// Alternation pattern
type Alternation struct {
	T        TransformFunction
	Patterns []Pattern
}

// Concatenation pattern
type Concatenation struct {
	T        TransformFunction
	Patterns []Pattern
}

// Repetition pattern
type Repetition struct {
	T       TransformFunction
	Min     int
	Max     int
	Pattern Pattern
}

// Exception pattern
type Exception struct {
	T         TransformFunction
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

// Transform matchResult
func (s *TerminalString) Transform(m *MatchResult) {
	if s.T != nil {
		s.T(m)
	}
}

// Transform matchResult
func (g *CharacterGroup) Transform(m *MatchResult) {
	if g.T != nil {
		g.T(m)
	}
}

// Transform matchResult
func (cr *CharacterRange) Transform(m *MatchResult) {
	if cr.T != nil {
		cr.T(m)
	}
}

// Transform matchResult
func (ce *CharacterEnum) Transform(m *MatchResult) {
	if ce.T != nil {
		ce.T(m)
	}
}

// Transform matchResult
func (a *Alternation) Transform(m *MatchResult) {
	if a.T != nil {
		a.T(m)
	}
}

// Transform matchResult
func (c *Concatenation) Transform(m *MatchResult) {
	if c.T != nil {
		c.T(m)
	}
}

// Transform matchResult
func (e *Exception) Transform(m *MatchResult) {
	if e.T != nil {
		e.T(m)
	}
}

// Transform matchResult
func (rep *Repetition) Transform(m *MatchResult) {
	if rep.T != nil {
		rep.T(m)
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

	result.Match = g.Group(rn)
	if result.Match {
		result.Result = r.String()
		g.Transform(result)
		r.PopPos()
	} else {
		r.RestorePos()
	}

	return result, nil
}

// Match a character from a range
func (cr *CharacterRange) Match(r *Reader) (*MatchResult, error) {
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

	if cr.Outside {
		result.Match = (rn < cr.Low || rn > cr.High)
	} else {
		result.Match = (rn >= cr.Low && rn <= cr.High)
	}

	if result.Match {
		result.Result = r.String()
		cr.Transform(result)
		r.PopPos()
	} else {
		r.RestorePos()
	}

	return result, nil
}

// Match a character from a string enum
func (ce *CharacterEnum) Match(r *Reader) (*MatchResult, error) {
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

	if ce.Outside {
		result.Match = !strings.ContainsRune(ce.Enum, rn)
	} else {
		result.Match = strings.ContainsRune(ce.Enum, rn)
	}

	if result.Match {
		result.Result = r.String()
		ce.Transform(result)
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
