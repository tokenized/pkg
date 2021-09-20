package storage

import (
	"bytes"
	"context"
	"io"

	"github.com/pkg/errors"
)

type Serializer interface {
	Serialize(io.Writer) error
}

type Deserializer interface {
	Deserialize(io.Reader) error
}

func Save(ctx context.Context, store Writer, path string, object Serializer) error {
	buf := &bytes.Buffer{}
	if err := object.Serialize(buf); err != nil {
		return errors.Wrap(err, "serialize")
	}

	if err := store.Write(ctx, path, buf.Bytes(), nil); err != nil {
		return errors.Wrap(err, "write")
	}

	return nil
}

func Load(ctx context.Context, store Reader, path string, object Deserializer) error {
	b, err := store.Read(ctx, path)
	if err != nil {
		return errors.Wrap(err, "read")
	}

	if err := object.Deserialize(bytes.NewReader(b)); err != nil {
		return errors.Wrap(err, "deserialize")
	}

	return nil
}
