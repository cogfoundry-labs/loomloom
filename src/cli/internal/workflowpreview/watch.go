package workflowpreview

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

func watch(ctx context.Context, path string, current *state, jobs chan resolveJob, emit func(Event) error) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	var candidate, sent, problem string
	var changed time.Time
	var hasCandidate, hasSent bool
	var sequence uint64
	var previousKey [32]byte
	var previousValid bool
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			content, err := readFile(path)
			if err != nil {
				hasCandidate = false
				if err.Error() != problem {
					problem = err.Error()
					current.change(func(s *Snapshot) { s.Problem = problem })
					if err = emit(Event{Type: "waiting", Code: problem}); err != nil {
						return err
					}
				}
				continue
			}
			if problem != "" {
				problem = ""
				current.change(func(s *Snapshot) { s.Problem = "" })
			}
			if !hasCandidate || candidate != content {
				candidate, changed, hasCandidate = content, now, true
				continue
			}
			if now.Sub(changed) < 300*time.Millisecond || hasSent && sent == candidate {
				continue
			}
			if sequence == ^uint64(0) {
				return fmt.Errorf("preview sequence exhausted")
			}
			sequence++
			seq := strconv.FormatUint(sequence, 10)
			sent, hasSent = candidate, true
			var bindings Bindings
			valid := json.Unmarshal([]byte(content), &bindings) == nil && bindings.Steps != nil && len(bindings.Steps) <= 200
			raw, _ := json.Marshal(bindings)
			valid = valid && len(raw) <= 128*1024
			key := sha256.Sum256(raw)
			current.change(func(s *Snapshot) {
				s.Sequence = seq
				s.Content = &content
				s.Problem = ""
				if !valid || !previousValid || previousKey != key {
					s.Context = nil
				}
			})
			previousKey, previousValid = key, valid
			if valid {
				// The typed whitelist excludes prompts, inputs and workbook content.
				select {
				case <-jobs:
				default:
				}
				jobs <- resolveJob{seq, bindings, key}
			}

			if err = emit(Event{Type: "synced", Sequence: seq}); err != nil {
				return err
			}
		}
	}
}
