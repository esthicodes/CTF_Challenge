package main

import (
	"net"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type State struct {
	Guesses     int
	WordLen     int
	Conn        net.Conn
	GuessIndex  int
	LetterIndex int
	Letters     []*tview.Button
	Level       int
	Complete    bool
	Message     *tview.Button
	AlertChan   chan bool
	Candidates  map[rune]*tview.Button
}

func (state *State) CurrentLetters() []*tview.Button {
	start := state.GuessIndex * state.WordLen
	end := start + state.WordLen
	return state.Letters[start:end]
}

func (state *State) CurrentLetter() *tview.Button {
	return state.Letters[(state.GuessIndex*state.WordLen)+state.LetterIndex]
}

func (state *State) SetMessage(message string, fadeout bool) {
	state.Message.SetLabel("[::b]" + message)
	state.AlertChan <- fadeout
}

func (state *State) MessageAnimationHandler() {
	stopChan := make(chan struct{})
	go state.MessageFadeOut(stopChan, false)
	for {
		fadeout := <-state.AlertChan
		stopChan <- struct{}{}
		app.QueueUpdateDraw(func() {
			state.Message.SetBackgroundColor(colorWhite)
		})
		go state.MessageFadeOut(stopChan, fadeout)
	}
}

func (state *State) MessageFadeOut(stopChan chan struct{}, fadeout bool) {
	var now time.Time

	select {
	case <-stopChan:
		return
	case <-time.After(time.Second):
	}

	now = time.Now()
	if fadeout {
		var i int32
		for i = 255; i >= 0; i-- {
			select {
			case <-stopChan:
				return
			default:
			}

			now = now.Add(2 * time.Millisecond)
			<-time.After(time.Until(now))
			app.QueueUpdateDraw(func() {
				state.Message.SetBackgroundColor(tcell.NewRGBColor(i, i, i))
			})
		}
	}

	<-stopChan
}

func (state *State) UpdateIndicators(indicators []rune) {
	letters := state.CurrentLetters()
	for i, indicator := range indicators {
		state.LetterIndex = i

		color := colorBlack
		switch indicator {
		case '🟩':
			color = colorGreen
		case '🟨':
			color = colorYellow
		case '⬛':
			color = colorLightGray
		}

		state.CurrentLetter().SetBackgroundColor(color)
		state.CurrentLetter().SetBackgroundColorActivated(color)
		state.CurrentLetter().SetLabelColor(colorBlack)
		state.CurrentLetter().SetLabelColorActivated(colorBlack)

		label := []rune(letters[i].GetLabel())
		if len(label) == 6 {
			c, ok := state.Candidates[label[5]]
			if ok {
				prevColor := c.GetBackgroundColor()
				switch prevColor {
				case colorLightGray:
					if color == colorLightGray {
						color = colorGray
					}
					c.SetBackgroundColor(color)
				case colorYellow:
					if color == colorGreen {
						c.SetBackgroundColor(color)
					}
				}

				if color == colorYellow || color == colorGreen {
					c.SetLabelColor(colorBlack)
				}
			}
		}
	}
}
