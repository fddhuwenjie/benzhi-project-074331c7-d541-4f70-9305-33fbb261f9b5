package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func Digest(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func SpecificationDigest(project MapProject) (string, error) {
	project.Status = ""
	project.Revision = 0
	return Digest(project)
}
func ManifestDigest(m ReleaseManifest) (string, error) { m.CanonicalDigest = ""; return Digest(m) }
func VerifyManifest(m ReleaseManifest) bool {
	d, err := ManifestDigest(m)
	return err == nil && d == m.CanonicalDigest
}
