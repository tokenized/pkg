package storage

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
)

func TestRedis_ReadWriteDelete(t *testing.T) {
	ctx := context.Background()

	uri := os.Getenv("REDIS_URL")
	if len(uri) == 0 {
		t.Skip("REDIS_URL not set")
	}

	u, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := redis.Dial("tcp", u.Host)
	if err != nil {
		t.Fatal(err)
	}

	store := NewRedisStorage(conn)

	key := fmt.Sprintf("test-%v", time.Now().UnixNano())
	payload := "hello"

	// write
	if err := store.Write(ctx, key, []byte(payload), nil); err != nil {
		t.Fatal(err)
	}

	// read
	got, err := store.Read(ctx, key)
	if err != nil {
		t.Fatal(err)
	}

	gotString := fmt.Sprintf("%s", got)

	wantString := "hello"

	if gotString != wantString {
		t.Errorf("got %q want %q", gotString, wantString)
	}

	// delete
	if err := store.Remove(ctx, key); err != nil {
		t.Fatal(err)
	}

	// check that item was deleted
	if _, err := store.Read(ctx, key); err != ErrNotFound {
		t.Fatal(err)
	}
}

func TestRedis_List(t *testing.T) {
	ctx := context.Background()

	uri := os.Getenv("REDIS_URL")
	if len(uri) == 0 {
		t.Skip("REDIS_URL not set")
	}

	u, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := redis.Dial("tcp", u.Host)
	if err != nil {
		t.Fatal(err)
	}

	store := NewRedisStorage(conn)

	prefix := "pkg-unit-test"

	testPrefix := fmt.Sprintf("%s/%v", prefix, time.Now().UnixNano())

	keys := []string{}

	for i := 0; i < 10; i++ {
		k := fmt.Sprintf("%s/%v", testPrefix, time.Now().UnixNano())
		keys = append(keys, k)

		if err := store.Write(ctx, k, []byte("foo"), nil); err != nil {
			t.Fatal(err)
		}
	}

	got, err := store.List(ctx, prefix)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, keys) {
		t.Errorf("got\n%v\nwant\n%v", got, keys)
	}

	testKeys, err := store.List(ctx, prefix)
	if err != nil {
		t.Fatal(err)
	}

	// delete keys from any previous unit tests as well
	for _, k := range testKeys {
		if err := store.Remove(ctx, k); err != nil {
			t.Fatal(err)
		}
	}
}
