package xun

type chain interface {
	Next(hf HandleFunc) HandleFunc
}
