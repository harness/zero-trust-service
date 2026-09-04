// Copyright 2026 Harness, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package ordering is a reference, in-memory implementation of the run-state the
// execution-ordering verifier delegates to the caller; replace for production.
package ordering

import (
	"sort"
	"sync"
)

// Store tracks completed step FQNs per plan execution ID; safe for concurrent use.
// It keeps two views: raw instance FQNs (audit, matched per instance) and folded
// logical FQNs (lookup), so an ancestor check on a logical node succeeds once any
// instance ran. The SDK supplies the fold.
type Store struct {
	mu        sync.Mutex
	instances map[string]map[string]struct{} // planExecID -> raw instance FQNs
	logical   map[string]map[string]struct{} // planExecID -> folded logical FQNs
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{
		instances: make(map[string]map[string]struct{}),
		logical:   make(map[string]map[string]struct{}),
	}
}

// MarkRan records that instanceFQN has completed under planExecID. logicalFQN is
// its SDK-resolved logical node (equal to instanceFQN for a non-strategy step).
func (s *Store) MarkRan(planExecID, instanceFQN, logicalFQN string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	add(s.instances, planExecID, instanceFQN)
	add(s.logical, planExecID, logicalFQN)
}

// Ran reports whether fqn has completed under planExecID; signature matches
// pipeline.StepRanFunc. Matches an exact instance or logical FQN in either view;
// neither view folds, so an un-run instance never matches on a sibling's run.
func (s *Store) Ran(planExecID, fqn string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.instances[planExecID][fqn]; ok {
		return true
	}
	_, ok := s.logical[planExecID][fqn]
	return ok
}

// Instances returns the sorted raw completed FQNs under planExecID (audit view,
// each strategy instance listed individually).
func (s *Store) Instances(planExecID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	set := s.instances[planExecID]
	out := make([]string, 0, len(set))
	for fqn := range set {
		out = append(out, fqn)
	}
	sort.Strings(out)
	return out
}

// add inserts key into m[planExecID], creating the inner set on demand.
func add(m map[string]map[string]struct{}, planExecID, key string) {
	set := m[planExecID]
	if set == nil {
		set = make(map[string]struct{})
		m[planExecID] = set
	}
	set[key] = struct{}{}
}
