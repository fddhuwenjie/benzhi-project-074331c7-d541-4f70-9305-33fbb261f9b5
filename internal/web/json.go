package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"tactile-atlas-gate/internal/domain"
)

const maxBodyBytes = 1 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("请求正文只能包含一个 JSON 对象")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrIdempotency), errors.Is(err, domain.ErrPublished), errors.Is(err, domain.ErrInvalidState), errors.Is(err, domain.ErrApprovalGate), errors.Is(err, domain.ErrReviewer):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrInvalid):
		status = http.StatusUnprocessableEntity
	}
	if status == http.StatusInternalServerError {
		writeJSON(w, status, map[string]string{"error": "服务处理失败"})
		return
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func badRequest(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误: " + err.Error()})
}
