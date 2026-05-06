package tui

import (
	"net/http"

	"github.com/charmbracelet/lipgloss"
)

func statusBadgeStyle(status int) lipgloss.Style {
	switch {
	case status >= http.StatusInternalServerError:
		return badgeStyle.Background(coralColor).Foreground(lipgloss.Color("231"))
	case status >= http.StatusBadRequest:
		return badgeStyle.Background(amberColor).Foreground(lipgloss.Color("16"))
	default:
		return badgeStyle.Background(mintColor).Foreground(lipgloss.Color("16"))
	}
}
