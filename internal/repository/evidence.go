package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"tactile-atlas-gate/internal/domain"
)

func verifyImmutableTransition(current, next domain.Aggregate) error {
	if current.Project.Status != domain.StatusDraft {
		before, _ := domain.SpecificationDigest(current.Project)
		after, _ := domain.SpecificationDigest(next.Project)
		if before != after {
			return fmt.Errorf("%w: 已冻结制作规格不可修改", domain.ErrConflict)
		}
	}
	if len(next.Proofs) < len(current.Proofs) {
		return fmt.Errorf("%w: 不得删除已提交校样", domain.ErrConflict)
	}
	for i := range current.Proofs {
		if !reflect.DeepEqual(current.Proofs[i], next.Proofs[i]) {
			return fmt.Errorf("%w: 已提交校样不可修改", domain.ErrConflict)
		}
	}
	if current.Manifest != nil && !reflect.DeepEqual(current.Manifest, next.Manifest) {
		return fmt.Errorf("%w: 发布清单不可修改", domain.ErrConflict)
	}
	return nil
}

func writeNewEvidence(dir string, current, next domain.Aggregate) error {
	proofDir := filepath.Join(dir, "proofs")
	manifestDir := filepath.Join(dir, "manifests")
	specDir := filepath.Join(dir, "specifications")
	if err := os.MkdirAll(proofDir, 0700); err != nil {
		return err
	}
	if err := os.MkdirAll(manifestDir, 0700); err != nil {
		return err
	}
	if err := os.MkdirAll(specDir, 0700); err != nil {
		return err
	}
	if current.Project.Status == domain.StatusDraft && next.Project.Status == domain.StatusFrozen {
		if next.SpecPreflight == nil {
			return fmt.Errorf("%w: 冻结缺少规格预检证据", domain.ErrInvalid)
		}
		evidence := domain.FrozenSpecification(next.Project, *next.SpecPreflight)
		if err := writeJSONExclusive(filepath.Join(specDir, "frozen.json"), evidence); err != nil {
			return err
		}
	}
	for i := len(current.Proofs); i < len(next.Proofs); i++ {
		name := fmt.Sprintf("%06d-%s.json", next.Proofs[i].Sequence, next.Proofs[i].ProofID)
		if err := writeJSONExclusive(filepath.Join(proofDir, name), next.Proofs[i]); err != nil {
			return err
		}
	}
	if current.Manifest == nil && next.Manifest != nil {
		if err := writeJSONExclusive(filepath.Join(manifestDir, next.Manifest.ManifestID+".json"), next.Manifest); err != nil {
			return err
		}
	}
	return nil
}

func writeJSONExclusive(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("不可变证据已存在: %s", filepath.Base(path))
	}
	if err != nil {
		return err
	}
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func verifyEvidence(dir string, aggregate domain.Aggregate) error {
	if aggregate.Project.Status != domain.StatusDraft {
		if aggregate.SpecPreflight == nil {
			return fmt.Errorf("规格预检证据缺失")
		}
		var stored domain.FrozenSpecificationEvidence
		if err := readEvidence(filepath.Join(dir, "specifications", "frozen.json"), &stored); err != nil {
			return err
		}
		expected := domain.FrozenSpecification(aggregate.Project, *aggregate.SpecPreflight)
		if !reflect.DeepEqual(stored, expected) {
			return fmt.Errorf("规格不可变证据与快照不一致")
		}
	}
	for _, proof := range aggregate.Proofs {
		name := fmt.Sprintf("%06d-%s.json", proof.Sequence, proof.ProofID)
		var stored domain.ProofRevision
		if err := readEvidence(filepath.Join(dir, "proofs", name), &stored); err != nil {
			return err
		}
		if !reflect.DeepEqual(stored, proof) {
			return fmt.Errorf("校样不可变证据与快照不一致: %s", proof.ProofID)
		}
	}
	if aggregate.Manifest != nil {
		var stored domain.ReleaseManifest
		if err := readEvidence(filepath.Join(dir, "manifests", aggregate.Manifest.ManifestID+".json"), &stored); err != nil {
			return err
		}
		if !reflect.DeepEqual(stored, *aggregate.Manifest) {
			return fmt.Errorf("发布清单证据与快照不一致")
		}
	}
	return nil
}

func readEvidence(path string, output any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取不可变证据 %s: %w", filepath.Base(path), err)
	}
	if err = json.Unmarshal(b, output); err != nil {
		return fmt.Errorf("不可变证据损坏: %w", err)
	}
	return nil
}
