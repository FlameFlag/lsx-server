package discord

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"lt2_reverse/lsx_server_go/internal/eventpath"
	"lt2_reverse/lsx_server_go/internal/lsx"
	"lt2_reverse/lsx_server_go/internal/lsxvalue"
	"lt2_reverse/lsx_server_go/internal/strutil"
)

type renderedEvent struct {
	title       string
	description string
	fields      []field
}

type eventLayout struct {
	title       textTemplate
	description textTemplate
	fields      []fieldSpec
}

type textTemplate func(eventView) string
type valueTemplate func(eventView) string

type fieldSpec struct {
	name   string
	inline bool
	value  valueTemplate
}

type lineSpec struct {
	label string
	value valueTemplate
}

var eventLayouts = map[string]eventLayout{
	"sync": {
		title:       prefixQueryTitle("Score uploaded", "companyname", "Unnamed company"),
		description: syncDescription,
		fields:      scoreEventFields,
	},
	"sync_rejected": {
		title:       prefixQueryTitle("Score rejected", "companyname", "Unnamed company"),
		description: syncDescription,
		fields:      scoreEventFields,
	},
	"sync_error": {
		title:       staticTitle("Score upload error"),
		description: syncDescription,
		fields:      scoreEventFields,
	},
	"account": {
		title:       prefixQueryTitle("Account created", "username", "unknown user"),
		description: accountDescription,
		fields: []fieldSpec{
			requestField,
			serverFieldSpec(true),
		},
	},
	"account_error": {
		title:       staticTitle("Account request error"),
		description: accountDescription,
		fields: []fieldSpec{
			requestField,
			serverFieldSpec(true),
		},
	},
}

var scoreEventFields = []fieldSpec{
	{name: "Run", inline: false, value: lines(runLines)},
	{name: "Financials", inline: true, value: lines(financialLines)},
	{name: "Operations", inline: true, value: lines(operationLines)},
	serverFieldSpec(true),
}

var requestField = fieldSpec{name: "Request", inline: true, value: lines(requestLines)}

var runLines = []lineSpec{
	{label: "CEO", value: query("ceoname", valuePlain)},
	{label: "User", value: query("username", valuePlain)},
	{label: "Mode", value: modeGoalText},
	{label: "Lifespan", value: query("lifespan", valuePlain)},
	{label: "Received", value: relativeTime},
}

var financialLines = []lineSpec{
	{label: "Market", value: marketValue},
	{label: "Revenue", value: query("revenues", valueMoney)},
	{label: "Retained", value: query("retainedearnings", valueMoney)},
}

var operationLines = []lineSpec{
	{label: "Cash", value: query("cashassets", valueMoney)},
	{label: "Stock", value: query("stockassets", valueMoney)},
	{label: "Stands", value: query("stands", valuePlain)},
	{label: "Cups", value: query("cupssold", valueWholeNumber)},
}

var requestLines = []lineSpec{
	{label: "Username", value: query("username", valuePlain)},
	{label: "Received", value: relativeTime},
}

type eventView struct {
	event  lsx.Event
	values url.Values
}

func render(ev lsx.Event) renderedEvent {
	view := eventView{event: ev, values: eventpath.Values(ev.Path)}
	layout, ok := eventLayouts[ev.Kind]
	if !ok {
		layout = eventLayout{
			title:       func(view eventView) string { return "LSX " + view.event.Kind },
			description: func(view eventView) string { return view.event.Message },
			fields:      []fieldSpec{serverFieldSpec(false)},
		}
	}
	return renderedEvent{
		title:       layout.title(view),
		description: layout.description(view),
		fields:      compactFields(renderFields(view, layout.fields)),
	}
}

func renderFields(view eventView, specs []fieldSpec) []field {
	fields := make([]field, 0, len(specs))
	for _, spec := range specs {
		fields = append(fields, field{
			Name:   spec.name,
			Value:  spec.value(view),
			Inline: spec.inline,
		})
	}
	return fields
}

func staticTitle(title string) textTemplate {
	return func(eventView) string {
		return title
	}
}

func prefixQueryTitle(prefix, key, fallback string) textTemplate {
	return func(view eventView) string {
		return prefix + " · " + strutil.FirstNonEmpty(view.values.Get(key), fallback)
	}
}

func syncDescription(view eventView) string {
	company := strutil.FirstNonEmpty(view.values.Get("companyname"), "Unnamed company")
	ceo := strutil.FirstNonEmpty(view.values.Get("ceoname"), "unknown CEO")
	parts := []string{"**" + company + "** led by **" + ceo + "**"}
	if user := view.values.Get("username"); user != "" {
		parts = append(parts, "Submitted by **"+user+"** "+relativeTime(view))
	}
	parts = append(parts, checksumLine(view.event.Message))
	return strings.Join(parts, "\n")
}

func accountDescription(view eventView) string {
	return "Account request for **" + safeValue(view.values.Get("username")) + "** received " + relativeTime(view)
}

func serverFieldSpec(inline bool) fieldSpec {
	return fieldSpec{name: "Server", inline: inline, value: lines([]lineSpec{
		{label: "Status", value: statusValue},
		{label: "Kind", value: kindValue},
		{label: "Route", value: routeValue},
	})}
}

func lines(specs []lineSpec) valueTemplate {
	return func(view eventView) string {
		out := make([]string, 0, len(specs))
		for _, spec := range specs {
			out = append(out, spec.label+": "+spec.value(view))
		}
		return strings.Join(out, "\n")
	}
}

func query(key string, format func(string) string) valueTemplate {
	return func(view eventView) string {
		return bold(format(view.values.Get(key)))
	}
}

func statusValue(view eventView) string {
	return bold(strconv.Itoa(view.event.Status))
}

func kindValue(view eventView) string {
	return "`" + view.event.Kind + "`"
}

func routeValue(view eventView) string {
	return "`" + eventpath.Route(view.event.Path) + "`"
}

func modeGoalText(view eventView) string {
	mode := valuePlain(view.values.Get("gamemode"))
	goal := valuePlain(view.values.Get("gamegoal"))
	return bold(mode) + " / " + bold(goal)
}

func marketValue(view eventView) string {
	total := lsxvalue.ParseInt(view.values.Get("cashassets")) +
		lsxvalue.ParseInt(view.values.Get("stockassets")) +
		lsxvalue.ParseInt(view.values.Get("standsassets")) +
		lsxvalue.ParseInt(view.values.Get("upgradesassets"))
	return bold(lsxvalue.FormatCents(total))
}

func compactFields(fields []field) []field {
	out := make([]field, 0, len(fields))
	for _, field := range fields {
		if field.Value == "" || field.Value == "-" {
			continue
		}
		if len(field.Value) > 1024 {
			field.Value = field.Value[:1021] + "..."
		}
		out = append(out, field)
		if len(out) == 25 {
			break
		}
	}
	return out
}

func colorFor(ev lsx.Event) int {
	for _, rule := range colorRules {
		if rule.match(ev) {
			return rule.color
		}
	}
	return colorSuccess
}

type colorRule struct {
	color int
	match func(lsx.Event) bool
}

const (
	colorSuccess = 0x57f287
	colorWarning = 0xfaa61a
	colorError   = 0xed4245
)

var colorRules = []colorRule{
	{color: colorError, match: func(ev lsx.Event) bool {
		return ev.Status >= http.StatusInternalServerError
	}},
	{color: colorWarning, match: func(ev lsx.Event) bool {
		return ev.Status >= http.StatusBadRequest ||
			strings.Contains(ev.Kind, "rejected") ||
			strings.Contains(ev.Kind, "error")
	}},
}

func footerText(ev lsx.Event) string {
	parts := []string{"LT2 LSX server", ev.Kind}
	if ev.RemoteAddr != "" {
		parts = append(parts, ev.RemoteAddr)
	}
	return strings.Join(parts, " · ")
}

func checksumLine(message string) string {
	for _, rule := range checksumRules {
		if strings.Contains(message, rule.contains) {
			return rule.line
		}
	}
	return message
}

var checksumRules = []struct {
	contains string
	line     string
}{
	{contains: "checksum mismatch", line: "**Checksum:** mismatch"},
	{contains: "checksum ok", line: "**Checksum:** ok"},
	{contains: "checksum missing", line: "**Checksum:** missing"},
}
