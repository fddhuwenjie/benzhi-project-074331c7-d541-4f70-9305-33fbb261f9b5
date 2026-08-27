package web

import (
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"tactile-atlas-gate/internal/domain"
	"tactile-atlas-gate/internal/workflow"
)

type Handler struct {
	service *workflow.Service
	mux     *http.ServeMux
}

func NewHandler(service *workflow.Service) *Handler {
	h := &Handler{service: service, mux: http.NewServeMux()}
	h.routes()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) routes() {
	staticFS, _ := fs.Sub(assets, "static")
	h.mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(staticFS))))
	h.mux.HandleFunc("GET /", h.Workbench)
	h.mux.HandleFunc("GET /healthz", h.Health)
	h.mux.HandleFunc("GET /ui-api/projects", h.ListProjects)
	h.mux.HandleFunc("POST /ui-api/projects", h.CreateProject)
	h.mux.HandleFunc("GET /ui-api/projects/{project_id}", h.GetProject)
	h.mux.HandleFunc("POST /ui-api/projects/{project_id}/spec-preflight", h.PreflightSpecification)
	h.mux.HandleFunc("POST /ui-api/projects/{project_id}/freeze", h.FreezeProject)
	h.mux.HandleFunc("POST /ui-api/projects/{project_id}/proofs/preflight", h.PreflightProof)
	h.mux.HandleFunc("POST /ui-api/projects/{project_id}/proofs", h.SubmitProof)
	h.mux.HandleFunc("GET /ui-api/projects/{project_id}/validation-runs", h.ValidationHistory)
	h.mux.HandleFunc("POST /ui-api/projects/{project_id}/validation-runs", h.RunValidation)
	h.mux.HandleFunc("GET /ui-api/projects/{project_id}/validation-runs/compare", h.CompareValidationRuns)
	h.mux.HandleFunc("GET /ui-api/projects/{project_id}/findings", h.ListFindings)
	h.mux.HandleFunc("POST /ui-api/projects/{project_id}/findings", h.AddFinding)
	h.mux.HandleFunc("POST /ui-api/projects/{project_id}/findings/batch", h.BatchDecideFindings)
	h.mux.HandleFunc("POST /ui-api/projects/{project_id}/findings/{finding_id}/decision", h.DecideFinding)
	h.mux.HandleFunc("GET /ui-api/projects/{project_id}/diff", h.GetDiff)
	h.mux.HandleFunc("GET /ui-api/projects/{project_id}/approval-gate", h.GetApprovalGate)
	h.mux.HandleFunc("POST /ui-api/projects/{project_id}/approve", h.ApproveProject)
	h.mux.HandleFunc("GET /ui-api/projects/{project_id}/manifest/verify", h.VerifyManifest)
}

func (h *Handler) Workbench(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	b, err := assets.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "页面资源不可用", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ListProjects(w http.ResponseWriter, _ *http.Request) {
	items, err := h.service.List()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func projectID(r *http.Request) (string, error) {
	id := strings.TrimSpace(r.PathValue("project_id"))
	if id == "" {
		return "", errors.New("project_id 不能为空")
	}
	return id, nil
}

func queryInt(r *http.Request, key string) (int, error) {
	v, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || v < 1 {
		return 0, errors.New(key + " 必须是正整数")
	}
	return v, nil
}

func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	id, err := projectID(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	a, err := h.service.Get(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

type createRequest struct {
	ProjectID       string  `json:"project_id"`
	Title           string  `json:"title"`
	VenueZone       string  `json:"venue_zone"`
	SheetWidthMM    float64 `json:"sheet_width_mm"`
	SheetHeightMM   float64 `json:"sheet_height_mm"`
	MinimumGapMM    float64 `json:"minimum_gap_mm"`
	BrailleStandard string  `json:"braille_standard"`
	ReviewerID      string  `json:"reviewer_id"`
	RequestID       string  `json:"request_id"`
}

func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var in createRequest
	if err := decodeJSON(w, r, &in); err != nil {
		badRequest(w, err)
		return
	}
	a, err := h.service.CreateProjectContext(r.Context(), workflow.CreateInput{ProjectID: in.ProjectID, Title: in.Title, VenueZone: in.VenueZone, Width: in.SheetWidthMM, Height: in.SheetHeightMM, Gap: in.MinimumGapMM, Standard: in.BrailleStandard, Reviewer: in.ReviewerID, RequestID: in.RequestID})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

type commandRequest struct {
	Actor            string `json:"actor"`
	RequestID        string `json:"request_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	PreflightDigest  string `json:"preflight_digest,omitempty"`
	GateDigest       string `json:"gate_digest,omitempty"`
}

func (h *Handler) FreezeProject(w http.ResponseWriter, r *http.Request) {
	id, err := projectID(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	var in commandRequest
	if err = decodeJSON(w, r, &in); err != nil {
		badRequest(w, err)
		return
	}
	a, err := h.service.FreezeWithPreflight(id, in.Actor, in.RequestID, in.ExpectedRevision, in.PreflightDigest)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

type proofRequest struct {
	Actor            string               `json:"actor"`
	RequestID        string               `json:"request_id"`
	ExpectedRevision int64                `json:"expected_revision"`
	Proof            domain.ProofRevision `json:"proof"`
}

func (h *Handler) SubmitProof(w http.ResponseWriter, r *http.Request) {
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
	a, err := h.service.SubmitProof(id, in.Actor, in.RequestID, in.ExpectedRevision, in.Proof)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

type validationRequest struct {
	Actor            string   `json:"actor"`
	RequestID        string   `json:"request_id"`
	ExpectedRevision int64    `json:"expected_revision"`
	Rules            []string `json:"rules"`
}

func (h *Handler) RunValidation(w http.ResponseWriter, r *http.Request) {
	id, err := projectID(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	var in validationRequest
	if err = decodeJSON(w, r, &in); err != nil {
		badRequest(w, err)
		return
	}
	a, err := h.service.Validate(id, in.Actor, in.RequestID, in.ExpectedRevision, in.Rules)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

type findingRequest struct {
	Actor            string   `json:"actor"`
	RequestID        string   `json:"request_id"`
	ExpectedRevision int64    `json:"expected_revision"`
	Severity         string   `json:"severity"`
	Description      string   `json:"description"`
	ElementRefs      []string `json:"element_refs"`
}

func (h *Handler) AddFinding(w http.ResponseWriter, r *http.Request) {
	id, err := projectID(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	var in findingRequest
	if err = decodeJSON(w, r, &in); err != nil {
		badRequest(w, err)
		return
	}
	a, err := h.service.AddFinding(id, in.Actor, in.RequestID, in.Severity, in.Description, in.ElementRefs, in.ExpectedRevision)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

type decisionRequest struct {
	Actor              string `json:"actor"`
	RequestID          string `json:"request_id"`
	ExpectedRevision   int64  `json:"expected_revision"`
	Status             string `json:"status"`
	ResolutionEvidence string `json:"resolution_evidence"`
}

func (h *Handler) DecideFinding(w http.ResponseWriter, r *http.Request) {
	id, err := projectID(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	var in decisionRequest
	if err = decodeJSON(w, r, &in); err != nil {
		badRequest(w, err)
		return
	}
	a, err := h.service.DecideFinding(id, r.PathValue("finding_id"), in.Actor, in.RequestID, in.Status, in.ResolutionEvidence, in.ExpectedRevision)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *Handler) GetDiff(w http.ResponseWriter, r *http.Request) {
	id, err := projectID(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	from, err := queryInt(r, "from")
	if err != nil {
		badRequest(w, err)
		return
	}
	to, err := queryInt(r, "to")
	if err != nil {
		badRequest(w, err)
		return
	}
	diff, err := h.service.ImpactMatrix(id, from, to)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, diff)
}

func (h *Handler) ApproveProject(w http.ResponseWriter, r *http.Request) {
	id, err := projectID(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	var in commandRequest
	if err = decodeJSON(w, r, &in); err != nil {
		badRequest(w, err)
		return
	}
	a, err := h.service.ApproveWithGate(id, in.Actor, in.RequestID, in.ExpectedRevision, in.GateDigest)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *Handler) VerifyManifest(w http.ResponseWriter, r *http.Request) {
	id, err := projectID(r)
	if err != nil {
		badRequest(w, err)
		return
	}
	report, err := h.service.VerifyManifestDetailed(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}
