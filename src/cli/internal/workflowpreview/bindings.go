package workflowpreview

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type Binding struct {
	Kind              string `json:"kind"`
	SubjectRevisionID string `json:"subjectRevisionId,omitempty"`
	ProfileID         string `json:"profileId,omitempty"`
	ProfileRevision   string `json:"profileRevision,omitempty"`
}
type Step struct {
	StepID           string          `json:"stepId"`
	ExecutionBinding Binding         `json:"executionBinding"`
	ModelSelection   *ModelSelection `json:"modelSelection,omitempty"`
}
type ModelSelection struct {
	DefaultModelID string `json:"defaultModelId"`
}
type Bindings struct {
	Steps []Step `json:"steps"`
}
type resolveJob struct {
	sequence string
	bindings Bindings
	key      [32]byte
}

func resolveLoop(ctx context.Context, resolve Resolver, jobs <-chan resolveJob, current *state) {
	var cachedKey [32]byte
	var cached json.RawMessage
	var pending resolveJob
	var retry <-chan time.Time
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case pending = <-jobs:
			timer.Stop()
			retry = nil
		case <-retry:
			retry = nil
		}
		current.RLock()
		valid := current.snapshot.Sequence == pending.sequence
		current.RUnlock()
		if !valid {
			continue
		}
		value := cached
		var err error
		var transient bool
		if resolve == nil {
			err = fmt.Errorf("unavailable")
		} else if cached == nil || cachedKey != pending.key {
			callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			value, err = resolve(callCtx, pending.bindings)
			cancel()
			transient = err == nil && contextDependencyUnavailable(value)
			if err == nil && !transient {
				cachedKey, cached = pending.key, value
			}
		}
		current.change(func(s *Snapshot) {
			if s.Sequence != pending.sequence {
				return
			}
			if err != nil {
				if s.Problem == "" {
					s.Problem = "context_unavailable"
				}
				return
			}
			s.Context = value
			if s.Problem == "context_unavailable" {
				s.Problem = ""
			}
		})
		if (err != nil || transient) && resolve != nil {
			timer.Reset(10 * time.Second)
			retry = timer.C
		}
	}
}

// A partial HTTP 200 projection can still describe a temporary authority outage.
// Keep its resolved ports visible, but do not cache the outage for this session.
func contextDependencyUnavailable(raw json.RawMessage) bool {
	var value struct {
		Steps []struct {
			Reason string `json:"reason"`
		} `json:"steps"`
	}
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	for _, step := range value.Steps {
		if step.Reason == "dependency_unavailable" {
			return true
		}
	}
	return false
}
