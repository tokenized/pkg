package storage

import (
	"context"
	"fmt"
	"testing"
)

func TestS3ListLimit(t *testing.T) {
	t.Skip() // Must be run manually with a valid bucket name

	ctx := context.Background()
	store := NewS3Storage(Config{
		Bucket:     "s3-bucket-name",
		MaxRetries: 10,
		RetryDelay: 100,
	})

	counts := []int64{S3ListLimit / 2, S3ListLimit, S3ListLimit - 1, S3ListLimit + 1,
		S3ListLimit + (S3ListLimit / 2)}
	for _, count := range counts {
		path := fmt.Sprintf("test%d", count)
		keys := make([]string, count)

		// create objects
		for i := int64(0); i < count; i++ {
			key := fmt.Sprintf("%s/key%d", path, i)
			keys[i] = key
			if err := store.Write(ctx, key, []byte(key), nil); err != nil {
				t.Fatalf("Failed to write key %s : %s", key, err)
			}
		}

		t.Logf("Successfully wrote %d s3 items", count)

		// list objects
		list, err := store.List(ctx, path)
		if err != nil {
			t.Fatalf("Failed to list : %s", err)
		}

		if len(list) != int(count) {
			t.Fatalf("Wrong list length : got %d, want %d", len(list), count)
		}

		t.Logf("Successfully listed %d s3 items", count)
	}
}
