package main

import (
	"fmt"
	"log"
	"os"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"pppordle/check"
	"pppordle/game"
)

var (
	app   = tview.NewApplication()
	pages = tview.NewPages()

	colorBlack     = tcell.NewRGBColor(0x00, 0x00, 0x00)
	colorWhite     = tcell.NewRGBColor(0xff, 0xff, 0xff)
	colorGreen     = tcell.NewRGBColor(0x56, 0xb4, 0x4d)
	colorYellow    = tcell.NewRGBColor(0xff, 0xd9, 0x67)
	colorLightGray = tcell.NewRGBColor(0xa8, 0xa8, 0xa8)
	colorGray      = tcell.NewRGBColor(0x47, 0x47, 0x47)
	colorRed       = tcell.NewRGBColor(0xff, 0x56, 0x56)
)

func startUI() {
	pages.SetBackgroundColor(colorBlack)
	pages.AddPage("Level Selector", levelSelector(pages), true, true)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			frontPage, _ := pages.GetFrontPage()
			if frontPage == "Level Selector" {
				app.Stop()
				return nil
			} else {
				pages.RemovePage("Level")
				pages.SwitchToPage("Level Selector")
				return nil
			}
		}

		return event
	})

	err := app.SetRoot(pages, true).
		SetFocus(pages).
		Run()
	if err != nil {
		panic(err)
	}
}

func levelSelector(pages *tview.Pages) tview.Primitive {
	grid := tview.NewGrid().
		SetRows(0, 5, 30, 0).
		SetColumns(0, 80, 0).
		SetBorders(false).
		SetGap(1, 1)

	selector := tview.NewModal().
		SetText("Level Selector").
		AddButtons([]string{"1", "2", "3", "4"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			var level tview.Primitive
			loading, loadingText := loading(buttonIndex + 1)
			go func() {
				level = generateLevel(buttonIndex+1, loadingText)
				app.QueueUpdateDraw(func() {
					pages.AddAndSwitchToPage("Level", level, true)
				})
			}()
			pages.AddAndSwitchToPage("Loading", loading, true)
		}).
		SetBackgroundColor(colorGreen)

	grid.AddItem(title(), 1, 1, 1, 1, 0, 0, false)
	grid.AddItem(selector, 2, 1, 1, 1, 0, 0, true)

	return grid
}

func title() tview.Primitive {
	titleText := "PPPORDLE"
	grid := tview.NewGrid().
		SetRows(0).
		SetColumns(0, 0, 0, 0, 0, 0, 0, 0).
		SetBorders(false).
		SetGap(1, 1)

	for i, l := range titleText {
		b := tview.NewButton("[::b]" + string(l))
		b.SetLabelColor(colorBlack)
		switch i {
		case 0:
			b.SetBackgroundColor(colorGreen)
		case 1:
			b.SetBackgroundColor(colorYellow)
		case 2:
			b.SetBackgroundColor(colorLightGray)
		default:
			b.SetBackgroundColor(colorGray)
			b.SetLabelColor(colorWhite)
		}

		grid.AddItem(b, 0, i, 1, 1, 0, 0, false)
	}

	return grid
}

func loading(level int) (tview.Primitive, *tview.TextView) {
	modal := func(p tview.Primitive, width, height int) tview.Primitive {
		return tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(p, height, 1, false).
				AddItem(nil, 0, 1, false), width, 1, false).
			AddItem(nil, 0, 1, false)
	}

	tv := tview.NewTextView().SetChangedFunc(func() { app.Draw() })
	tv.SetBackgroundColor(colorGreen).
		SetTitle(fmt.Sprintf("[::b]Loading Level %d", level)).
		SetTitleColor(colorWhite).
		SetBorder(true)

	return modal(tv, 40, 5), tv
}

func generateLevel(level int, loadingText *tview.TextView) tview.Primitive {
	errorModal := tview.NewModal().
		AddButtons([]string{"Ok"}).
		SetBackgroundColor(colorRed).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			pages.RemovePage("Level")
			pages.SwitchToPage("Level Selector")
		})

	conn, err := startSession(level, loadingText)
	if err != nil {
		log.Println(err)
		errorModal.SetText(err.Error())
		return errorModal
	}

	infoResult, err := makeRequest[*game.InfoResult](conn, game.Request{Type: game.RequestInfo})
	if err != nil {
		log.Println(err)
		errorModal.SetText(err.Error())
		return errorModal
	}
	log.Printf("received game info result: %+v", infoResult)

	candidateMap, candidateButtons := buildCandidates(infoResult.Candidates)
	state := State{
		Guesses:     infoResult.Guesses,
		WordLen:     infoResult.Length,
		Conn:        conn,
		GuessIndex:  0,
		LetterIndex: 0,
		Level:       level,
		Candidates:  candidateMap,
	}

	scale := 50 / state.WordLen

	var guessRows = make([]int, state.Guesses+6)
	guessRows[0] = 1
	guessRows[1] = 3
	guessRows[2] = 0
	guessRows[3] = 3
	for i := 0; i < state.Guesses; i++ {
		guessRows[i+4] = scale / 2
	}
	guessRows[len(guessRows)-2] = 0
	guessRows[len(guessRows)-1] = -5

	var guessCols = make([]int, state.WordLen+4)
	guessCols[0] = 0
	guessCols[1] = 0
	for i := 0; i < state.WordLen; i++ {
		guessCols[i+2] = scale
	}
	guessCols[len(guessCols)-2] = 0
	guessCols[len(guessCols)-1] = 0

	state.Message = messageBox()
	state.AlertChan = make(chan bool)
	go state.MessageAnimationHandler()

	grid := tview.NewGrid().
		SetRows(guessRows...).
		SetColumns(guessCols...).
		SetBorders(false).
		SetGap(1, 1).
		AddItem(title(), 1, 2, 1, len(guessCols)-4, 0, 0, false).
		AddItem(candidateButtons, len(guessRows)-1, 1, 1, len(guessCols)-2, 0, 0, false).
		AddItem(state.Message, 3, 2, 1, len(guessCols)-4, 0, 0, false)

	for i := 0; i < state.Guesses; i++ {
		for j := 0; j < state.WordLen; j++ {
			inputLetter := tview.NewButton("")
			inputLetter.SetBackgroundColor(colorGray)
			inputLetter.SetBackgroundColorActivated(colorGray)
			inputLetter.SetLabelColorActivated(colorWhite)
			state.Letters = append(state.Letters, inputLetter)

			grid.AddItem(inputLetter, i+4, j+2, 1, 1, 0, 0, false)
		}
	}

	app.SetFocus(state.Letters[state.LetterIndex])

	grid.SetInputCapture(gameboardInputHandler(&state))

	return grid
}

func messageBox() *tview.Button {
	b := tview.NewButton("").SetLabelColor(colorBlack)
	b.SetBackgroundColor(colorBlack)
	return b
}

func buildCandidates(candidates []rune) (map[rune]*tview.Button, tview.Primitive) {
	candidateRange := 250 - 20
	rowRange := 28 - 8
	rowSize := ((len(candidates) * rowRange) / candidateRange) + 8
	log.Println("Row Size:", rowSize)

	grid := tview.NewGrid().
		SetBorders(false).
		SetGap(1, 1)

	buttons := make(map[rune]*tview.Button)
	col := -1
	row := -1
	for i, c := range candidates {
		if i%rowSize == 0 {
			col = 0
			row += 1
		}

		newButton := tview.NewButton("[::b]" + string(c))
		newButton.SetLabelColor(colorWhite)
		newButton.SetBackgroundColor(colorLightGray)
		buttons[c] = newButton
		grid.AddItem(newButton, row, col, 1, 1, 0, 0, false)
		col += 1
	}

	return buttons, grid
}

func gameboardInputHandler(state *State) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		if state.Complete {
			pages.RemovePage("Level")
			pages.SwitchToPage("Level Selector")
			return nil
		}

		if event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
			removeLetter(state)
			return nil
		}

		if event.Key() == tcell.KeyCR || event.Key() == tcell.KeyEnter {
			sendGuess(state)
			return nil
		}

		if len(string(event.Rune())) > 0 && !unicode.IsSpace(event.Rune()) {
			addLetter(unicode.ToUpper(event.Rune()), state)
			return nil
		}

		return event
	}
}

func addLetter(letter rune, state *State) {
	state.CurrentLetter().SetLabel("[::b]" + string(letter))

	if state.LetterIndex == state.WordLen-1 {
		return
	}
	state.LetterIndex = (state.LetterIndex + 1) % state.WordLen

	app.SetFocus(state.CurrentLetter())
}

func removeLetter(state *State) {
	if state.LetterIndex == 0 {
		return
	}

	if state.LetterIndex == state.WordLen-1 && len(state.CurrentLetter().GetLabel()) != 0 {
		state.CurrentLetter().SetLabel("")
		return
	}

	state.LetterIndex = (state.LetterIndex - 1) % state.WordLen

	state.CurrentLetter().SetLabel("")
	app.SetFocus(state.CurrentLetter())
}

func sendGuess(state *State) {
	guess := ""
	for _, l := range state.CurrentLetters() {
		styled := []rune(l.GetLabel())
		if len(styled) != 6 {
			state.SetMessage("Not enough letters", true)
			return
		}
		guess += string(styled[len(styled)-1:])
	}

	guessResult, err := makeRequest[*game.GuessResult](state.Conn, game.Request{
		Type: game.RequestGuess,
		Data: guess,
	})
	if err != nil {
		log.Println(err)
		state.SetMessage(err.Error(), true)
		return
	}

	if len(guessResult.Error) != 0 {
		log.Println(guessResult.Error)
		state.SetMessage(guessResult.Error, true)
		return
	}

	state.UpdateIndicators(guessResult.Indicators)

	if guessResult.Complete {
		state.Complete = true
		state.SetMessage(guessResult.CompleteMessage, false)
		log.Printf("level %d completed", state.Level)

		if state.Level < 4 {
			next := state.Level + 1
			err = os.WriteFile(fmt.Sprintf("certs/level%d.pem", next), guessResult.ClientCert.Cert, 0600)
			check.Fatal(fmt.Sprintf("failed to write level %d client certificate", next), err)
			err = os.WriteFile(fmt.Sprintf("certs/level%d.key", next), guessResult.ClientCert.Key, 0600)
			check.Fatal(fmt.Sprintf("failed to write level %d client key", next), err)
		}

		return
	}

	state.GuessIndex += 1
	state.LetterIndex = 0

	if state.GuessIndex >= state.Guesses {
		state.Complete = true
		state.SetMessage("Better luck next time", false)
		state.GuessIndex = state.Guesses - 1
		return
	}

	app.SetFocus(state.CurrentLetter())
	log.Printf("received guess result: %+v", guessResult)
}
