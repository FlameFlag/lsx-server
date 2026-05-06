package tui

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"lt2_reverse/lsx_server_go/internal/eventpath"
)

const (
	columnSeparator        = "│"
	defaultTableHeight     = 10
	defaultTableWidth      = 100
	maxLiveEntries         = 500
	maxPathPreviewWidth    = 48
	maxSampleCount         = 8
	minActivityColumnWidth = 9
	minMessageColumnWidth  = 12
	minPanelWidth          = 20
	minPathColumnWidth     = 12
	minRemoteColumnWidth   = 10
	minTableHeight         = 7
	overflowMarker         = "..."
	rawColumnCount         = 5
	tableCellPadding       = 16
	tableSeparatorCount    = 3
	tableVerticalReserve   = 22
	timeFormat             = "15:04:05"
)

func metaItem(label string, value string) string {
	return fieldStyle.Render(strings.ToUpper(label)) + " " + valueStyle.Render(value)
}

func panelHeading(title string, right string, width int) string {
	left := sectionTitleStyle.Render(title)
	if right == "" {
		return left
	}
	if lipgloss.Width(left)+lipgloss.Width(right)+1 > width {
		right = truncate(right, max(1, width-lipgloss.Width(left)-1))
	}
	return spread(left, right, width)
}

func spread(left string, right string, width int) string {
	space := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", space) + right
}

func fitLine(value string, width int) string {
	if lipgloss.Width(value) <= width {
		return value
	}
	return truncate(value, width)
}

func statusBadge(status int) string {
	return statusBadgeStyle(status).Render(statusText(status))
}

func statusText(status int) string {
	if status == 0 {
		return "-"
	}
	return strconv.Itoa(status)
}

func panelBodyWidth(total int) int {
	return max(1, total-2)
}

func panelTextWidth(total int) int {
	return max(1, total-4)
}

func truncate(value string, width int) string {
	if lipgloss.Width(value) <= width {
		return value
	}
	return ansi.Truncate(value, max(0, width), overflowMarker)
}

func pathValues(path string) url.Values {
	return eventpath.Values(path)
}

func routeOnly(path string) string {
	return eventpath.Route(path)
}

func localAccessURL(bound string) string {
	host, port, ok := splitBoundAddress(bound)
	if !ok {
		return "http://" + strings.TrimSuffix(bound, "/") + "/"
	}
	if isWildcardHost(host) {
		host = "localhost"
	}
	return "http://" + formatHostPort(host, port) + "/"
}

func localAccessURLForPath(bound string, endpoint string) string {
	endpoint = "/" + strings.TrimLeft(endpoint, "/")
	if endpoint == "/" {
		return localAccessURL(bound)
	}
	return strings.TrimRight(localAccessURL(bound), "/") + endpoint
}

func bindDescription(bound string) string {
	host, port, ok := splitBoundAddress(bound)
	if !ok {
		return bound
	}
	switch {
	case isWildcardHost(host):
		return "local + LAN, port " + port
	case isLoopbackHost(host):
		return "this machine only, port " + port
	default:
		return formatHostPort(host, port)
	}
}

func splitBoundAddress(bound string) (string, string, bool) {
	host, port, err := net.SplitHostPort(bound)
	if err == nil {
		return strings.Trim(host, "[]"), port, true
	}
	if strings.HasPrefix(bound, ":") && len(bound) > 1 {
		return "", strings.TrimPrefix(bound, ":"), true
	}
	return "", "", false
}

func formatHostPort(host string, port string) string {
	return net.JoinHostPort(host, port)
}

func isWildcardHost(host string) bool {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	return host == "" || host == "::" || host == "0.0.0.0"
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	return host == "localhost" || strings.HasPrefix(host, "127.") || host == "::1"
}
