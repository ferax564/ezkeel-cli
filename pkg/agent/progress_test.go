package agent

import (
	"bytes"
	"io"
	"testing"
)

func TestCountingReader(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 1024)
	cr := &CountingReader{Reader: bytes.NewReader(data)}

	buf := make([]byte, 256)
	total := 0
	for {
		n, err := cr.Read(buf)
		total += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if total != 1024 {
		t.Errorf("total bytes read = %d, want 1024", total)
	}
	if cr.N != 1024 {
		t.Errorf("CountingReader.N = %d, want 1024", cr.N)
	}
}

func TestCountingReader_CallsOnProgress(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 512)
	var lastN int64
	cr := &CountingReader{
		Reader: bytes.NewReader(data),
		OnProgress: func(n int64) {
			lastN = n
		},
	}

	io.Copy(io.Discard, cr)

	if lastN != 512 {
		t.Errorf("last OnProgress call got %d, want 512", lastN)
	}
}
