package fsnotify

type Event struct {
	// Path to the file or directory.
	Name string

	// File operation that triggered the event.
	Op Op
}

// Has reports if this event has the given operation.
func (e Event) Has(op Op) bool { return e.Op.Has(op) }

type Op uint32

const (
	Create Op = 1 << iota

	Write

	Remove
)

// Has reports if this operation has the given operation.
func (o Op) Has(h Op) bool { return o&h != 0 }
