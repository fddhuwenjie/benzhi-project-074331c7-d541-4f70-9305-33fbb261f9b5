package domain

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type Landmark struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	SymbolID string  `json:"symbol_id"`
	LabelID  string  `json:"label_id"`
}
type PathSegment struct {
	ID             string  `json:"id"`
	FromLandmarkID string  `json:"from_landmark_id"`
	ToLandmarkID   string  `json:"to_landmark_id"`
	Points         []Point `json:"points"`
}
type TactileSymbol struct {
	ID        string  `json:"id"`
	LegendKey string  `json:"legend_key"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	RadiusMM  float64 `json:"radius_mm"`
}
type BrailleLabel struct {
	ID       string  `json:"id"`
	Text     string  `json:"text"`
	Cells    string  `json:"cells"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	WidthMM  float64 `json:"width_mm"`
	HeightMM float64 `json:"height_mm"`
}
type LegendEntry struct {
	Key     string `json:"key"`
	Meaning string `json:"meaning"`
}

type ProofRevision struct {
	ProofID            string          `json:"proof_id"`
	ProjectID          string          `json:"project_id"`
	Sequence           int             `json:"sequence"`
	SourceDigest       string          `json:"source_digest"`
	Landmarks          []Landmark      `json:"landmarks"`
	PathSegments       []PathSegment   `json:"path_segments"`
	TactileSymbols     []TactileSymbol `json:"tactile_symbols"`
	BrailleLabels      []BrailleLabel  `json:"braille_labels"`
	LegendEntries      []LegendEntry   `json:"legend_entries"`
	DeclaredFindingIDs []string        `json:"declared_finding_ids,omitempty"`
	ImpactDigest       string          `json:"impact_digest,omitempty"`
	SubmittedBy        string          `json:"submitted_by"`
	SubmittedAt        time.Time       `json:"submitted_at"`
}

func (p *ProofRevision) Normalize() {
	sort.Slice(p.Landmarks, func(i, j int) bool { return p.Landmarks[i].ID < p.Landmarks[j].ID })
	sort.Slice(p.PathSegments, func(i, j int) bool { return p.PathSegments[i].ID < p.PathSegments[j].ID })
	sort.Slice(p.TactileSymbols, func(i, j int) bool { return p.TactileSymbols[i].ID < p.TactileSymbols[j].ID })
	sort.Slice(p.BrailleLabels, func(i, j int) bool { return p.BrailleLabels[i].ID < p.BrailleLabels[j].ID })
	sort.Slice(p.LegendEntries, func(i, j int) bool { return p.LegendEntries[i].Key < p.LegendEntries[j].Key })
	sort.Strings(p.DeclaredFindingIDs)
}

func (p ProofRevision) Validate() error {
	issues := p.ValidationIssues()
	if len(issues) > 0 {
		return fmt.Errorf("%w: %s %s", ErrInvalid, issues[0].Field, issues[0].Reason)
	}
	return nil
}

var sourceDigestPattern = regexp.MustCompile(`^(sha256:[A-Za-z0-9._-]+|[A-Za-z0-9._-]{8,})$`)

type FieldIssue struct {
	Section string `json:"section"`
	ItemID  string `json:"item_id,omitempty"`
	Field   string `json:"field"`
	Reason  string `json:"reason"`
}

type ProofPreflight struct {
	Ready         bool          `json:"ready"`
	Issues        []FieldIssue  `json:"issues"`
	Preview       ProofRevision `json:"preview"`
	PreviewDigest string        `json:"preview_digest,omitempty"`
	Impact        *ProofDiff    `json:"impact,omitempty"`
}

func (p ProofRevision) ValidationIssues() []FieldIssue {
	issues := []FieldIssue{}
	add := func(section, id, field, reason string) {
		issues = append(issues, FieldIssue{Section: section, ItemID: id, Field: field, Reason: reason})
	}
	if strings.TrimSpace(p.ProofID) == "" {
		add("proof", "", "proof_id", "不能为空")
	}
	if strings.TrimSpace(p.ProjectID) == "" {
		add("proof", "", "project_id", "不能为空")
	}
	if p.Sequence < 1 {
		add("proof", "", "sequence", "必须大于零")
	}
	if strings.TrimSpace(p.SubmittedBy) == "" {
		add("proof", "", "submitted_by", "不能为空")
	}
	if !sourceDigestPattern.MatchString(strings.TrimSpace(p.SourceDigest)) {
		add("proof", "", "source_digest", "必须是至少 8 位的稳定摘要，或 sha256:<摘要>")
	}
	if len(p.Landmarks) == 0 {
		add("landmarks", "", "items", "至少需要一个导览点")
	}
	if len(p.TactileSymbols) == 0 {
		add("tactile_symbols", "", "items", "至少需要一个触觉符号")
	}
	if len(p.BrailleLabels) == 0 {
		add("braille_labels", "", "items", "至少需要一个盲文标签")
	}
	if len(p.LegendEntries) == 0 {
		add("legend_entries", "", "items", "至少需要一个图例条目")
	}
	seen := map[string]string{}
	checkID := func(section, id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			add(section, id, "id", "不能为空")
			return
		}
		if first, ok := seen[id]; ok {
			add(section, id, "id", "与 "+first+" 中的标识重复")
		} else {
			seen[id] = section
		}
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	landmarks, symbols, labels, legends := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, x := range p.Landmarks {
		checkID("landmarks", x.ID)
		landmarks[x.ID] = true
		if strings.TrimSpace(x.Name) == "" {
			add("landmarks", x.ID, "name", "不能为空")
		}
		if !finite(x.X) {
			add("landmarks", x.ID, "x", "必须是有限数值")
		}
		if !finite(x.Y) {
			add("landmarks", x.ID, "y", "必须是有限数值")
		}
	}
	for _, x := range p.PathSegments {
		checkID("path_segments", x.ID)
		if x.FromLandmarkID == x.ToLandmarkID {
			add("path_segments", x.ID, "to_landmark_id", "路径端点不能相同")
		}
		for i, pt := range x.Points {
			if !finite(pt.X) || !finite(pt.Y) {
				add("path_segments", x.ID, fmt.Sprintf("points[%d]", i), "坐标必须是有限数值")
			}
		}
	}
	for _, x := range p.TactileSymbols {
		checkID("tactile_symbols", x.ID)
		symbols[x.ID] = true
		if strings.TrimSpace(x.LegendKey) == "" {
			add("tactile_symbols", x.ID, "legend_key", "不能为空")
		}
		if !finite(x.X) {
			add("tactile_symbols", x.ID, "x", "必须是有限数值")
		}
		if !finite(x.Y) {
			add("tactile_symbols", x.ID, "y", "必须是有限数值")
		}
		if !finite(x.RadiusMM) || x.RadiusMM <= 0 {
			add("tactile_symbols", x.ID, "radius_mm", "必须是有限正数")
		}
	}
	for _, x := range p.BrailleLabels {
		checkID("braille_labels", x.ID)
		labels[x.ID] = true
		if strings.TrimSpace(x.Text) == "" {
			add("braille_labels", x.ID, "text", "不能为空")
		}
		if strings.TrimSpace(x.Cells) == "" {
			add("braille_labels", x.ID, "cells", "不能为空")
		}
		if !finite(x.X) {
			add("braille_labels", x.ID, "x", "必须是有限数值")
		}
		if !finite(x.Y) {
			add("braille_labels", x.ID, "y", "必须是有限数值")
		}
		if !finite(x.WidthMM) || x.WidthMM <= 0 {
			add("braille_labels", x.ID, "width_mm", "必须是有限正数")
		}
		if !finite(x.HeightMM) || x.HeightMM <= 0 {
			add("braille_labels", x.ID, "height_mm", "必须是有限正数")
		}
	}
	for _, x := range p.LegendEntries {
		key := strings.TrimSpace(x.Key)
		if key == "" {
			add("legend_entries", key, "key", "不能为空")
		} else if legends[key] {
			add("legend_entries", key, "key", "图例键重复")
		}
		legends[key] = true
		if strings.TrimSpace(x.Meaning) == "" {
			add("legend_entries", key, "meaning", "不能为空")
		}
	}
	for _, x := range p.Landmarks {
		if x.SymbolID != "" && !symbols[x.SymbolID] {
			add("landmarks", x.ID, "symbol_id", "引用的触觉符号不存在: "+x.SymbolID)
		}
		if x.LabelID != "" && !labels[x.LabelID] {
			add("landmarks", x.ID, "label_id", "引用的盲文标签不存在: "+x.LabelID)
		}
	}
	for _, x := range p.PathSegments {
		if !landmarks[x.FromLandmarkID] {
			add("path_segments", x.ID, "from_landmark_id", "引用的导览点不存在: "+x.FromLandmarkID)
		}
		if !landmarks[x.ToLandmarkID] {
			add("path_segments", x.ID, "to_landmark_id", "引用的导览点不存在: "+x.ToLandmarkID)
		}
	}
	for _, x := range p.TactileSymbols {
		if !legends[x.LegendKey] {
			add("tactile_symbols", x.ID, "legend_key", "引用的图例键不存在: "+x.LegendKey)
		}
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Section != issues[j].Section {
			return issues[i].Section < issues[j].Section
		}
		if issues[i].ItemID != issues[j].ItemID {
			return issues[i].ItemID < issues[j].ItemID
		}
		return issues[i].Field < issues[j].Field
	})
	return issues
}

func PreflightProof(p ProofRevision) ProofPreflight {
	p.Normalize()
	issues := p.ValidationIssues()
	digest := ""
	if len(issues) == 0 {
		digest, _ = Digest(p)
	}
	return ProofPreflight{Ready: len(issues) == 0, Issues: issues, Preview: p, PreviewDigest: digest}
}
func idsLandmarks(v []Landmark) []string {
	r := make([]string, len(v))
	for i, x := range v {
		r[i] = x.ID
	}
	return r
}
func idsPaths(v []PathSegment) []string {
	r := make([]string, len(v))
	for i, x := range v {
		r[i] = x.ID
	}
	return r
}
func idsSymbols(v []TactileSymbol) []string {
	r := make([]string, len(v))
	for i, x := range v {
		r[i] = x.ID
	}
	return r
}
func idsLabels(v []BrailleLabel) []string {
	r := make([]string, len(v))
	for i, x := range v {
		r[i] = x.ID
	}
	return r
}
