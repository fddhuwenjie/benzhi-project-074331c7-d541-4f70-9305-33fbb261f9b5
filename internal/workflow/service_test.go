package workflow

import (
	"errors"
	"fmt"
	"testing"

	"tactile-atlas-gate/internal/domain"
	"tactile-atlas-gate/internal/repository"
	"tactile-atlas-gate/internal/validation"
)

func serviceFixture(t *testing.T) (*Service, domain.Aggregate, domain.ProofRevision) {
	t.Helper()
	store, err := repository.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(store, validation.NewEngine())
	a, err := s.CreateProject(CreateInput{ProjectID: "flow-1", Title: "流程测试", VenueZone: "展厅", Width: 300, Height: 200, Gap: 3, Standard: "GB/T 15720", Reviewer: "reviewer", RequestID: "create-1"})
	if err != nil {
		t.Fatal(err)
	}
	p := domain.ProofRevision{ProofID: "proof-1", SourceDigest: "sha256:fixture", Landmarks: []domain.Landmark{{ID: "L1", Name: "入口", X: 30, Y: 30}, {ID: "L2", Name: "展厅", X: 180, Y: 30}}, PathSegments: []domain.PathSegment{{ID: "P1", FromLandmarkID: "L1", ToLandmarkID: "L2"}}, TactileSymbols: []domain.TactileSymbol{{ID: "S1", LegendKey: "entry", X: 30, Y: 30, RadiusMM: 4}, {ID: "S2", LegendKey: "hall", X: 180, Y: 30, RadiusMM: 4}}, BrailleLabels: []domain.BrailleLabel{{ID: "B1", Text: "入口", Cells: "⠁", X: 30, Y: 55, WidthMM: 12, HeightMM: 8}, {ID: "B2", Text: "展厅", Cells: "⠃", X: 180, Y: 55, WidthMM: 12, HeightMM: 8}}, LegendEntries: []domain.LegendEntry{{Key: "entry", Meaning: "入口"}, {Key: "hall", Meaning: "展厅"}}}
	return s, a, p
}

func TestWorkflowRejectsStaleAndIdempotencyConflict(t *testing.T) {
	s, a, _ := serviceFixture(t)
	if _, err := s.Freeze(a.Project.ProjectID, "maker", "freeze-1", 99); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("期望修订冲突，得到 %v", err)
	}
	frozen, err := s.Freeze(a.Project.ProjectID, "maker", "freeze-1", a.Project.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Freeze(a.Project.ProjectID, "other", "freeze-1", frozen.Project.Revision); !errors.Is(err, domain.ErrIdempotency) {
		t.Fatalf("期望幂等冲突，得到 %v", err)
	}
}

func TestWorkflowPublishesOnlyAfterIndependentApproval(t *testing.T) {
	s, a, proof := serviceFixture(t)
	frozen, err := s.Freeze(a.Project.ProjectID, "maker", "freeze-2", a.Project.Revision)
	if err != nil {
		t.Fatal(err)
	}
	submitted, err := s.SubmitProof(a.Project.ProjectID, "maker", "proof-2", frozen.Project.Revision, proof)
	if err != nil {
		t.Fatal(err)
	}
	checked, err := s.Validate(a.Project.ProjectID, "reviewer", "validate-2", submitted.Project.Revision, nil)
	if err != nil {
		t.Fatal(err)
	}
	if checked.Project.Status != domain.StatusApproval {
		t.Fatalf("检查后状态为 %s", checked.Project.Status)
	}
	if _, err = s.Approve(a.Project.ProjectID, "maker", "approve-bad", checked.Project.Revision); !errors.Is(err, domain.ErrReviewer) {
		t.Fatalf("非复核员批准未拒绝: %v", err)
	}
	published, err := s.Approve(a.Project.ProjectID, "reviewer", "approve-2", checked.Project.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if published.Project.Status != domain.StatusPublished || published.Manifest == nil || !domain.VerifyManifest(*published.Manifest) {
		t.Fatal("发布清单或状态无效")
	}
	if _, err = s.SubmitProof(a.Project.ProjectID, "maker", "proof-after", published.Project.Revision, proof); !errors.Is(err, domain.ErrPublished) {
		t.Fatalf("发布后写入未拒绝: %v", err)
	}
}

func TestWorkflowFindingRequiresTargetedRevalidation(t *testing.T) {
	s, a, proof := serviceFixture(t)
	frozen, _ := s.Freeze(a.Project.ProjectID, "maker", "freeze-3", a.Project.Revision)
	proof.BrailleLabels[0].Cells = "invalid"
	bad, err := s.SubmitProof(a.Project.ProjectID, "maker", "proof-3", frozen.Project.Revision, proof)
	if err != nil {
		t.Fatal(err)
	}
	checked, err := s.Validate(a.Project.ProjectID, "reviewer", "validate-3", bad.Project.Revision, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(checked.Findings) == 0 {
		t.Fatal("未生成规则问题")
	}
	finding := checked.Findings[0]
	if _, err = s.DecideFinding(a.Project.ProjectID, finding.FindingID, "reviewer", "close-3", string(domain.FindingClosed), "未复验", checked.Project.Revision); !errors.Is(err, domain.ErrApprovalGate) {
		t.Fatalf("未复验问题被关闭: %v", err)
	}
}

func TestStrictFreezeRejectsFailedPreflightWithoutRevisionChange(t *testing.T) {
	s, _, _ := serviceFixture(t)
	created, err := s.CreateProject(CreateInput{ProjectID: "bad-spec", Title: "规格冲突", VenueZone: "展厅", Width: 100, Height: 80, Gap: 80, Standard: "GB/T 15720", Reviewer: "reviewer", RequestID: "bad-create"})
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := s.PreflightSpecification("bad-spec", "maker", created.Project.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if preflight.Ready {
		t.Fatal("冲突规格预检不应通过")
	}
	if _, err = s.FreezeWithPreflight("bad-spec", "maker", "bad-freeze", created.Project.Revision, preflight.SummaryDigest); !errors.Is(err, domain.ErrApprovalGate) {
		t.Fatalf("不合格规格被冻结: %v", err)
	}
	loaded, _ := s.Get("bad-spec")
	if loaded.Project.Revision != created.Project.Revision || loaded.Project.Status != domain.StatusDraft {
		t.Fatalf("拒绝冻结改变了项目: %#v", loaded.Project)
	}
}

func TestInvalidProofDoesNotCreateSnapshot(t *testing.T) {
	s, a, p := serviceFixture(t)
	frozen, err := s.Freeze(a.Project.ProjectID, "maker", "freeze-invalid-proof", a.Project.Revision)
	if err != nil {
		t.Fatal(err)
	}
	p.TactileSymbols[0].LegendKey = "missing"
	if _, err = s.SubmitProof(a.Project.ProjectID, "maker", "invalid-proof", frozen.Project.Revision, p); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("错误引用未被拒绝: %v", err)
	}
	loaded, _ := s.Get(a.Project.ProjectID)
	if len(loaded.Proofs) != 0 || loaded.Project.Revision != frozen.Project.Revision || loaded.Project.Status != domain.StatusFrozen {
		t.Fatal("错误校样留下了半成品或推进了状态")
	}
}

func TestBatchFindingDecisionIsAtomicAndIdempotent(t *testing.T) {
	s, a, p := serviceFixture(t)
	frozen, _ := s.Freeze(a.Project.ProjectID, "maker", "batch-freeze", a.Project.Revision)
	submitted, err := s.SubmitProof(a.Project.ProjectID, "maker", "batch-proof", frozen.Project.Revision, p)
	if err != nil {
		t.Fatal(err)
	}
	checked, err := s.Validate(a.Project.ProjectID, "reviewer", "batch-check", submitted.Project.Revision, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		checked, err = s.AddFinding(a.Project.ProjectID, "reviewer", fmt.Sprintf("manual-request-%d", i), "一般", fmt.Sprintf("人工问题 %d", i), []string{"L1"}, checked.Project.Revision)
		if err != nil {
			t.Fatal(err)
		}
	}
	ids := []string{checked.Findings[0].FindingID, checked.Findings[1].FindingID, checked.Findings[2].FindingID}
	beforeTimeline := len(checked.Timeline)
	confirmed, err := s.BatchDecideFindings(a.Project.ProjectID, "reviewer", "batch-confirm", checked.Project.Revision, BatchFindingInput{FindingIDs: ids, Status: string(domain.FindingConfirmed), Evidence: map[string]string{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range confirmed.Findings {
		if f.Status != domain.FindingConfirmed {
			t.Fatalf("问题未被统一确认: %s", f.Status)
		}
	}
	if len(confirmed.Timeline) != beforeTimeline+1 {
		t.Fatal("批量确认没有只写入一条时间线")
	}
	replayed, err := s.BatchDecideFindings(a.Project.ProjectID, "reviewer", "batch-confirm", checked.Project.Revision, BatchFindingInput{FindingIDs: ids, Status: string(domain.FindingConfirmed), Evidence: map[string]string{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(replayed.Timeline) != len(confirmed.Timeline) {
		t.Fatal("幂等重放重复写入了事件")
	}
	if _, err = s.BatchDecideFindings(a.Project.ProjectID, "reviewer", "batch-close", confirmed.Project.Revision, BatchFindingInput{FindingIDs: ids, Severity: "严重", Status: string(domain.FindingClosed), Evidence: map[string]string{ids[0]: "证据"}}); !errors.Is(err, domain.ErrApprovalGate) {
		t.Fatalf("缺少逐项证据的批量关闭未拒绝: %v", err)
	}
	loaded, _ := s.Get(a.Project.ProjectID)
	for _, f := range loaded.Findings {
		if f.Severity != "一般" || f.Status != domain.FindingConfirmed {
			t.Fatal("失败批次发生了部分写入")
		}
	}
}
