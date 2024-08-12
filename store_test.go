package main

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

func newStore() *Store {
	opts := StoreOpts{
		PathTransformFunc: CASPathTransformFunc,
		Root:              "customroot",
	}

	store := NewStore(opts)

	return store
}

func teardown(t *testing.T, s *Store) {
	if err := s.Clear(); err != nil {
		t.Error(err)
	}
}

func TestPathTransformFunc(t *testing.T) {
	key := "momsbestpicture"
	pathKey := CASPathTransformFunc(key)

	expectedFilename := "6804429f74181a63c50c3d81d733a12f14a353ff"
	expectedPathname := "6804429f/74181a63/c50c3d81/d733a12f/14a353ff"

	if pathKey.Pathname != expectedPathname {
		t.Errorf("have %s want %s", pathKey.Pathname, expectedPathname)
	}
	if pathKey.Filename != expectedFilename {
		t.Errorf("have %s want %s", pathKey.Filename, expectedFilename)
	}
}

func TestStore(t *testing.T) {
	store := newStore()

	defer teardown(t, store)

	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("foo_%d", i)
		data := []byte("some jpg bytes")

		err := store.writeStream(key, bytes.NewReader(data))

		if err != nil {
			t.Error(err)
		}

		fmt.Printf("Successfully wrote data to key %s", key)

		if ok := store.Has(key); !ok {
			t.Errorf("Expected to have key %s", key)
		}

		r, err := store.Read(key)
		if err != nil {
			t.Error(err)
		}

		dataFromDisk, err := io.ReadAll(r)
		if err != nil {
			t.Error(err)
		}

		if string(dataFromDisk) != string(data) {
			fmt.Printf("Incorrect data!")
		}

		err = store.Delete(key)

		if err != nil {
			t.Error(err)
		}

		if ok := store.Has(key); ok {
			t.Errorf("Expected to NOT have key %s", key)
		}
	}
}
