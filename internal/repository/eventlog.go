package repository

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type eventFrame struct {
	Sequence       int64           `json:"sequence"`
	Length         int             `json:"length"`
	PreviousDigest string          `json:"previous_digest"`
	Payload        json.RawMessage `json:"payload"`
	Digest         string          `json:"digest"`
}

func frameDigest(seq int64, length int, prev string, payload []byte) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d:%d:%s:", seq, length, prev)
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}
func readFrames(path string) ([]eventFrame, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			return i + 1, data[:i], nil
		}
		if atEOF && len(data) != 0 {
			return 0, nil, fmt.Errorf("事件日志被截断")
		}
		return 0, nil, nil
	})
	out := []eventFrame{}
	prev := ""
	for scanner.Scan() {
		line := scanner.Bytes()
		var fr eventFrame
		if json.Unmarshal(line, &fr) != nil {
			return nil, fmt.Errorf("事件帧 JSON 损坏")
		}
		if fr.Sequence != int64(len(out)+1) || fr.Length != len(fr.Payload) || fr.PreviousDigest != prev || fr.Digest != frameDigest(fr.Sequence, fr.Length, fr.PreviousDigest, fr.Payload) {
			return nil, fmt.Errorf("事件摘要链校验失败")
		}
		out = append(out, fr)
		prev = fr.Digest
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取事件帧: %w", err)
	}
	return out, nil
}
func verifyEvents(dir string) error {
	_, err := readFrames(filepath.Join(dir, "events.log"))
	return err
}
func appendEvent(dir string, event Event) error {
	path := filepath.Join(dir, "events.log")
	frames, err := readFrames(path)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	prev := ""
	if len(frames) > 0 {
		prev = frames[len(frames)-1].Digest
	}
	fr := eventFrame{Sequence: int64(len(frames) + 1), Length: len(payload), PreviousDigest: prev, Payload: payload}
	fr.Digest = frameDigest(fr.Sequence, fr.Length, fr.PreviousDigest, fr.Payload)
	line, err := json.Marshal(fr)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(line); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
