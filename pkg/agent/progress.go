package agent

import "io"

// CountingReader wraps a Reader and counts bytes read.
// If OnProgress is set, it is called after each Read with the cumulative count.
type CountingReader struct {
	Reader     io.Reader
	N          int64
	OnProgress func(int64)
}

func (cr *CountingReader) Read(p []byte) (int, error) {
	n, err := cr.Reader.Read(p)
	cr.N += int64(n)
	if cr.OnProgress != nil && n > 0 {
		cr.OnProgress(cr.N)
	}
	return n, err
}
