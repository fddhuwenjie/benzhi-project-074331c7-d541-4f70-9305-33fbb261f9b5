package domain

import (
	"fmt"
	"strings"
	"time"
)

type MapProject struct {
	ProjectID       string        `json:"project_id"`
	Title           string        `json:"title"`
	VenueZone       string        `json:"venue_zone"`
	SheetWidthMM    float64       `json:"sheet_width_mm"`
	SheetHeightMM   float64       `json:"sheet_height_mm"`
	MinimumGapMM    float64       `json:"minimum_gap_mm"`
	BrailleStandard string        `json:"braille_standard"`
	ReviewerID      string        `json:"reviewer_id"`
	Status          ProjectStatus `json:"status"`
	Revision        int64         `json:"revision"`
	CreatedAt       time.Time     `json:"created_at"`
}

func NewProject(id, title, zone string, width, height, gap float64, standard, reviewer string, now time.Time) (MapProject, error) {
	p := MapProject{ProjectID: strings.TrimSpace(id), Title: strings.TrimSpace(title), VenueZone: strings.TrimSpace(zone), SheetWidthMM: width, SheetHeightMM: height, MinimumGapMM: gap, BrailleStandard: strings.TrimSpace(standard), ReviewerID: strings.TrimSpace(reviewer), Status: StatusDraft, CreatedAt: now.UTC()}
	if err := p.Validate(); err != nil {
		return MapProject{}, err
	}
	return p, nil
}

func (p MapProject) Validate() error {
	if p.ProjectID == "" || p.Title == "" || p.VenueZone == "" || p.ReviewerID == "" {
		return fmt.Errorf("%w: 项目、标题、区域和复核员不能为空", ErrInvalid)
	}
	if p.SheetWidthMM <= 0 || p.SheetHeightMM <= 0 || p.MinimumGapMM <= 0 {
		return fmt.Errorf("%w: 尺寸和间距必须大于零", ErrInvalid)
	}
	if p.BrailleStandard != "GB/T 15720" && p.BrailleStandard != "UEB" {
		return fmt.Errorf("%w: 不支持的盲文规范", ErrInvalid)
	}
	return nil
}

func (p *MapProject) Transition(to ProjectStatus) error {
	if p.Status == StatusPublished {
		return ErrPublished
	}
	if !CanTransition(p.Status, to) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidState, p.Status, to)
	}
	p.Status = to
	return nil
}
