package service

import (
	"strings"

	"sync.gateway/internal/config"
)

// Registry holds registered services and provides lookup methods.
type Registry struct {
	services []config.ServiceConfig
}

// New creates a new empty Registry.
func New() *Registry {
	return &Registry{
		services: make([]config.ServiceConfig, 0),
	}
}

// Add adds services to the registry.
func (r *Registry) Add(services ...config.ServiceConfig) {
	r.services = append(r.services, services...)
}

// Clear removes all services from the registry.
func (r *Registry) Clear() {
	r.services = make([]config.ServiceConfig, 0)
}

// All returns all registered services.
func (r *Registry) All() []config.ServiceConfig {
	return r.services
}

// FindForPath finds the service that handles the given path.
// Returns the service alias and upstream URL, or empty strings if not found.
func (r *Registry) FindForPath(path string) (alias, url string) {
	for _, service := range r.services {
		if matchAlias(path, service.Alias) {
			return service.Alias, service.URL
		}
	}
	return "", ""
}

// FindServiceForPath finds the full service config for the given path.
// Returns nil if no matching service is found.
// This is used by the auth middleware to check auth requirements.
func (r *Registry) FindServiceForPath(path string) *config.ServiceConfig {
	for i := range r.services {
		if matchAlias(path, r.services[i].Alias) {
			return &r.services[i]
		}
	}
	return nil
}

// matchAlias checks if a path matches a service alias.
func matchAlias(path, alias string) bool {
	alias = strings.TrimSpace(alias)
	if alias == "" || alias == "/" {
		return false
	}
	base := strings.TrimSuffix(alias, "/")
	if path == base {
		return true
	}
	return strings.HasPrefix(path, base+"/")
}
