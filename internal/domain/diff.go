package domain

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

type FieldChange struct {
	ItemType   string `json:"item_type"`
	ItemID     string `json:"item_id"`
	ChangeType string `json:"change_type"`
	Field      string `json:"field,omitempty"`
	OldValue   any    `json:"old_value,omitempty"`
	NewValue   any    `json:"new_value,omitempty"`
}

type ImpactRow struct {
	FieldChange
	AffectedRules      []string `json:"affected_rules"`
	FindingIDs         []string `json:"finding_ids"`
	GlobalBoundary     bool     `json:"global_boundary"`
	GlobalConnectivity bool     `json:"global_connectivity"`
	Blocking           bool     `json:"blocking"`
	BlockingReason     string   `json:"blocking_reason,omitempty"`
}

type ProofDiff struct {
	FromSequence    int           `json:"from_sequence"`
	ToSequence      int           `json:"to_sequence"`
	ChangedSections []string      `json:"changed_sections"`
	AddedIDs        []string      `json:"added_ids"`
	RemovedIDs      []string      `json:"removed_ids"`
	ModifiedIDs     []string      `json:"modified_ids"`
	Changes         []FieldChange `json:"changes"`
	ImpactRows      []ImpactRow   `json:"impact_rows,omitempty"`
	ImpactDigest    string        `json:"impact_digest,omitempty"`
	Blocked         bool          `json:"blocked,omitempty"`
}

type diffRecord struct {
	section, id string
	value       any
}

func proofRecords(p ProofRevision) []diffRecord {
	out := []diffRecord{}
	for _, x := range p.Landmarks {
		out = append(out, diffRecord{"landmarks", x.ID, x})
	}
	for _, x := range p.PathSegments {
		out = append(out, diffRecord{"path_segments", x.ID, x})
	}
	for _, x := range p.TactileSymbols {
		out = append(out, diffRecord{"tactile_symbols", x.ID, x})
	}
	for _, x := range p.BrailleLabels {
		out = append(out, diffRecord{"braille_labels", x.ID, x})
	}
	for _, x := range p.LegendEntries {
		out = append(out, diffRecord{"legend_entries", "legend:" + x.Key, x})
	}
	return out
}

func valueMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	m := map[string]any{}
	_ = json.Unmarshal(b, &m)
	return m
}

func DiffProofs(a, b ProofRevision) ProofDiff {
	d := ProofDiff{FromSequence: a.Sequence, ToSequence: b.Sequence}
	am, bm := map[string]diffRecord{}, map[string]diffRecord{}
	for _, r := range proofRecords(a) {
		am[r.section+"\x00"+r.id] = r
	}
	for _, r := range proofRecords(b) {
		bm[r.section+"\x00"+r.id] = r
	}
	sections := map[string]bool{}
	for key, av := range am {
		bv, ok := bm[key]
		if !ok {
			d.RemovedIDs = append(d.RemovedIDs, av.id)
			d.Changes = append(d.Changes, FieldChange{ItemType: av.section, ItemID: av.id, ChangeType: "删除", OldValue: av.value})
			sections[av.section] = true
			continue
		}
		if reflect.DeepEqual(av.value, bv.value) {
			continue
		}
		d.ModifiedIDs = append(d.ModifiedIDs, av.id)
		sections[av.section] = true
		oldFields, newFields := valueMap(av.value), valueMap(bv.value)
		fields := make([]string, 0, len(oldFields)+len(newFields))
		seen := map[string]bool{}
		for f := range oldFields {
			if f != "id" && f != "key" {
				fields = append(fields, f)
				seen[f] = true
			}
		}
		for f := range newFields {
			if f != "id" && f != "key" && !seen[f] {
				fields = append(fields, f)
			}
		}
		sort.Strings(fields)
		for _, f := range fields {
			if !reflect.DeepEqual(oldFields[f], newFields[f]) {
				d.Changes = append(d.Changes, FieldChange{ItemType: av.section, ItemID: av.id, ChangeType: "修改", Field: f, OldValue: oldFields[f], NewValue: newFields[f]})
			}
		}
	}
	for key, bv := range bm {
		if _, ok := am[key]; !ok {
			d.AddedIDs = append(d.AddedIDs, bv.id)
			d.Changes = append(d.Changes, FieldChange{ItemType: bv.section, ItemID: bv.id, ChangeType: "新增", NewValue: bv.value})
			sections[bv.section] = true
		}
	}
	for s := range sections {
		d.ChangedSections = append(d.ChangedSections, s)
	}
	sort.Strings(d.ChangedSections)
	sort.Strings(d.AddedIDs)
	sort.Strings(d.RemovedIDs)
	sort.Strings(d.ModifiedIDs)
	sort.SliceStable(d.Changes, func(i, j int) bool {
		a, b := d.Changes[i], d.Changes[j]
		if a.ItemType != b.ItemType {
			return a.ItemType < b.ItemType
		}
		if a.ItemID != b.ItemID {
			return a.ItemID < b.ItemID
		}
		if a.ChangeType != b.ChangeType {
			return a.ChangeType < b.ChangeType
		}
		return a.Field < b.Field
	})
	return d
}

func (d ProofDiff) ChangedIDSet() map[string]bool {
	out := map[string]bool{}
	for _, id := range d.AddedIDs {
		out[id] = true
	}
	for _, id := range d.RemovedIDs {
		out[id] = true
	}
	for _, id := range d.ModifiedIDs {
		out[id] = true
	}
	return out
}
func (d ProofDiff) ValidateAdjacent() error {
	if d.ToSequence != d.FromSequence+1 {
		return fmt.Errorf("%w: 只能比较相邻校样", ErrInvalid)
	}
	return nil
}
