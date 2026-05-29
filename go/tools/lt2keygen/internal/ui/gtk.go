//go:build gtk

package ui

import (
	"os"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func runGTK(generate GenerateFunc) int {
	app := gtk.NewApplication("dev.flame.lsx.lt2keygen", gio.ApplicationFlagsNone)
	app.ConnectActivate(func() { activateGTK(app, generate) })
	return app.Run(os.Args)
}

func activateGTK(app *gtk.Application, generate GenerateFunc) {
	installGTKKeygenCSS()

	window := gtk.NewApplicationWindow(app)
	window.SetTitle(appTitle)
	window.SetDefaultSize(660, 460)

	nameEntry := gtk.NewEntry()
	nameEntry.SetPlaceholderText("Optional registration name")

	registrationName := readonlyGTKEntry()
	activationKey := readonlyGTKEntry()
	status := gtk.NewLabel("")
	status.SetWrap(true)
	status.SetXAlign(0)
	status.AddCSSClass("dim-label")

	generateButton := gtk.NewButtonWithLabel("Generate")
	copyNameButton := gtk.NewButtonWithLabel("Copy")
	copyKeyButton := gtk.NewButtonWithLabel("Copy")
	aboutButton := gtk.NewButtonWithLabel("About")
	exitButton := gtk.NewButtonWithLabel("Exit")

	copyNameButton.SetSensitive(false)
	copyKeyButton.SetSensitive(false)

	generateNow := func() {
		out, err := generate(strings.TrimSpace(nameEntry.Text()))
		if err != nil {
			status.SetText("Key generation failed: " + err.Error())
			return
		}
		registrationName.SetText(out.RegistrationName)
		activationKey.SetText(out.ActivationKey)
		status.SetText("Generated. Keep the registration name and activation key together.")
		copyNameButton.SetSensitive(true)
		copyKeyButton.SetSensitive(true)
	}

	nameEntry.ConnectActivate(generateNow)
	generateButton.ConnectClicked(generateNow)
	copyNameButton.ConnectClicked(func() { copyGTK(window, registrationName.Text()) })
	copyKeyButton.ConnectClicked(func() { copyGTK(window, activationKey.Text()) })
	aboutButton.ConnectClicked(func() { showGTKAbout(app, window) })
	exitButton.ConnectClicked(func() { app.Quit() })

	content := gtk.NewBox(gtk.OrientationVertical, 24)
	setGTKMargins(content, 34)

	header := gtk.NewBox(gtk.OrientationVertical, 4)
	title := gtk.NewLabel(appTitle)
	title.SetXAlign(0)
	title.AddCSSClass("title")
	subtitle := gtk.NewLabel("Generate a registration name and activation key.")
	subtitle.SetXAlign(0)
	subtitle.AddCSSClass("dim-label")
	header.Append(title)
	header.Append(subtitle)

	registration := gtk.NewBox(gtk.OrientationVertical, 8)
	registration.Append(sectionGTKLabel("Registration"))
	registrationRow := gtk.NewBox(gtk.OrientationHorizontal, 10)
	registrationRow.Append(nameEntry)
	registrationRow.Append(generateButton)
	registration.Append(registrationRow)

	outputs := gtk.NewBox(gtk.OrientationVertical, 12)
	outputs.Append(sectionGTKLabel("Activation Pair"))
	outputs.Append(outputGTKRow("Registration name", registrationName, copyNameButton))
	outputs.Append(outputGTKRow("Activation key", activationKey, copyKeyButton))

	footer := gtk.NewBox(gtk.OrientationHorizontal, 10)
	footer.Append(aboutButton)
	footer.Append(exitButton)

	content.Append(header)
	content.Append(registration)
	content.Append(outputs)
	content.Append(status)
	content.Append(footer)

	window.SetChild(content)
	window.Show()
}

func readonlyGTKEntry() *gtk.Entry {
	entry := gtk.NewEntry()
	entry.SetEditable(false)
	entry.AddCSSClass("monospace")
	return entry
}

func outputGTKRow(label string, entry *gtk.Entry, copyButton *gtk.Button) *gtk.Box {
	row := gtk.NewBox(gtk.OrientationVertical, 5)
	row.Append(captionGTKLabel(label))

	valueRow := gtk.NewBox(gtk.OrientationHorizontal, 10)
	valueRow.Append(entry)
	valueRow.Append(copyButton)
	row.Append(valueRow)
	return row
}

func sectionGTKLabel(text string) *gtk.Label {
	label := gtk.NewLabel(text)
	label.SetXAlign(0)
	label.AddCSSClass("section-label")
	return label
}

func captionGTKLabel(text string) *gtk.Label {
	label := gtk.NewLabel(text)
	label.SetXAlign(0)
	label.AddCSSClass("caption-label")
	return label
}

func copyGTK(window *gtk.ApplicationWindow, text string) {
	if text == "" {
		return
	}
	window.Clipboard().SetText(text)
}

func showGTKAbout(app *gtk.Application, parent *gtk.ApplicationWindow) {
	window := gtk.NewApplicationWindow(app)
	window.SetTitle("How It Works")
	window.SetDefaultSize(920, 680)
	window.SetTransientFor(&parent.Window)

	selectedTitle := gtk.NewLabel(aboutTopics[0].Title)
	selectedTitle.SetXAlign(0)
	selectedTitle.AddCSSClass("title")

	body := gtk.NewLabel(gtkAboutBody(aboutTopics[0].Body))
	body.SetXAlign(0)
	body.SetWrap(true)
	body.SetSelectable(true)

	sidebar := gtk.NewBox(gtk.OrientationVertical, 8)
	setGTKMargins(sidebar, 16)
	sidebar.SetSizeRequest(220, -1)
	sidebar.Append(sectionGTKLabel("How It Works"))

	for _, topic := range aboutTopics {
		topic := topic
		button := gtk.NewButtonWithLabel(topic.Title)
		button.ConnectClicked(func() {
			selectedTitle.SetText(topic.Title)
			body.SetText(gtkAboutBody(topic.Body))
		})
		sidebar.Append(button)
	}

	detail := gtk.NewBox(gtk.OrientationVertical, 16)
	setGTKMargins(detail, 28)
	detail.Append(selectedTitle)
	summary := gtk.NewLabel("Lemonade Tycoon 2: New York Edition uses a signed Armadillo ShortV3 key. This walkthrough explains each step with small examples.")
	summary.SetXAlign(0)
	summary.SetWrap(true)
	summary.AddCSSClass("dim-label")
	detail.Append(summary)
	detail.Append(body)

	scroller := gtk.NewScrolledWindow()
	scroller.SetChild(detail)

	root := gtk.NewBox(gtk.OrientationHorizontal, 0)
	root.Append(sidebar)
	root.Append(scroller)

	window.SetChild(root)
	window.Show()
}

func gtkAboutBody(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}

type gtkMarginWidget interface {
	SetMarginTop(int)
	SetMarginBottom(int)
	SetMarginStart(int)
	SetMarginEnd(int)
}

func setGTKMargins(widget gtkMarginWidget, margin int) {
	widget.SetMarginTop(margin)
	widget.SetMarginBottom(margin)
	widget.SetMarginStart(margin)
	widget.SetMarginEnd(margin)
}

func installGTKKeygenCSS() {
	provider := gtk.NewCSSProvider()
	provider.LoadFromData(`
		.title { font-size: 22px; font-weight: 700; }
		.section-label { font-weight: 700; }
		.caption-label { font-size: 12px; font-weight: 600; opacity: 0.72; }
		.dim-label { opacity: 0.72; }
		.monospace { font-family: monospace; }
	`)
	gtk.StyleContextAddProviderForDisplay(gdk.DisplayGetDefault(), provider, 600)
}
