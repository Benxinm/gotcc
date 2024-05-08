package manager

import (
	"errors"
	"fmt"
	"gotcc/core/component"
	"sync"
)

type registerCenter struct {
	mu         sync.RWMutex
	components map[string]component.TCCComponent
}

func newRegisterCenter() *registerCenter {
	return &registerCenter{
		components: make(map[string]component.TCCComponent),
	}
}

func (r *registerCenter) register(component component.TCCComponent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.components[component.ID()]; ok {
		return errors.New("repeat component id")
	}
	r.components[component.ID()] = component
	return nil
}

func (r *registerCenter) getComponents(componentIDs ...string) ([]component.TCCComponent, error) {
	components := make([]component.TCCComponent, 0, len(componentIDs))
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, componentID := range componentIDs {
		component, ok := r.components[componentID]
		if !ok {
			return nil, fmt.Errorf("component id: %s not existed", componentID)
		}
		components = append(components, component)
	}
	return components, nil
}
