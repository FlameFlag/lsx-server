package tui

import "github.com/charmbracelet/lipgloss"

var (
	panelColor  = lipgloss.Color("65")
	mutedColor  = lipgloss.Color("245")
	textColor   = lipgloss.Color("252")
	lemonColor  = lipgloss.Color("220")
	mintColor   = lipgloss.Color("84")
	skyColor    = lipgloss.Color("117")
	coralColor  = lipgloss.Color("203")
	violetColor = lipgloss.Color("141")
	amberColor  = lipgloss.Color("214")

	screenStyle = lipgloss.NewStyle().
			Align(lipgloss.Center).
			AlignVertical(lipgloss.Center)

	headerStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(lemonColor).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lemonColor)

	titleMetaStyle = lipgloss.NewStyle().
			Foreground(skyColor)

	badgeStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1)

	modeBadgeStyle = badgeStyle.
			Foreground(lipgloss.Color("16")).
			Background(violetColor)

	healthBadgeStyle = badgeStyle.
				Foreground(lipgloss.Color("16")).
				Background(mintColor)

	statusStripStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Padding(0, 1).
				MarginTop(1)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(panelColor).
			Padding(0, 1).
			MarginTop(1)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder(), false, false, false, true).
			BorderForeground(panelColor).
			Padding(0, 1).
			MarginTop(1)

	sectionTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(skyColor)
	cardLabelStyle    = lipgloss.NewStyle().Foreground(mutedColor).Faint(true)
	fieldStyle        = lipgloss.NewStyle().Foreground(mintColor).Bold(true)
	valueStyle        = lipgloss.NewStyle().Foreground(textColor)
	previewKeyStyle   = lipgloss.NewStyle().Foreground(violetColor)
	previewValueStyle = lipgloss.NewStyle().Foreground(textColor)
	emptyStateStyle   = lipgloss.NewStyle().Foreground(mutedColor).Italic(true)
	subtleStyle       = lipgloss.NewStyle().Foreground(mutedColor).MarginTop(1)
)
