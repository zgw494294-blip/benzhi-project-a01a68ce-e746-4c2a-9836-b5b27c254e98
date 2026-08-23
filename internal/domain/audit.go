package domain

import "sort"

type AuditSummary struct {
	Total       int            `json:"total"`
	ByAction    map[string]int `json:"byAction"`
	ByActor     map[string]int `json:"byActor"`
	FirstAction *AuditEntry    `json:"firstAction,omitempty"`
	LastAction  *AuditEntry    `json:"lastAction,omitempty"`
}

func (s *TastingSession) AuditSummary() AuditSummary {
	result := AuditSummary{Total: len(s.Audit), ByAction: map[string]int{}, ByActor: map[string]int{}}
	if len(s.Audit) == 0 {
		return result
	}
	entries := append([]AuditEntry(nil), s.Audit...)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].At.Before(entries[j].At) })
	for _, entry := range entries {
		result.ByAction[entry.Action]++
		result.ByActor[entry.Actor]++
	}
	first, last := entries[0], entries[len(entries)-1]
	result.FirstAction, result.LastAction = &first, &last
	return result
}

func (s *TastingSession) AuditEntries(actorFilter, actionFilter string) []AuditEntry {
	entries := make([]AuditEntry, 0, len(s.Audit))
	for _, entry := range s.Audit {
		if actorFilter != "" && entry.Actor != actorFilter {
			continue
		}
		if actionFilter != "" && entry.Action != actionFilter {
			continue
		}
		copy := entry
		if entry.Detail != nil {
			copy.Detail = map[string]string{}
			for k, v := range entry.Detail {
				copy.Detail[k] = v
			}
		}
		entries = append(entries, copy)
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].At.Before(entries[j].At) })
	return entries
}
