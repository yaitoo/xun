package htmx

type RoutingOptions struct {
	metadata map[string]any
	viewer   Viewer
}

func (ro *RoutingOptions) Get(name string) any {
	return ro.metadata[name]
}

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

type RoutingOption func(*RoutingOptions)

const (
	NavigationName   = "name"
	NavigationIcon   = "icon"
	NavigationAccess = "access"
)

func WithMetadata(key string, value any) RoutingOption {
	return func(ro *RoutingOptions) {
		if ro.metadata == nil {
			ro.metadata = make(map[string]any)
		}

		ro.metadata[key] = value
	}
}

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
