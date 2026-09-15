package release

import (
	"context"
	"sync"
)

// fakeBackends implements both adapters with in-memory state for tests.
type fakeBackends struct {
	mu        sync.Mutex
	rpcNodes  map[string][]string
	httpNodes map[string]map[string]int
	live      map[string]bool
	readErr   error
	execErr   error
	writes    int
	// diverge makes ReadRoute report nodes that never match the desired
	// state, simulating an out-of-band registry.
	diverge bool
}

func newFakeBackends() *fakeBackends {
	return &fakeBackends{
		rpcNodes:  map[string][]string{},
		httpNodes: map[string]map[string]int{},
		live:      map[string]bool{},
	}
}

func (f *fakeBackends) RegisterRPC(_ context.Context, service string, nodes []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++
	if f.execErr != nil {
		return f.execErr
	}
	f.rpcNodes[service] = append([]string{}, nodes...)
	return nil
}
func (f *fakeBackends) DeregisterRPC(_ context.Context, service string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++
	if f.execErr != nil {
		return f.execErr
	}
	delete(f.rpcNodes, service)
	return nil
}
func (f *fakeBackends) ReadRPC(_ context.Context, service string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.readErr != nil {
		return nil, f.readErr
	}
	return append([]string{}, f.rpcNodes[service]...), nil
}
func (f *fakeBackends) SyncRoute(_ context.Context, routeID, _ string, nodes map[string]int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++
	if f.execErr != nil {
		return f.execErr
	}
	cp := map[string]int{}
	for k, v := range nodes {
		cp[k] = v
	}
	f.httpNodes[routeID] = cp
	f.live[routeID] = true
	return nil
}
func (f *fakeBackends) DisableRoute(_ context.Context, routeID, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++
	if f.execErr != nil {
		return f.execErr
	}
	f.live[routeID] = false
	return nil
}
func (f *fakeBackends) ReadRoute(_ context.Context, routeID string) (map[string]int, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.readErr != nil {
		return nil, false, f.readErr
	}
	if f.diverge {
		return map[string]int{"10.9.9.9:8080": 1}, true, nil
	}
	return f.httpNodes[routeID], f.live[routeID], nil
}
