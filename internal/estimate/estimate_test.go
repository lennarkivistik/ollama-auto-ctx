package estimate

import "testing"

func TestBucketize(t *testing.T) {
	buckets := []int{2048, 4096, 8192}
	if got := Bucketize(1, buckets); got != 2048 {
		t.Fatalf("expected 2048, got %d", got)
	}
	if got := Bucketize(2048, buckets); got != 2048 {
		t.Fatalf("expected 2048, got %d", got)
	}
	if got := Bucketize(2049, buckets); got != 4096 {
		t.Fatalf("expected 4096, got %d", got)
	}
	if got := Bucketize(9000, buckets); got != 9000 {
		t.Fatalf("expected 9000, got %d", got)
	}
}

func TestClampCtx(t *testing.T) {
	if got := ClampCtx(1000, 2048, 8192); got != 2048 {
		t.Fatalf("expected 2048, got %d", got)
	}
	if got := ClampCtx(9000, 2048, 8192); got != 8192 {
		t.Fatalf("expected 8192, got %d", got)
	}
	if got := ClampCtx(4096, 2048, 8192); got != 4096 {
		t.Fatalf("expected 4096, got %d", got)
	}
}
