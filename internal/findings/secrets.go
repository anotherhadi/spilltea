package findings

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/rs/zerolog"
	"github.com/zricethezav/gitleaks/v8/detect"
	"github.com/zricethezav/gitleaks/v8/logging"

	"github.com/anotherhadi/spilltea/internal/message"
)

const cacheSize = 4096

type Secrets struct {
	d     *detect.Detector
	cache map[[sha256.Size]byte][]Hit
}

func NewSecrets() (*Secrets, error) {
	// gitleaks logs to stderr
	logging.Logger = zerolog.Nop()
	d, err := detect.NewDetectorDefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("gitleaks: %w", err)
	}
	return &Secrets{d: d, cache: map[[sha256.Size]byte][]Hit{}}, nil
}

func (s *Secrets) Name() string { return "secrets" }

func (s *Secrets) Detect(_ *message.Request, resp *message.Response) []Hit {
	if resp == nil || len(resp.Body) == 0 || binary(resp) {
		return nil
	}
	sum := sha256.Sum256(resp.Body)
	if hits, ok := s.cache[sum]; ok {
		return hits
	}
	var hits []Hit
	for _, f := range s.d.DetectBytes(resp.Body) {
		secret := f.Secret
		if secret == "" {
			secret = f.Match
		}
		key := sha256.Sum256([]byte(secret))
		hits = append(hits, Hit{
			Key:    f.RuleID + ":" + hex.EncodeToString(key[:8]),
			Title:  f.RuleID,
			Detail: fmt.Sprintf("%s\nline %d: %s", short(secret), f.StartLine+1, short(f.Match)),
		})
	}
	if len(s.cache) >= cacheSize {
		clear(s.cache)
	}
	s.cache[sum] = hits
	return hits
}

var binaryTypes = []string{"image/", "font/", "video/", "audio/"}

// Same heuristic as git: a NUL in the first 8000 bytes.
func binary(resp *message.Response) bool {
	mime := message.Mime(resp.Header.Get("Content-Type"))
	for _, t := range binaryTypes {
		if strings.HasPrefix(mime, t) {
			return true
		}
	}
	return bytes.IndexByte(resp.Body[:min(len(resp.Body), 8000)], 0) >= 0
}

func short(s string) string { return ansi.Truncate(s, 300, "…") }
