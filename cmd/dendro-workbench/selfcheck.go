package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"dendro-chronology-workbench/internal/domain"
)

type checkClient struct {
	base   string
	client *http.Client
}

func runSelfCheck(ctx context.Context, base string) error {
	c := checkClient{base: base, client: &http.Client{Timeout: 4 * time.Second}}
	now := time.Now().UTC().Truncate(time.Second)
	batchID := "self-check-batch"
	create := map[string]any{"request_id": "sc-create-001", "expected_revision": 0, "actor_id": "operator-sc", "batch_id": batchID, "site_code": "SC-SITE", "species": "Pinus test", "sampled_at": now.Add(-24 * time.Hour), "operator_id": "operator-sc", "cores": []map[string]any{{"core_id": "core-sc-a", "tree_code": "tree-a", "radius_code": "A"}, {"core_id": "core-sc-b", "tree_code": "tree-b", "radius_code": "A"}}}
	batch, _, err := c.postBatch(ctx, "/api/batches", create, http.StatusCreated)
	if err != nil {
		return fmt.Errorf("自检建档: %w", err)
	}
	digestA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	images := map[string]any{"request_id": "sc-images-001", "expected_revision": batch.Revision, "actor_id": "operator-sc", "images": []map[string]any{{"core_id": "core-sc-a", "preparation_method": "fine sanding", "image_digest": digestA, "microns_per_pixel": 2.5, "captured_at": now}, {"core_id": "core-sc-b", "preparation_method": "fine sanding", "image_digest": digestB, "microns_per_pixel": 2.5, "captured_at": now}}}
	batch, _, err = c.postBatch(ctx, "/api/batches/"+batchID+"/images", images, http.StatusOK)
	if err != nil {
		return fmt.Errorf("自检影像登记: %w", err)
	}
	observations := []map[string]any{}
	widthsA := []float64{100, 220, 340, 190}
	widthsB := []float64{105, -20, 350, 200}
	for ci, core := range []string{"core-sc-a", "core-sc-b"} {
		widths := widthsA
		if ci == 1 {
			widths = widthsB
		}
		for i, width := range widths {
			obs := map[string]any{"observation_id": fmt.Sprintf("obs-%d-%d", ci, i), "core_id": core, "ring_index": i + 1, "calendar_year": 1999 + i, "width_microns": width, "boundary_position": float64((i + 1) * 400), "marker_kind": "NONE"}
			if i == 1 {
				obs["anchor_group"] = "anchor-2000"
			}
			observations = append(observations, obs)
		}
	}
	measure := map[string]any{"request_id": "sc-measure-001", "expected_revision": batch.Revision, "actor_id": "operator-sc", "observations": observations}
	batch, _, err = c.postBatch(ctx, "/api/batches/"+batchID+"/observations", measure, http.StatusOK)
	if err != nil {
		return fmt.Errorf("自检测量提交: %w", err)
	}
	validate := map[string]any{"request_id": "sc-validate-001", "expected_revision": batch.Revision, "actor_id": "operator-sc"}
	batch, rawValidation, err := c.postBatch(ctx, "/api/batches/"+batchID+"/validate", validate, http.StatusOK)
	if err != nil {
		return fmt.Errorf("自检质量规则: %w", err)
	}
	if batch.State != domain.StateCorrectionRequired {
		return fmt.Errorf("自检期望 CORRECTION_REQUIRED，实际 %s", batch.State)
	}
	var widthFinding string
	for _, finding := range batch.Findings {
		if finding.Status == domain.FindingOpen && finding.RuleCode == "WIDTH_RANGE" {
			widthFinding = finding.FindingID
			break
		}
	}
	if widthFinding == "" {
		return fmt.Errorf("自检未生成 WIDTH_RANGE 异常")
	}
	_, replayBody, err := c.postBatch(ctx, "/api/batches/"+batchID+"/validate", validate, http.StatusOK)
	if err != nil {
		return fmt.Errorf("自检幂等重放: %w", err)
	}
	if !bytes.Equal(bytes.TrimSpace(rawValidation), bytes.TrimSpace(replayBody)) {
		return fmt.Errorf("幂等重放未返回原始响应")
	}
	correction := map[string]any{"request_id": "sc-correct-001", "expected_revision": batch.Revision, "actor_id": "operator-sc", "items": []map[string]any{{"finding_id": widthFinding, "reason": "重新核对边界后更正录入符号", "replacement": map[string]any{"width_microns": 230}}}}
	batch, _, err = c.postBatch(ctx, "/api/batches/"+batchID+"/corrections", correction, http.StatusOK)
	if err != nil {
		return fmt.Errorf("自检异常整改: %w", err)
	}
	if batch.State != domain.StateReviewReady {
		return fmt.Errorf("整改后期望 REVIEW_READY，实际 %s", batch.State)
	}
	var preflight struct {
		Inspection struct {
			InspectedRevision int64 `json:"inspected_revision"`
			Signable          bool  `json:"signable"`
		} `json:"inspection"`
	}
	if err := c.get(ctx, "/api/batches/"+batchID+"/review-inspection?reviewer_id=reviewer-sc", &preflight); err != nil {
		return fmt.Errorf("自检复核预检: %w", err)
	}
	if !preflight.Inspection.Signable || preflight.Inspection.InspectedRevision != batch.Revision {
		return fmt.Errorf("自检复核预检未绑定当前 revision")
	}
	staleRevision := batch.Revision - 1
	stale := map[string]any{"request_id": "sc-stale-001", "expected_revision": staleRevision, "actor_id": "reviewer-sc", "reviewer_id": "reviewer-sc", "decision": "APPROVE", "note": "证据完整且规则通过", "inspected_revision": staleRevision}
	if _, _, err = c.postBatch(ctx, "/api/batches/"+batchID+"/review", stale, http.StatusConflict); err != nil {
		return fmt.Errorf("自检陈旧 revision: %w", err)
	}
	review := map[string]any{"request_id": "sc-review-001", "expected_revision": batch.Revision, "actor_id": "reviewer-sc", "reviewer_id": "reviewer-sc", "decision": "APPROVE", "note": "已独立核验基线、影像、修订轨迹和规则结果", "inspected_revision": preflight.Inspection.InspectedRevision}
	batch, _, err = c.postBatch(ctx, "/api/batches/"+batchID+"/review", review, http.StatusOK)
	if err != nil {
		return fmt.Errorf("自检独立复核: %w", err)
	}
	if batch.State != domain.StateVerified {
		return fmt.Errorf("自检期望 VERIFIED")
	}
	seal := map[string]any{"request_id": "sc-seal-001", "expected_revision": batch.Revision, "actor_id": "operator-sc"}
	batch, _, err = c.postBatch(ctx, "/api/batches/"+batchID+"/seal", seal, http.StatusOK)
	if err != nil {
		return fmt.Errorf("自检封存: %w", err)
	}
	if batch.State != domain.StateSealed {
		return fmt.Errorf("自检期望 SEALED")
	}
	var manifest domain.Manifest
	if err := c.get(ctx, "/api/batches/"+batchID+"/manifest", &manifest); err != nil {
		return fmt.Errorf("自检读取清单: %w", err)
	}
	if !domain.VerifyManifest(manifest) || manifest.BatchID != batchID || manifest.ManifestDigest == "" {
		return fmt.Errorf("自检封存清单摘要无效")
	}
	return nil
}

func (c checkClient) postBatch(ctx context.Context, path string, body any, expected int) (*domain.DendroBatch, []byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(b))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != expected {
		return nil, raw, fmt.Errorf("%s 返回 %d，期望 %d: %s", path, resp.StatusCode, expected, raw)
	}
	if expected >= 400 {
		return nil, raw, nil
	}
	var decoded struct {
		Batch *domain.DendroBatch `json:"batch"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, raw, err
	}
	if decoded.Batch == nil {
		return nil, raw, fmt.Errorf("响应缺少 batch")
	}
	return decoded.Batch, raw, nil
}

func (c checkClient) get(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s 返回 %d: %s", path, resp.StatusCode, raw)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}
