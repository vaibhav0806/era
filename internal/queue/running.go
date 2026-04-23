package queue

import "sync"

// RunningSet tracks taskID → docker container name for live-running tasks,
// plus a "killed" flag set when /cancel docker-kills a task so RunNext knows
// to transition it to 'cancelled' instead of 'failed'. Exported because
// the runner adapter (in internal/runner) needs to Register/Deregister.
type RunningSet struct {
	mu     sync.Mutex
	m      map[int64]string
	killed map[int64]bool
}

func NewRunningSet() *RunningSet {
	return &RunningSet{m: map[int64]string{}, killed: map[int64]bool{}}
}

func (r *RunningSet) Register(id int64, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[id] = name
}

func (r *RunningSet) Deregister(id int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.m, id)
	delete(r.killed, id)
}

func (r *RunningSet) Get(id int64) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name, ok := r.m[id]
	return name, ok
}

func (r *RunningSet) MarkKilled(id int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.killed[id] = true
}

func (r *RunningSet) WasKilled(id int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.killed[id]
}
