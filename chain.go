package htmx

type chain interface {
	Next(hf HandleFunc) HandleFunc
}
