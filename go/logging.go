package main

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

func configureLogger() {
	styles := log.DefaultStyles()
	styles.Timestamp = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	styles.Prefix = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	styles.Message = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	styles.Key = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
	styles.Value = lipgloss.NewStyle().Foreground(lipgloss.Color("229"))
	styles.Separator = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styles.Levels[log.DebugLevel] = lipgloss.NewStyle().
		SetString("DBG").
		Bold(true).
		Foreground(lipgloss.Color("63"))
	styles.Levels[log.InfoLevel] = lipgloss.NewStyle().
		SetString("RUN").
		Bold(true).
		Foreground(lipgloss.Color("86"))
	styles.Levels[log.WarnLevel] = lipgloss.NewStyle().
		SetString("WRN").
		Bold(true).
		Foreground(lipgloss.Color("214"))
	styles.Levels[log.ErrorLevel] = lipgloss.NewStyle().
		SetString("ERR").
		Bold(true).
		Foreground(lipgloss.Color("204"))

	log.SetPrefix("lsx")
	log.SetStyles(styles)
}
