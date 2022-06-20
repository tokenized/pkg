package storage

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

func Test_S3_ListLimit(t *testing.T) {
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

type testStreamObject struct {
	value string
}

func (t testStreamObject) Serialize(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(t.value))); err != nil {
		return errors.Wrap(err, "size")
	}

	if _, err := w.Write([]byte(t.value)); err != nil {
		return errors.Wrap(err, "value")
	}

	return nil
}

func (t *testStreamObject) Deserialize(r io.Reader) error {
	size := uint32(0)
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return errors.Wrap(err, "size")
	}

	v := make([]byte, size)
	if _, err := io.ReadFull(r, v); err != nil {
		return errors.Wrap(err, "value")
	}
	t.value = string(v)

	return nil
}

func Test_S3_Stream(t *testing.T) {
	t.Skip() // Must be run manually with a valid bucket name

	ctx := context.Background()
	store := NewS3Storage(Config{
		Bucket:     "s3-bucket-name",
		MaxRetries: 10,
		RetryDelay: 100,
	})

	object := testStreamObject{
		value: uuid.New().String(),
	}

	if err := StreamWrite(ctx, store, "test-value-name", object); err != nil {
		t.Fatalf("Failed to stream write object : %s", err)
	}

	readObject := &testStreamObject{}
	if err := StreamRead(ctx, store, "test-value-name", readObject); err != nil {
		t.Fatalf("Failed to stream read object : %s", err)
	}

	t.Logf("Read : %s", readObject.value)

	if readObject.value != object.value {
		t.Errorf("Wrong read value : got %s, want %s", readObject.value, object.value)
	}
}
