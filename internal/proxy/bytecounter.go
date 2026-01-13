package proxy

import (
	"io"
	"net/http"
	"sync/atomic"
)

// CountingWriter wraps an http.ResponseWriter to count bytes written.
type CountingWriter struct {
	http.ResponseWriter
	bytesWritten int64
	statusCode   int
}

// NewCountingWriter creates a new CountingWriter.
func NewCountingWriter(w http.ResponseWriter) *CountingWriter {
	return &CountingWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// Write implements io.Writer.
func (w *CountingWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	atomic.AddInt64(&w.bytesWritten, int64(n))
	return n, err
}

// WriteHeader implements http.ResponseWriter.
func (w *CountingWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// BytesWritten returns the total bytes written.
func (w *CountingWriter) BytesWritten() int64 {
	return atomic.LoadInt64(&w.bytesWritten)
}

// StatusCode returns the HTTP status code.
func (w *CountingWriter) StatusCode() int {
	return w.statusCode
}

// Flush implements http.Flusher if the underlying writer supports it.
func (w *CountingWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// CountingReader wraps an io.ReadCloser to count bytes read.
type CountingReader struct {
	io.ReadCloser
	bytesRead int64
}

// NewCountingReader creates a new CountingReader.
func NewCountingReader(r io.ReadCloser) *CountingReader {
	return &CountingReader{ReadCloser: r}
}

// Read implements io.Reader.
func (r *CountingReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	atomic.AddInt64(&r.bytesRead, int64(n))
	return n, err
}

// BytesRead returns the total bytes read.
func (r *CountingReader) BytesRead() int64 {
	return atomic.LoadInt64(&r.bytesRead)
}

// CountingReadCloser wraps an io.ReadCloser and calls a callback on close.
type CountingReadCloser struct {
	io.ReadCloser
	bytesRead int64
	onClose   func(bytesRead int64)
}

// NewCountingReadCloser creates a reader that reports bytes on close.
func NewCountingReadCloser(r io.ReadCloser, onClose func(bytesRead int64)) *CountingReadCloser {
	return &CountingReadCloser{
		ReadCloser: r,
		onClose:    onClose,
	}
}

// Read implements io.Reader.
func (r *CountingReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	atomic.AddInt64(&r.bytesRead, int64(n))
	return n, err
}

// Close implements io.Closer and calls the onClose callback.
func (r *CountingReadCloser) Close() error {
	err := r.ReadCloser.Close()
	if r.onClose != nil {
		r.onClose(atomic.LoadInt64(&r.bytesRead))
	}
	return err
}

// BytesRead returns the total bytes read.
func (r *CountingReadCloser) BytesRead() int64 {
	return atomic.LoadInt64(&r.bytesRead)
}

// FormatBytes formats bytes to a human-readable string (B/KB/MB/GB).
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return formatFloat(float64(bytes)/float64(GB)) + " GB"
	case bytes >= MB:
		return formatFloat(float64(bytes)/float64(MB)) + " MB"
	case bytes >= KB:
		return formatFloat(float64(bytes)/float64(KB)) + " KB"
	default:
		return formatInt(bytes) + " B"
	}
}

func formatFloat(f float64) string {
	if f >= 100 {
		return formatInt(int64(f))
	}
	if f >= 10 {
		return floatToString(f, 1)
	}
	return floatToString(f, 2)
}

func formatInt(i int64) string {
	s := ""
	if i < 0 {
		s = "-"
		i = -i
	}
	str := ""
	for i > 0 {
		str = string('0'+byte(i%10)) + str
		i /= 10
	}
	if str == "" {
		str = "0"
	}
	return s + str
}

func floatToString(f float64, decimals int) string {
	if decimals <= 0 {
		return formatInt(int64(f + 0.5))
	}

	mult := 1.0
	for i := 0; i < decimals; i++ {
		mult *= 10
	}

	rounded := int64(f*mult + 0.5)
	whole := rounded / int64(mult)
	frac := rounded % int64(mult)

	fracStr := ""
	for i := 0; i < decimals; i++ {
		fracStr = string('0'+byte(frac%10)) + fracStr
		frac /= 10
	}

	return formatInt(whole) + "." + fracStr
}
