package storage

import (
	"context"
	"sync"

	"github.com/tokenized/threads"

	"github.com/pkg/errors"
)

func StreamWrite(ctx context.Context, store StreamWriter, key string, s Serializer) error {
	buf := NewBuffer()

	var wait sync.WaitGroup

	writeThread := threads.NewUninterruptableThread("Stream Write",
		func(ctx context.Context) error {
			return store.StreamWrite(ctx, key, buf)
		})
	writeThread.SetWait(&wait)
	writeThread.Start(ctx)

	serializeErr := s.Serialize(buf)
	if serializeErr != nil {
		buf.Close()
		return errors.Wrap(serializeErr, "serialize")
	}

	buf.Close()
	wait.Wait()

	if err := writeThread.Error(); err != nil {
		return errors.Wrap(err, "write")
	}

	return nil
}

func StreamRead(ctx context.Context, store StreamReader, key string, d Deserializer) error {
	r, err := store.StreamRead(ctx, key)
	if err != nil {
		return errors.Wrap(err, "read")
	}
	defer r.Close()

	if err := d.Deserialize(r); err != nil {
		return errors.Wrap(err, "deserialize")
	}

	return nil
}
