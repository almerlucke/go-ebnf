package main

import (
	"bufio"
	"io"
	"log"
	"strings"
	"unicode"
)

/*
terminal symbols -> "..." | '...', special sequence (for instance ? whitespace ?)
production rule -> alternation, concatenation, optional, repetition, grouping, exception
non terminals
*/

// Reader buffers runes read until they are consumed, this allows us to read runes ahead
// and go back (reset) when the runes do not match a pattern
type Reader struct {
	reader              *bufio.Reader
	runeBuf             []rune
	runeBufPos          int
	runeBufPrevPosStack []int
}

type PatternType int

const (
	TerminalStringPattern PatternType = iota
	TerminalCharacterGroupPattern
	AlternationPattern
	ConcatenationPattern
	OptionalPattern
	RepetitionPattern
	ExceptionPattern
)

// MatchResult contains the result of a match
type MatchResult struct {
	Match      bool
	Result     interface{}
	Identifier string
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

// TerminalCharacterGroup pattern, for instance whitespace group
type TerminalCharacterGroup struct {
	T     TransformFunction
	Group CharacterGroupFunction
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

// type Element struct {
// 	Pattern Pattern
// 	Type    Pattern
// }

// NewReader creates a new reader
func NewReader(r io.Reader) *Reader {
	return &Reader{
		reader:              bufio.NewReader(r),
		runeBuf:             []rune{},
		runeBufPrevPosStack: []int{0},
	}
}

// SavePos pushes the current buffer position on the stack
func (r *Reader) SavePos() {
	r.runeBufPrevPosStack = append(r.runeBufPrevPosStack, r.runeBufPos)
}

// RestorePos pops and restores the buffer position to the last pushed buffer position from the stack
func (r *Reader) RestorePos() {
	l := len(r.runeBufPrevPosStack) - 1
	r.runeBufPos, r.runeBufPrevPosStack = r.runeBufPrevPosStack[l], r.runeBufPrevPosStack[:l]
}

// PopPos pops the last pushed buffer position from the stack without restoring
func (r *Reader) PopPos() {
	l := len(r.runeBufPrevPosStack) - 1
	r.runeBufPrevPosStack = r.runeBufPrevPosStack[:l]
}

// String gets the current buffer content between the previous pos and the current pos as string
func (r *Reader) String() string {
	prevPos := r.runeBufPrevPosStack[len(r.runeBufPrevPosStack)-1]
	return string(r.runeBuf[prevPos:r.runeBufPos])
}

// Finished returns true if EOF is reached
func (r *Reader) Finished() (bool, error) {
	_, err := r.Peak()
	if err != nil {
		if err == io.EOF {
			return true, nil
		}
		return false, err
	}

	return false, nil
}

// Peak returns the next rune without advancing the read position
func (r *Reader) Peak() (rn rune, err error) {
	if r.runeBufPos < len(r.runeBuf) {
		rn = r.runeBuf[r.runeBufPos]
	} else {
		rn, _, err = r.reader.ReadRune()
		if err == nil {
			err = r.reader.UnreadRune()
		}
	}

	return
}

// Read returns the next rune and advances the read position
func (r *Reader) Read() (rn rune, err error) {
	if r.runeBufPos < len(r.runeBuf) {
		rn = r.runeBuf[r.runeBufPos]
		r.runeBufPos++
	} else {
		rn, _, err = r.reader.ReadRune()
		if err == nil {
			r.runeBuf = append(r.runeBuf, rn)
			r.runeBufPos++
		}
	}

	return
}

// Transform matchResult
func (s *TerminalString) Transform(m *MatchResult) {
	if s.T != nil {
		s.T(m)
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

// Transform matchResult
func (g *TerminalCharacterGroup) Transform(m *MatchResult) {
	if g.T != nil {
		g.T(m)
	}
}

// Match a character from a group
func (g *TerminalCharacterGroup) Match(r *Reader) (*MatchResult, error) {
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

// Transform matchResult
func (a *Alternation) Transform(m *MatchResult) {
	if a.T != nil {
		a.T(m)
	}
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

// Transform matchResult
func (c *Concatenation) Transform(m *MatchResult) {
	if c.T != nil {
		c.T(m)
	}
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

// Transform matchResult
func (e *Exception) Transform(m *MatchResult) {
	if e.T != nil {
		e.T(m)
	}
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

// Transform matchResult
func (rep *Repetition) Transform(m *MatchResult) {
	if rep.T != nil {
		rep.T(m)
	}
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

/**
*
*	Test transform functionality
*
**/

type assignment struct {
	Identifier string
	Value      string
}

type program struct {
	Identifier  string
	Assignments []*assignment
}

func identifierTransform(m *MatchResult) {
	params := m.Result.([]*MatchResult)
	var builder strings.Builder

	builder.WriteString(params[0].Result.(string))

	charResults := params[1].Result.([]*MatchResult)
	for _, m := range charResults {
		builder.WriteString(m.Result.(string))
	}

	m.Result = builder.String()
}

func numberTransform(m *MatchResult) {
	var builder strings.Builder
	charResults := m.Result.([]*MatchResult)

	for _, m := range charResults {
		builder.WriteString(m.Result.(string))
	}

	m.Result = builder.String()
}

func stringTransform(m *MatchResult) {
	var builder strings.Builder
	params := m.Result.([]*MatchResult)

	charResults := params[1].Result.([]*MatchResult)
	for _, m := range charResults {
		builder.WriteString(m.Result.(string))
	}

	m.Result = builder.String()
}

func assignmentTransform(m *MatchResult) {
	params := m.Result.([]*MatchResult)

	for _, param := range params {
		log.Printf("params %v\n", *param)
	}

	identifier := params[0].Result.(string)
	value := params[2].Result.(string)

	m.Result = &assignment{
		Identifier: identifier,
		Value:      value,
	}
}

func programTransform(m *MatchResult) {
	program := &program{}

	params := m.Result.([]*MatchResult)
	program.Identifier = params[2].Result.(string)

	assignments := []*assignment{}
	assignmentResults := params[6].Result.([]*MatchResult)

	for _, assignmentResult := range assignmentResults {
		assignment := assignmentResult.Result.([]*MatchResult)[0].Result.(*assignment)
		assignments = append(assignments, assignment)
	}

	program.Assignments = assignments

	m.Result = program
}

func main() {
	reader := NewReader(strings.NewReader("PROGRAM DEMO1\nBEGIN\nA:=\"test\";\nTEST:=12234;\nEND"))
	ebnf := NewEBNF()

	ebnf.Rules["whitespace"] = &TerminalCharacterGroup{Group: unicode.IsSpace}
	ebnf.Rules["visible_character"] = &TerminalCharacterGroup{Group: unicode.IsPrint}
	ebnf.Rules["digit"] = &TerminalCharacterGroup{Group: unicode.IsDigit}
	ebnf.Rules["alphabetic_character"] = &Alternation{
		Patterns: []Pattern{
			&TerminalString{String: "A"}, &TerminalString{String: "B"}, &TerminalString{String: "C"}, &TerminalString{String: "D"}, &TerminalString{String: "E"},
			&TerminalString{String: "F"}, &TerminalString{String: "G"}, &TerminalString{String: "H"}, &TerminalString{String: "I"}, &TerminalString{String: "J"},
			&TerminalString{String: "K"}, &TerminalString{String: "L"}, &TerminalString{String: "M"}, &TerminalString{String: "N"}, &TerminalString{String: "O"},
			&TerminalString{String: "P"}, &TerminalString{String: "Q"}, &TerminalString{String: "R"}, &TerminalString{String: "S"}, &TerminalString{String: "T"},
			&TerminalString{String: "U"}, &TerminalString{String: "V"}, &TerminalString{String: "W"}, &TerminalString{String: "X"}, &TerminalString{String: "Y"},
			&TerminalString{String: "Z"},
		},
	}

	ebnf.Rules["identifier"] = &Concatenation{
		T: identifierTransform,
		Patterns: []Pattern{
			ebnf.Rules["alphabetic_character"],
			&Repetition{
				Min: 0,
				Max: 0,
				Pattern: &Alternation{
					Patterns: []Pattern{
						ebnf.Rules["alphabetic_character"],
						ebnf.Rules["digit"],
					},
				},
			},
		},
	}

	ebnf.Rules["number"] = &Repetition{
		T:       numberTransform,
		Min:     1,
		Max:     0,
		Pattern: ebnf.Rules["digit"],
	}

	ebnf.Rules["string"] = &Concatenation{
		T: stringTransform,
		Patterns: []Pattern{
			&TerminalString{String: "\""},
			&Repetition{
				Min: 0,
				Max: 0,
				Pattern: &Exception{
					MustMatch: ebnf.Rules["visible_character"],
					Except:    &TerminalString{String: "\""},
				},
			},
			&TerminalString{String: "\""},
		},
	}

	ebnf.Rules["assignment"] = &Concatenation{
		T: assignmentTransform,
		Patterns: []Pattern{
			ebnf.Rules["identifier"],
			&TerminalString{String: ":="},
			&Alternation{
				Patterns: []Pattern{
					ebnf.Rules["number"],
					ebnf.Rules["identifier"],
					ebnf.Rules["string"],
				},
			},
		},
	}

	ebnf.Rules["program"] = &Concatenation{
		T: programTransform,
		Patterns: []Pattern{
			&TerminalString{String: "PROGRAM"},
			ebnf.Rules["whitespace"],
			ebnf.Rules["identifier"],
			ebnf.Rules["whitespace"],
			&TerminalString{String: "BEGIN"},
			ebnf.Rules["whitespace"],
			&Repetition{
				Min: 0,
				Max: 0,
				Pattern: &Concatenation{
					Patterns: []Pattern{
						ebnf.Rules["assignment"],
						&TerminalString{String: ";"},
						ebnf.Rules["whitespace"],
					},
				},
			},
			&TerminalString{String: "END"},
		},
	}

	ebnf.RootRule = "program"

	result, err := ebnf.Match(reader)
	if err != nil {
		log.Fatalf("err %v\n", err)
	}

	if result.Match {
		program := result.Result.(*program)

		log.Printf("program name %v\n", program.Identifier)

		for _, assignment := range program.Assignments {
			log.Printf("assignment identifier: %v = %v\n", assignment.Identifier, assignment.Value)
		}
	} else {
		log.Printf("no match\n")
	}
}
