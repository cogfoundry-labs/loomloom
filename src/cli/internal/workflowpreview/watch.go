package workflowpreview

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

func watch(ctx context.Context, path string, current *state, emit func(Event) error) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	var candidate, sent, problem string
	var changed time.Time
	var hasCandidate, hasSent bool
	var sequence uint64
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
				current.change(func(s *Snapshot) {
					s.Problem = ""
					if s.Content != nil && !json.Valid([]byte(*s.Content)) {
						s.Problem = "invalid_json"
					}
				})
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
			current.change(func(s *Snapshot) {
				s.Sequence = seq
				s.Content = &content
				s.Problem = ""
				if !json.Valid([]byte(content)) {
					s.Problem = "invalid_json"
				}
			})

			if err = emit(Event{Type: "synced", Sequence: seq}); err != nil {
				return err
			}
		}
	}
}
