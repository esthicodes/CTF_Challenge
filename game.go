package game

import (
	"errors"

	"pppordle/cert"

	"github.com/google/uuid"
)

type Result interface {
	*GuessResult | *InfoResult | *InitResult
}

type GuessValidator func(game *Game, guess []rune) error

type RequestType int

const (
	RequestInfo = iota
	RequestGuess
	RequestInit
)

type Game struct {
	Word       []rune
	Guesses    int
	Validator  GuessValidator
	Level      int
	Candidates []rune
	CompleteMessage string
}

type GuessResult struct {
	Error            string
	Indicators       []rune
	Complete         bool
	CompleteMessage  string
	RemainingGuesses int
	ClientCert       cert.PemCertPair
}

type InfoResult struct {
	Error      string
	Length     int
	Level      int
	Guesses    int
	Candidates []rune
}

type InitResult struct {
	SessionID  uuid.UUID
	LevelCount int
}

type Request struct {
	Type RequestType
	Data string
}

func (g *Game) ProcessGuess(guess []rune) *GuessResult {
	if len(guess) != len(g.Word) {
		return &GuessResult{
			Error: errors.New("Invalid Guess").Error(),
		}
	}

	err := g.Validator(g, guess)
	if err != nil {
		return &GuessResult{
			Error: err.Error(),
		}
	}

	count := 0
	runesLeft := make([]rune, len(g.Word))
	_ = copy(runesLeft, g.Word)
	var indicators []rune
	for i, guessRune := range guess {
		if guessRune == g.Word[i] {
			runesLeft = removeFromRunesLeft(runesLeft, guessRune)
			indicators = append(indicators, '🟩')
			count += 1
		} else {
			indicators = append(indicators, '⬛')
		}
	}

	for i, guessRune := range guess {
		if guessRune == g.Word[i] {
			continue
		} else if wordContainsRune(runesLeft, guessRune) > 0 {
			runesLeft = removeFromRunesLeft(runesLeft, guessRune)
			indicators[i] = '🟨'
		}
	}

	return &GuessResult{
		Error:      "",
		Indicators: indicators,
		Complete:   count == len(g.Word),
	}
}

func removeFromRunesLeft(runesLeft []rune, guessRune rune) []rune {
	for i, r := range runesLeft {
		if guessRune == r {
			return append(runesLeft[:i], runesLeft[i+1:]...)
		}
	}

	return runesLeft
}

func wordContainsRune(word []rune, guess rune) int {
	count := 0
	for _, r := range word {
		if guess == r {
			count++
		}
	}

	return count
}
