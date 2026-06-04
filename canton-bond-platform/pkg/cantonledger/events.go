package cantonledger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// CreatedEvent is a normalized created-event from the active-contracts response.
type CreatedEvent struct {
	ContractID      string
	TemplateID      string
	CreateArguments map[string]any
}

// ExtractCreatedEvents parses active-contracts entries and optionally filters by template ID.
func ExtractCreatedEvents(resp ActiveContractsResponse, filterTemplates ...string) []CreatedEvent {
	var events []CreatedEvent
	for _, entry := range resp {
		ce, ok := extractCreatedEvent(entry)
		if !ok {
			continue
		}
		if len(filterTemplates) > 0 {
			matched := false
			for _, ft := range filterTemplates {
				if TemplateIDMatches(ce.TemplateID, ft) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		events = append(events, ce)
	}
	return events
}

func extractCreatedEvent(entry map[string]any) (CreatedEvent, bool) {
	if ce, ok := entry["createdEvent"].(map[string]any); ok {
		return parseCreatedEvent(ce), true
	}
	if ce, ok := entry["contractEntry"].(map[string]any); ok {
		if evt, ok := ce["createdEvent"].(map[string]any); ok {
			return parseCreatedEvent(evt), true
		}
		if js, ok := ce["JsActiveContract"].(map[string]any); ok {
			if evt, ok := js["createdEvent"].(map[string]any); ok {
				return parseCreatedEvent(evt), true
			}
		}
	}
	return CreatedEvent{}, false
}

func parseCreatedEvent(raw map[string]any) CreatedEvent {
	evt := CreatedEvent{
		CreateArguments: raw,
	}
	if cid, ok := raw["contractId"].(string); ok {
		evt.ContractID = cid
	}
	if tid, ok := raw["templateId"].(string); ok {
		evt.TemplateID = tid
	}
	if args, ok := raw["createArgument"].(map[string]any); ok {
		evt.CreateArguments = args
	}
	return evt
}

// GetField returns a top-level create-argument field.
func (e CreatedEvent) GetField(name string) (any, bool) {
	v, ok := e.CreateArguments[name]
	return v, ok
}

// GetStringField returns a string create-argument field.
func (e CreatedEvent) GetStringField(name string) string {
	v, ok := e.GetField(name)
	if !ok {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		b, _ := json.Marshal(s)
		return string(bytes.Trim(b, "\""))
	}
}

// GetDecimalField parses a numeric create-argument field.
func (e CreatedEvent) GetDecimalField(name string) float64 {
	s := e.GetStringField(name)
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// GetNestedField reads a sub-field from a map-valued create-argument field.
func (e CreatedEvent) GetNestedField(name, subField string) string {
	v, ok := e.GetField(name)
	if !ok {
		return ""
	}
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	sv, ok := m[subField]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", sv)
}

// IsLocked reports whether the event is a locked holding template.
func (e CreatedEvent) IsLocked() bool {
	return strings.Contains(e.TemplateID, "LockedSimpleHolding")
}

// TemplateIDMatches compares full or tail template IDs (module:entity suffix).
func TemplateIDMatches(actual, expected string) bool {
	if actual == expected {
		return true
	}
	if expected == "" || actual == "" {
		return false
	}
	return TemplateIDTail(actual) == TemplateIDTail(expected)
}

// TemplateIDTail returns the module:entity portion after the package hash or # prefix.
func TemplateIDTail(id string) string {
	if idx := strings.Index(id, ":"); idx >= 0 && idx+1 < len(id) {
		return id[idx+1:]
	}
	return id
}
