package main

type ReadStream struct {
	name     string
	listener map[string][]chan []byte
}

// add Listener
func (b *ReadStream) AddListener(e string, ch chan []byte) {

	if b.listener == nil {
		b.listener = make(map[string][]chan []byte)
	}
	if _, ok := b.listener[e]; ok {
		b.listener[e] = append(b.listener[e], ch)
	} else {
		b.listener[e] = []chan []byte{ch}
	}

}

// Remove listener
func (b *ReadStream) RemoveListener(e string, ch chan []byte) {
	if _, ok := b.listener[e]; ok {
		for i := range b.listener[e] {
			if b.listener[e][i] == ch {
				b.listener[e] = append(b.listener[e][:i], b.listener[e][i+1:]...)
				break
			}
		}
	}
}

// Emit event
func (b *ReadStream) Emit(e string, response []byte) {

	if _, ok := b.listener[e]; ok {
		for _, handler := range b.listener[e] {
			go func(handler chan []byte) {
				handler <- response
			}(handler)
		}
	}
}
