package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type checkAggregate struct {
	Project struct {
		ProjectID string `json:"project_id"`
		Status    string `json:"status"`
		Revision  int64  `json:"revision"`
	} `json:"project"`
}

func runSelfCheck(ctx context.Context, addr string) error {
	client := &http.Client{}
	base := "http://" + addr
	var aggregate checkAggregate
	create := map[string]any{"project_id": "self-check-map", "title": "自检触觉导览图", "venue_zone": "一层常设展厅", "sheet_width_mm": 300, "sheet_height_mm": 200, "minimum_gap_mm": 3, "braille_standard": "GB/T 15720", "reviewer_id": "reviewer-self-check", "request_id": "self-create"}
	if err := checkJSON(ctx, client, http.MethodPost, base+"/ui-api/projects", create, &aggregate); err != nil {
		return err
	}
	var specPreflight struct {
		Ready         bool   `json:"ready"`
		SummaryDigest string `json:"summary_digest"`
	}
	if err := checkJSON(ctx, client, http.MethodPost, base+"/ui-api/projects/self-check-map/spec-preflight", map[string]any{"actor": "maker-self-check", "expected_revision": aggregate.Project.Revision}, &specPreflight); err != nil {
		return err
	}
	if !specPreflight.Ready {
		return fmt.Errorf("自检规格预检未通过")
	}
	freeze := map[string]any{"actor": "maker-self-check", "request_id": "self-freeze", "expected_revision": aggregate.Project.Revision, "preflight_digest": specPreflight.SummaryDigest}
	if err := checkJSON(ctx, client, http.MethodPost, base+"/ui-api/projects/self-check-map/freeze", freeze, &aggregate); err != nil {
		return err
	}
	proof := map[string]any{
		"proof_id": "proof-self-1", "source_digest": "sha256:self-check-source-v1",
		"landmarks":       []any{map[string]any{"id": "L1", "name": "入口", "x": 30, "y": 30, "symbol_id": "S1", "label_id": "B1"}, map[string]any{"id": "L2", "name": "展厅", "x": 180, "y": 30, "symbol_id": "S2", "label_id": "B2"}},
		"path_segments":   []any{map[string]any{"id": "P1", "from_landmark_id": "L1", "to_landmark_id": "L2", "points": []any{map[string]any{"x": 30, "y": 30}, map[string]any{"x": 180, "y": 30}}}},
		"tactile_symbols": []any{map[string]any{"id": "S1", "legend_key": "entrance", "x": 30, "y": 30, "radius_mm": 4}, map[string]any{"id": "S2", "legend_key": "gallery", "x": 180, "y": 30, "radius_mm": 4}},
		"braille_labels":  []any{map[string]any{"id": "B1", "text": "入口", "cells": "⠁⠃", "x": 30, "y": 55, "width_mm": 12, "height_mm": 8}, map[string]any{"id": "B2", "text": "展厅", "cells": "⠉⠙", "x": 180, "y": 55, "width_mm": 12, "height_mm": 8}},
		"legend_entries":  []any{map[string]any{"key": "entrance", "meaning": "入口"}, map[string]any{"key": "gallery", "meaning": "展厅"}},
	}
	submit := map[string]any{"actor": "maker-self-check", "request_id": "self-proof", "expected_revision": aggregate.Project.Revision, "proof": proof}
	if err := checkJSON(ctx, client, http.MethodPost, base+"/ui-api/projects/self-check-map/proofs", submit, &aggregate); err != nil {
		return err
	}
	validate := map[string]any{"actor": "reviewer-self-check", "request_id": "self-validation", "expected_revision": aggregate.Project.Revision}
	if err := checkJSON(ctx, client, http.MethodPost, base+"/ui-api/projects/self-check-map/validation-runs", validate, &aggregate); err != nil {
		return err
	}
	if aggregate.Project.Status != "待批准" {
		return fmt.Errorf("自检规则检查后状态异常: %s", aggregate.Project.Status)
	}
	var gate struct {
		Ready         bool   `json:"ready"`
		SummaryDigest string `json:"summary_digest"`
	}
	if err := checkJSON(ctx, client, http.MethodGet, base+"/ui-api/projects/self-check-map/approval-gate?actor=reviewer-self-check", nil, &gate); err != nil {
		return err
	}
	if !gate.Ready {
		return fmt.Errorf("自检独立批准门禁未通过")
	}
	approve := map[string]any{"actor": "reviewer-self-check", "request_id": "self-approve", "expected_revision": aggregate.Project.Revision, "gate_digest": gate.SummaryDigest}
	if err := checkJSON(ctx, client, http.MethodPost, base+"/ui-api/projects/self-check-map/approve", approve, &aggregate); err != nil {
		return err
	}
	if aggregate.Project.Status != "已发布" {
		return fmt.Errorf("自检未进入已发布状态")
	}
	var verified struct {
		Valid bool `json:"valid"`
		Items []struct {
			Passed bool   `json:"passed"`
			Type   string `json:"type"`
		} `json:"items"`
	}
	if err := checkJSON(ctx, client, http.MethodGet, base+"/ui-api/projects/self-check-map/manifest/verify", nil, &verified); err != nil {
		return err
	}
	if !verified.Valid {
		return fmt.Errorf("自检母版清单摘要无效: %+v", verified.Items)
	}
	if len(verified.Items) != 5 {
		return fmt.Errorf("自检母版逐项证据数量异常: %d", len(verified.Items))
	}
	for _, item := range verified.Items {
		if !item.Passed {
			return fmt.Errorf("自检母版证据未通过: %s", item.Type)
		}
	}
	return nil
}

func checkJSON(ctx context.Context, client *http.Client, method, url string, input, output any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("自检 HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("自检 %s %s 返回 %d: %s", method, url, resp.StatusCode, message)
	}
	if output != nil {
		if err := json.NewDecoder(resp.Body).Decode(output); err != nil {
			return err
		}
	}
	return nil
}
