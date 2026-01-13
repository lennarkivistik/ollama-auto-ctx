package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCountingWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	cw := NewCountingWriter(rec)

	// Write some data
	n1, err := cw.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n1 != 5 {
		t.Errorf("Write returned %d, want 5", n1)
	}

	n2, err := cw.Write([]byte(" world"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n2 != 6 {
		t.Errorf("Write returned %d, want 6", n2)
	}

	if cw.BytesWritten() != 11 {
		t.Errorf("BytesWritten = %d, want 11", cw.BytesWritten())
	}

	if cw.StatusCode() != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", cw.StatusCode(), http.StatusOK)
	}
}

func TestCountingWriterStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	cw := NewCountingWriter(rec)

	cw.WriteHeader(http.StatusCreated)
	if cw.StatusCode() != http.StatusCreated {
		t.Errorf("StatusCode = %d, want %d", cw.StatusCode(), http.StatusCreated)
	}
}

func TestCountingReader(t *testing.T) {
	data := []byte("test data to read")
	reader := NewCountingReader(io.NopCloser(bytes.NewReader(data)))

	buf := make([]byte, 5)
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if n != 5 {
		t.Errorf("Read returned %d, want 5", n)
	}
	if reader.BytesRead() != 5 {
		t.Errorf("BytesRead = %d, want 5", reader.BytesRead())
	}

	// Read more
	n, err = reader.Read(buf)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if reader.BytesRead() != 10 {
		t.Errorf("BytesRead = %d, want 10", reader.BytesRead())
	}
}

func TestCountingReadCloserCallback(t *testing.T) {
	data := []byte("test data")
	var callbackBytes int64
	callback := func(n int64) {
		callbackBytes = n
	}

	reader := NewCountingReadCloser(io.NopCloser(bytes.NewReader(data)), callback)

	// Read all
	buf := make([]byte, 100)
	for {
		_, err := reader.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}
	}

	// Close triggers callback
	reader.Close()

	if callbackBytes != int64(len(data)) {
		t.Errorf("callback got %d bytes, want %d", callbackBytes, len(data))
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1023, "1023 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{10240, "10.0 KB"},
		{102400, "100 KB"},
		{1048576, "1.00 MB"},
		{1572864, "1.50 MB"},
		{10485760, "10.0 MB"},
		{1073741824, "1.00 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}
