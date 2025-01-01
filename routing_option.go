package xun

// RoutingOptions holds metadata and a viewer for routing configuration.
type RoutingOptions struct {
	metadata map[string]any
	viewer   Viewer
}

// Get returns the value associated with the given name from the routing metadata.
// If the name does not exist, it returns nil.
func (ro *RoutingOptions) Get(name string) any {
	return ro.metadata[name]
}

// GetString returns the value associated with the given name from the routing
// metadata as a string. If the name does not exist, it returns an empty string.
func (ro *RoutingOptions) GetString(name string) string {
	it, ok := ro.metadata[name]
	if !ok {
		return ""
	}

	s, ok := it.(string)
	if !ok {
		return ""
	}

	return s
}

// GetInt returns the value associated with the given name from the routing
// metadata as an integer. If the name does not exist, it returns 0.
func (ro *RoutingOptions) GetInt(name string) int {
	it, ok := ro.metadata[name]
	if !ok {
		return 0
	}

	v, ok := it.(int)
	if !ok {
		return 0
	}

	return v
}

// RoutingOption is a function that takes a pointer to RoutingOptions and
// modifies it. It is used to customize the behavior of the router when
// adding routes.
type RoutingOption func(*RoutingOptions)

const (
	NavigationName   = "name"
	NavigationIcon   = "icon"
	NavigationAccess = "access"
)

// WithMetadata adds a key-value pair to the routing metadata.
// It creates a new map if the metadata map is nil.
func WithMetadata(key string, value any) RoutingOption {
	return func(ro *RoutingOptions) {
		if ro.metadata == nil {
			ro.metadata = make(map[string]any)
		}

		ro.metadata[key] = value
	}
}

// WithNavigation adds navigation-related metadata to the routing options.
// It sets the name, icon, and access level for the navigation element.
func WithNavigation(name, icon, access string) RoutingOption {
	return func(ro *RoutingOptions) {
		if ro.metadata == nil {
			ro.metadata = make(map[string]any)
		}

		ro.metadata[NavigationName] = name
		ro.metadata[NavigationIcon] = icon
		ro.metadata[NavigationAccess] = access
	}
}
