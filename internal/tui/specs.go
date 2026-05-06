package tui

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"lt2_reverse/lsx_server_go/internal/lsx"
)

type queryField struct {
	Key   string
	Label string
}

type displayField struct {
	Key   string
	Label string
}

type eventSpec struct {
	Display []displayField
	Samples []string
	Preview []queryField
}

var eventSpecs = map[string]eventSpec{
	"sync":          syncEventSpec,
	"sync_rejected": syncEventSpec,
	"sync_error":    syncEventSpec,
	"detail": {
		Display: []displayField{{Key: "d1", Label: "company"}},
		Samples: []string{"d1"},
		Preview: []queryField{
			{Key: "d1", Label: "Company"},
			{Key: "d2", Label: "CEO"},
			{Key: "d3", Label: "Mode"},
			{Key: "d4", Label: "Goal"},
			{Key: "d5", Label: "Rank"},
			{Key: "d6", Label: "Date"},
			{Key: "d7", Label: "Title"},
			{Key: "d8", Label: "Lifespan"},
			{Key: "d9", Label: "Goal code"},
			{Key: "d10", Label: "Retained raw"},
			{Key: "d11", Label: "Market cap"},
			{Key: "d12", Label: "Revenues"},
			{Key: "d13", Label: "Retained"},
			{Key: "d14", Label: "Stands"},
			{Key: "d15", Label: "Cash"},
			{Key: "d16", Label: "Stock"},
			{Key: "d17", Label: "Stand assets"},
			{Key: "d18", Label: "Upgrade assets"},
		},
	},
	"account": {
		Display: []displayField{{Key: "username", Label: "username"}},
		Samples: []string{"username"},
		Preview: []queryField{{Key: "username", Label: "Username"}},
	},
	"account_error": {
		Display: []displayField{{Key: "username", Label: "username"}},
		Samples: []string{"username"},
		Preview: []queryField{{Key: "username", Label: "Username"}},
	},
	"leaderboard": {
		Samples: []string{"username"},
		Preview: []queryField{{Key: "username", Label: "Username filter"}},
	},
}

var syncEventSpec = eventSpec{
	Display: []displayField{
		{Key: "companyname", Label: "company"},
		{Key: "username", Label: "username"},
	},
	Samples: []string{"companyname", "username"},
	Preview: []queryField{
		{Key: "companyname", Label: "Company"},
		{Key: "ceoname", Label: "CEO"},
		{Key: "username", Label: "User"},
		{Key: "gamemode", Label: "Mode"},
		{Key: "gamegoal", Label: "Goal"},
		{Key: "gamestartingdate", Label: "Start date"},
		{Key: "lifespan", Label: "Lifespan"},
		{Key: "stands", Label: "Stands"},
		{Key: "cupssold", Label: "Cups sold"},
		{Key: "cashassets", Label: "Cash"},
		{Key: "stockassets", Label: "Stock"},
		{Key: "standsassets", Label: "Stand assets"},
		{Key: "upgradesassets", Label: "Upgrade assets"},
		{Key: "retainedearnings", Label: "Retained"},
		{Key: "revenues", Label: "Revenues"},
		{Key: "checksumclient", Label: "Checksum"},
	},
}

type statSpec struct {
	Label string
	Color lipgloss.Color
	Value func(model) int
}

var statSpecs = []statSpec{
	{Label: "sync", Color: lipgloss.Color("42"), Value: func(m model) int { return m.counts["sync"] }},
	{Label: "accounts", Color: lipgloss.Color("39"), Value: func(m model) int { return m.counts["account"] }},
	{Label: "pages", Color: lipgloss.Color("111"), Value: func(m model) int { return m.counts["leaderboard"] }},
	{Label: "details", Color: lipgloss.Color("147"), Value: func(m model) int { return m.counts["detail"] }},
	{Label: "history", Color: lipgloss.Color("204"), Value: func(m model) int { return len(m.history) }},
}

type statusRule struct {
	Match  func(lsx.Event) bool
	Prefix string
}

var statusRules = []statusRule{
	{Match: func(ev lsx.Event) bool { return ev.Status >= http.StatusInternalServerError }, Prefix: "ERR "},
	{Match: func(ev lsx.Event) bool {
		return ev.Status >= http.StatusBadRequest || strings.Contains(ev.Kind, "rejected")
	}, Prefix: "WARN "},
	{Match: func(ev lsx.Event) bool { return ev.Kind == "sync" || ev.Kind == "account" }, Prefix: "OK "},
}

func displayPath(ev lsx.Event) string {
	values := pathValues(ev.Path)
	route := routeOnly(ev.Path)
	spec, ok := eventSpecs[ev.Kind]
	if !ok {
		return route
	}
	for _, field := range spec.Display {
		if value := values.Get(field.Key); value != "" {
			return route + " " + field.Label + "=" + value
		}
	}
	return route
}

func activitySamples(kind string, path string) []string {
	spec, ok := eventSpecs[kind]
	if !ok {
		return nil
	}
	values := pathValues(path)
	for _, key := range spec.Samples {
		if sample := strings.TrimSpace(values.Get(key)); sample != "" {
			return []string{sample}
		}
	}
	return nil
}

func previewFields(entry eventEntry) []previewField {
	values := pathValues(entry.path)
	fields := []previewField{{label: "Route", value: routeOnly(entry.path)}}
	if spec, ok := eventSpecs[entry.kind]; ok {
		fields = append(fields, previewFieldsFromSpec(values, spec.Preview)...)
	}
	fields = append(fields, previewField{label: "Message", value: entry.message})
	return compactPreviewFields(fields)
}

func previewFieldsFromSpec(values url.Values, fields []queryField) []previewField {
	out := make([]previewField, 0, len(fields))
	for _, field := range fields {
		out = append(out, previewField{label: field.Label, value: values.Get(field.Key)})
	}
	return out
}

func statusPrefix(ev lsx.Event) string {
	for _, rule := range statusRules {
		if rule.Match(ev) {
			return rule.Prefix
		}
	}
	return fmt.Sprintf("%d ", ev.Status)
}

func isErrorKind(kind string) bool {
	return strings.Contains(kind, "error") || strings.Contains(kind, "rejected") || kind == "not_found"
}
