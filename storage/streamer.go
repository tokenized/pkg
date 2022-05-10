package storage

import (
	"context"
	"sync"

	"github.com/pkg/errors"
)

func StreamWrite(ctx context.Context, store StreamWriter, key string, s Serializer) error {
	var wait sync.WaitGroup
	var writeErr error
	buf := NewBuffer()

	wait.Add(1)
	go func() {
		writeErr = store.StreamWrite(ctx, key, buf)
		wait.Done()
	}()

	serializeErr := s.Serialize(buf)
	buf.Close()
	wait.Wait()

	if serializeErr != nil {
		return errors.Wrap(serializeErr, "serialize")
	}

	if writeErr != nil {
		return errors.Wrap(writeErr, "write")
	}

	return nil
}

func StreamRead(ctx context.Context, store StreamReader, key string, d Deserializer) error {
	r, err := store.StreamRead(ctx, key)
	if err != nil {
		return errors.Wrap(err, "read")
	}

	if err := d.Deserialize(r); err != nil {
		return errors.Wrap(err, "deserialize")
	}

	return nil
}
