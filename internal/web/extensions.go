package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"tactile-atlas-gate/internal/domain"
	"tactile-atlas-gate/internal/workflow"
)

func (h *Handler) PreflightSpecification(w http.ResponseWriter, r *http.Request) {
	id, err := projectID(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	var in struct {
		Actor            string `json:"actor"`
		ExpectedRevision int64  `json:"expected_revision"`
	}
	if err = decodeJSON(w, r, &in); err != nil {
		badRequest(w, err)
		return
	}
	result, err := h.service.PreflightSpecification(id, in.Actor, in.ExpectedRevision)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) PreflightProof(w http.ResponseWriter, r *http.Request) {
	id, err := projectID(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	var in proofRequest
	if err = decodeJSON(w, r, &in); err != nil {
		badRequest(w, err)
		return
	}
	result, err := h.service.PreflightProof(id, in.Actor, in.ExpectedRevision, in.Proof)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) ValidationHistory(w http.ResponseWriter, r *http.Request) {
	id, err := projectID(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	filter := workflow.ValidationRunFilter{RuleVersion: strings.TrimSpace(r.URL.Query().Get("rule_version"))}
	if value := r.URL.Query().Get("proof_sequence"); value != "" {
		filter.ProofSequence, err = strconv.Atoi(value)
		if err != nil || filter.ProofSequence < 1 {
			badRequest(w, domain.ErrInvalid)
			return
		}
	}
	if value := r.URL.Query().Get("type"); value != "" {
		targeted := value == "targeted"
		if value != "targeted" && value != "full" {
			badRequest(w, domain.ErrInvalid)
			return
		}
		filter.Targeted = &targeted
	}
	if value := r.URL.Query().Get("from"); value != "" {
		filter.From, err = time.Parse(time.RFC3339, value)
		if err != nil {
			badRequest(w, err)
			return
		}
	}
	if value := r.URL.Query().Get("to"); value != "" {
		filter.To, err = time.Parse(time.RFC3339, value)
		if err != nil {
			badRequest(w, err)
			return
		}
	}
	items, err := h.service.ValidationHistory(id, filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) CompareValidationRuns(w http.ResponseWriter, r *http.Request) {
	id, err := projectID(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	baseline, target := strings.TrimSpace(r.URL.Query().Get("baseline")), strings.TrimSpace(r.URL.Query().Get("target"))
	if baseline == "" || target == "" {
		badRequest(w, domain.ErrInvalid)
		return
	}
	result, err := h.service.CompareValidation(id, baseline, target)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) ListFindings(w http.ResponseWriter, r *http.Request) {
	id, err := projectID(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	q := r.URL.Query()
	filter := workflow.FindingFilter{Source: q.Get("source"), Severity: q.Get("severity"), Status: q.Get("status"), RuleCode: q.Get("rule_code"), ElementRef: q.Get("element_ref")}
	if value := q.Get("proof_sequence"); value != "" {
		filter.ProofSequence, err = strconv.Atoi(value)
		if err != nil || filter.ProofSequence < 1 {
			badRequest(w, domain.ErrInvalid)
			return
		}
	}
	result, err := h.service.FilterFindings(id, filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) BatchDecideFindings(w http.ResponseWriter, r *http.Request) {
	id, err := projectID(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	var in struct {
		Actor            string            `json:"actor"`
		RequestID        string            `json:"request_id"`
		ExpectedRevision int64             `json:"expected_revision"`
		FindingIDs       []string          `json:"finding_ids"`
		Severity         string            `json:"severity"`
		Status           string            `json:"status"`
		Evidence         map[string]string `json:"evidence"`
	}
	if err = decodeJSON(w, r, &in); err != nil {
		badRequest(w, err)
		return
	}
	result, err := h.service.BatchDecideFindings(id, in.Actor, in.RequestID, in.ExpectedRevision, workflow.BatchFindingInput{FindingIDs: in.FindingIDs, Severity: in.Severity, Status: in.Status, Evidence: in.Evidence})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetApprovalGate(w http.ResponseWriter, r *http.Request) {
	id, err := projectID(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	actor := strings.TrimSpace(r.URL.Query().Get("actor"))
	if actor == "" {
		badRequest(w, domain.ErrInvalid)
		return
	}
	gate, err := h.service.ApprovalGate(id, actor)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, gate)
}
