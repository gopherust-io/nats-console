package natsclient

import (
	"io"

	"github.com/gopherust-io/nats-consol/pkg/common/bufpool"
)

func readBodyPooled(r io.Reader) ([]byte, error) {
	buf := bufpool.GetBytes()
	defer func() {
		if cap(buf) <= 1<<20 {
			bufpool.PutBytes(buf)
		}
	}()

	for {
		if len(buf) == cap(buf) {
			next := make([]byte, len(buf), cap(buf)*2)
			copy(next, buf)
			buf = next
		}
		n, err := r.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		if err != nil {
			if err == io.EOF {
				if len(buf) == 0 {
					return nil, nil
				}
				out := make([]byte, len(buf))
				copy(out, buf)
				return out, nil
			}
			return nil, err
		}
	}
}
