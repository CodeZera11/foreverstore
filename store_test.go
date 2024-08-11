package main

import (
	"bytes"
	"io"
	"testing"
)

func TestPathTransformFunc(t *testing.T) {
	key := "momsbestpicture"
	pathKey := CASPathTransformFunc(key)

	expectedOriginal := "6804429f74181a63c50c3d81d733a12f14a353ff"
	expectedPathname := "6804429f/74181a63/c50c3d81/d733a12f/14a353ff"

	if pathKey.Pathname != expectedPathname {
		t.Errorf("have %s want %s", pathKey.Pathname, expectedPathname)
	}
	if pathKey.Filename != expectedOriginal {
		t.Errorf("have %s want %s", pathKey.Filename, expectedOriginal)
	}
}

func TestWriteStream(t *testing.T) {
	opts := StoreOpts{
		PathTransformFunc: CASPathTransformFunc,
	}

	store := NewStore(opts)

	key := "specialPicture"
	data := []byte("some jpg bytes")

	if err := store.writeStream(key, bytes.NewReader(data)); err != nil {
		t.Error(err)
	}

	r, err := store.Read(key)

	if err != nil {
		t.Error(err)
	}

	f, _ := io.ReadAll(r)

	if string(f) != string(data) {
		t.Errorf("have %s want %s", string(f), string(data))
	}
}
