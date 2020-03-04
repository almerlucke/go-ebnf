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

// type PatternType int

// const (
// 	TerminalSymbolPattern PatternType = iota
// 	TerminalCharacterGroupPattern
// 	AlternationPattern
// 	ConcatenationPattern
// 	OptionalPattern
// 	RepetitionPattern
// 	ExceptionPattern
// )

// MatchResult contains the result of a match
type MatchResult struct {
	Match  bool
	Result interface{}
}

// Pattern to match
type Pattern interface {
	Match(r *Reader) (*MatchResult, error)
}

// TerminalString pattern
type TerminalString string

// TerminalCharacterGroup pattern, for instance whitespace group
type TerminalCharacterGroup func(r rune) bool

// Alternation pattern
type Alternation []Pattern

// Concatenation pattern
type Concatenation []Pattern

// Repetition pattern
type Repetition struct {
	Min     int
	Max     int
	Pattern Pattern
}

// Exception pattern
type Exception struct {
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

// Match a terminal string
func (s TerminalString) Match(r *Reader) (*MatchResult, error) {
	r.SavePos()

	result := &MatchResult{
		Match: false,
	}
	runes := []rune(s)

	for _, rn1 := range runes {
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

	r.PopPos()

	return result, nil
}

// Match a character from a group
func (g TerminalCharacterGroup) Match(r *Reader) (*MatchResult, error) {
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

	result.Match = g(rn)
	if result.Match {
		result.Result = r.String()
		r.PopPos()
	} else {
		r.RestorePos()
	}

	return result, nil
}

// Match alternation pattern, matches if one of the alternating patterns matches
func (a Alternation) Match(r *Reader) (*MatchResult, error) {
	for _, p := range a {
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
			return nil, err
		}

		if result.Match {
			r.PopPos()
			return result, nil
		}

		r.RestorePos()
	}

	return &MatchResult{Match: false}, nil
}

// Match concatenation pattern
func (c Concatenation) Match(r *Reader) (*MatchResult, error) {
	matches := []*MatchResult{}

	r.SavePos()

	for _, p := range c {
		result, err := p.Match(r)
		if err != nil {
			r.RestorePos()
			return nil, err
		}

		if !result.Match {
			r.RestorePos()
			return &MatchResult{Match: false}, nil
		}

		matches = append(matches, result)
	}

	r.PopPos()

	return &MatchResult{
		Match:  true,
		Result: matches,
	}, nil
}

// Match exception pattern
func (e *Exception) Match(r *Reader) (result *MatchResult, err error) {
	result, err = e.Except.Match(r)
	if err != nil {
		return
	}

	if result.Match {
		result.Match = false
		result.Result = nil
		return
	}

	result, err = e.MustMatch.Match(r)

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

	return &MatchResult{
		Match:  true,
		Result: matches,
	}, nil
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

func main() {
	reader := NewReader(strings.NewReader("PROGRAM DEMO1\nBEGIN\nA:=3;\nEND"))
	ebnf := NewEBNF()

	ebnf.Rules["whitespace"] = TerminalCharacterGroup(unicode.IsSpace)
	ebnf.Rules["visible_character"] = TerminalCharacterGroup(unicode.IsPrint)
	ebnf.Rules["digit"] = TerminalCharacterGroup(unicode.IsDigit)
	ebnf.Rules["alphabetic_character"] = Alternation{
		TerminalString("A"), TerminalString("B"), TerminalString("C"), TerminalString("D"), TerminalString("E"),
		TerminalString("F"), TerminalString("G"), TerminalString("H"), TerminalString("I"), TerminalString("J"),
		TerminalString("K"), TerminalString("L"), TerminalString("M"), TerminalString("N"), TerminalString("O"),
		TerminalString("P"), TerminalString("Q"), TerminalString("R"), TerminalString("S"), TerminalString("T"),
		TerminalString("U"), TerminalString("V"), TerminalString("W"), TerminalString("X"), TerminalString("Y"),
		TerminalString("Z"),
	}

	ebnf.Rules["identifier"] = Concatenation{
		ebnf.Rules["alphabetic_character"],
		&Repetition{Min: 0, Max: 0, Pattern: Alternation{ebnf.Rules["alphabetic_character"], ebnf.Rules["digit"]}},
	}

	ebnf.Rules["number"] = &Repetition{Min: 1, Max: 0, Pattern: ebnf.Rules["digit"]}

	ebnf.Rules["string"] = Concatenation{
		TerminalString("\""),
		&Exception{
			MustMatch: ebnf.Rules["visible_character"],
			Except:    TerminalString("\""),
		},
		TerminalString("\""),
	}

	ebnf.Rules["assignment"] = Concatenation{
		ebnf.Rules["identifier"],
		TerminalString(":="),
		Alternation{
			ebnf.Rules["number"],
			ebnf.Rules["identifier"],
			ebnf.Rules["string"],
		},
	}

	ebnf.Rules["program"] = Concatenation{
		TerminalString("PROGRAM"),
		ebnf.Rules["whitespace"],
		ebnf.Rules["identifier"],
		ebnf.Rules["whitespace"],
		TerminalString("BEGIN"),
		ebnf.Rules["whitespace"],
		&Repetition{
			Min: 0,
			Max: 0,
			Pattern: Concatenation{
				ebnf.Rules["assignment"],
				TerminalString(";"),
				ebnf.Rules["whitespace"],
			},
		},
		TerminalString("END"),
	}

	ebnf.RootRule = "program"

	result, err := ebnf.Match(reader)
	if err != nil {
		log.Fatalf("err %v\n", err)
	}

	if result.Match {
		log.Printf("matched %v\n", result.Result)
	} else {
		log.Printf("no match\n")
	}
}
