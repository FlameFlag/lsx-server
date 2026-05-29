//go:build windows && windigo

package ui

import (
	"fmt"
	"runtime"
	"strings"
	"unicode/utf16"

	"github.com/rodrigocfd/windigo/co"
	windigoui "github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"
)

type windowsView struct {
	wnd              *windigoui.Main
	nameInput        *windigoui.Edit
	registrationName *windigoui.Edit
	activationKey    *windigoui.Edit
	status           *windigoui.Static
	generateButton   *windigoui.Button
	copyNameButton   *windigoui.Button
	copyKeyButton    *windigoui.Button
	aboutButton      *windigoui.Button
	exitButton       *windigoui.Button
	generate         GenerateFunc
}

func runWindows(generate GenerateFunc) int {
	runtime.LockOSThread()

	wnd := windigoui.NewMain(
		windigoui.OptsMain().
			Title(appTitle).
			Size(windigoui.Dpi(660, 430)),
	)

	windowsLabel(wnd, appTitle, 28, 24, 580, 24)
	windowsLabel(wnd, "Generate a registration name and activation key.", 28, 54, 580, 20)

	windowsLabel(wnd, "Registration", 28, 102, 160, 20)
	nameInput := windigoui.NewEdit(wnd, windigoui.OptsEdit().Position(windigoui.Dpi(28, 128)).Width(windigoui.DpiX(470)))
	generateButton := windigoui.NewButton(wnd, windigoui.OptsButton().Text("&Generate").Position(windigoui.Dpi(510, 127)).Width(windigoui.DpiX(110)))

	windowsLabel(wnd, "Activation Pair", 28, 178, 180, 20)
	windowsLabel(wnd, "Registration name", 28, 212, 180, 18)
	registrationName := readonlyWindowsEdit(wnd, 28, 234, 470, 23, "", false)
	copyNameButton := windigoui.NewButton(wnd, windigoui.OptsButton().Text("Copy").Position(windigoui.Dpi(510, 233)).Width(windigoui.DpiX(80)))
	windowsLabel(wnd, "Activation key", 28, 276, 180, 18)
	activationKey := readonlyWindowsEdit(wnd, 28, 298, 470, 23, "", false)
	copyKeyButton := windigoui.NewButton(wnd, windigoui.OptsButton().Text("Copy").Position(windigoui.Dpi(510, 297)).Width(windigoui.DpiX(80)))

	status := windigoui.NewStatic(wnd, windigoui.OptsStatic().Text("").Position(windigoui.Dpi(28, 344)).Size(windigoui.Dpi(580, 22)))
	aboutButton := windigoui.NewButton(wnd, windigoui.OptsButton().Text("About").Position(windigoui.Dpi(28, 376)).Width(windigoui.DpiX(90)))
	exitButton := windigoui.NewButton(wnd, windigoui.OptsButton().Text("Exit").Position(windigoui.Dpi(128, 376)).Width(windigoui.DpiX(90)))

	view := &windowsView{
		wnd:              wnd,
		nameInput:        nameInput,
		registrationName: registrationName,
		activationKey:    activationKey,
		status:           status,
		generateButton:   generateButton,
		copyNameButton:   copyNameButton,
		copyKeyButton:    copyKeyButton,
		aboutButton:      aboutButton,
		exitButton:       exitButton,
		generate:         generate,
	}
	view.events()
	return wnd.RunAsMain()
}

func (view *windowsView) events() {
	view.generateButton.On().BnClicked(func() {
		out, err := view.generate(strings.TrimSpace(view.nameInput.Text()))
		if err != nil {
			msg := fmt.Sprintf("Key generation failed: %v", err)
			view.status.SetTextAndResize(msg)
			view.wnd.Hwnd().MessageBox(msg, appTitle, co.MB_ICONERROR)
			return
		}
		view.registrationName.SetText(out.RegistrationName)
		view.activationKey.SetText(out.ActivationKey)
		view.status.SetTextAndResize("Generated. Keep the registration name and activation key together.")
	})

	view.copyNameButton.On().BnClicked(func() {
		view.copyText(view.registrationName.Text(), "Registration name")
	})
	view.copyKeyButton.On().BnClicked(func() {
		view.copyText(view.activationKey.Text(), "Activation key")
	})
	view.aboutButton.On().BnClicked(func() { showWindowsAbout(view.wnd) })
	view.exitButton.On().BnClicked(func() { _ = view.wnd.Hwnd().DestroyWindow() })
}

func (view *windowsView) copyText(text, label string) {
	if text == "" {
		return
	}
	if err := setWindowsClipboardText(view.wnd.Hwnd(), text); err != nil {
		view.wnd.Hwnd().MessageBox(fmt.Sprintf("Could not copy %s: %v", label, err), appTitle, co.MB_ICONERROR)
		return
	}
	view.status.SetTextAndResize(label + " copied.")
}

func showWindowsAbout(parent *windigoui.Main) {
	modal := windigoui.NewModal(
		parent,
		windigoui.OptsModal().
			Title("How It Works").
			Size(windigoui.Dpi(920, 650)),
	)

	windowsLabel(modal, "How It Works", 18, 18, 180, 22)
	selectedTitle := windowsLabel(modal, aboutTopics[0].Title, 232, 18, 620, 24)
	windowsLabel(modal, "Lemonade Tycoon 2: New York Edition uses a signed Armadillo ShortV3 key. Select a topic to walk through the steps.", 232, 48, 620, 38)
	body := readonlyWindowsEdit(modal, 232, 96, 650, 480, aboutTopics[0].Body, true)

	for i, topic := range aboutTopics {
		topic := topic
		button := windigoui.NewButton(modal, windigoui.OptsButton().Text(topic.Title).Position(windigoui.Dpi(18, 54+i*36)).Width(windigoui.DpiX(180)))
		button.On().BnClicked(func() {
			selectedTitle.SetTextAndResize(topic.Title)
			body.SetText(topic.Body)
		})
	}

	closeButton := windigoui.NewButton(modal, windigoui.OptsButton().Text("Close").Position(windigoui.Dpi(802, 596)).Width(windigoui.DpiX(80)))
	closeButton.On().BnClicked(func() { _ = modal.Hwnd().DestroyWindow() })
	modal.ShowModal()
}

func windowsLabel(parent windigoui.Parent, text string, x, y, w, h int) *windigoui.Static {
	return windigoui.NewStatic(parent, windigoui.OptsStatic().Text(text).Position(windigoui.Dpi(x, y)).Size(windigoui.Dpi(w, h)))
}

func readonlyWindowsEdit(parent windigoui.Parent, x, y, w, h int, text string, multiline bool) *windigoui.Edit {
	style := co.ES_AUTOHSCROLL | co.ES_NOHIDESEL | co.ES_READONLY
	if multiline {
		style = co.ES_MULTILINE | co.ES_AUTOVSCROLL | co.ES_READONLY | co.ES_NOHIDESEL
	}
	return windigoui.NewEdit(parent, windigoui.OptsEdit().Text(text).Position(windigoui.Dpi(x, y)).Width(windigoui.DpiX(w)).Height(windigoui.DpiY(h)).CtrlStyle(style))
}

func setWindowsClipboardText(owner win.HWND, text string) error {
	clip, err := win.OpenClipboard(owner)
	if err != nil {
		return err
	}
	defer clip.CloseClipboard()

	if err := clip.EmptyClipboard(); err != nil {
		return err
	}
	return clip.SetClipboardData(co.CF_UNICODETEXT, utf16Bytes(text))
}

func utf16Bytes(text string) []byte {
	encoded := utf16.Encode([]rune(text + "\x00"))
	out := make([]byte, 0, len(encoded)*2)
	for _, value := range encoded {
		out = append(out, byte(value), byte(value>>8))
	}
	return out
}
