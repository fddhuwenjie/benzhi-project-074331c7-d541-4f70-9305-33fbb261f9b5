package repository

import (
	"encoding/json"
	"os"
	"path/filepath"

	"tactile-atlas-gate/internal/domain"
)

func writeSnapshot(dir string, a domain.Aggregate) error {
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "snapshot-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(name)
		}
	}()
	if err = f.Chmod(0600); err != nil {
		return err
	}
	if _, err = f.Write(b); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	// 重命名即提交点：rename 成功后新快照已经对后续加载可见。
	if err = os.Rename(name, filepath.Join(dir, "snapshot.json")); err != nil {
		return err
	}
	ok = true
	// 目录 fsync 仅用于崩溃持久化；rename 已提交，失败不得当作保存失败，
	// 否则调用方会误以为可以回滚已经可见的新状态。
	if d, derr := os.Open(dir); derr == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
