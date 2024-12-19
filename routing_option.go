package htmx

import "reflect"

type RoutingOptions struct {
	metadata map[string]any
	viewer   Viewer
}

func (ro *RoutingOptions) Get(name string) any {
	return ro.metadata[name]
}

func (ro *RoutingOptions) String(name string) string {
	v, ok := ro.metadata[name]
	if !ok {
		return ""
	}

	s, ok := v.(string)
	if !ok {
		return ""
	}

	return s
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

		v := reflect.ValueOf(value)
		if v.IsZero() {
			delete(ro.metadata, key)
		} else {
			ro.metadata[key] = value
		}
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
