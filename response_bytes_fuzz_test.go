package httpclient

import (
	"bytes"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
)

func FuzzReadResponseBytes(f *testing.F) {
	f.Add([]byte(""), byte(0))
	f.Add([]byte{0, 0xff, '\n'}, byte(2))
	f.Add([]byte("exactly!"), byte(7))
	f.Add([]byte("overflow!"), byte(7))
	f.Fuzz(func(t *testing.T, payload []byte, capacity byte) {
		if len(payload) > 4096 {
			payload = payload[:4096]
		}
		maximum := 1 + int(capacity%64)
		var closed atomic.Int64
		reader := &rawByteCounter{Reader: bytes.NewReader(payload)}
		response := &http.Response{Body: &responseTestBody{Reader: reader, closed: &closed}}
		body, err := ReadResponseBytes(response, ReadOptions{MaximumBodyBytes: int64(maximum)})
		if len(payload) <= maximum {
			if err != nil || !bytes.Equal(body, payload) {
				t.Fatal("bounded raw bytes changed")
			}
		} else if !errors.Is(err, ErrResponseBodyLimit) || body != nil {
			t.Fatal("overflow admitted or retained partial bytes")
		}
		if closed.Load() != 1 || reader.read > maximum+1 {
			t.Fatal("body cleanup or one-byte probe bound changed")
		}
	})
}
