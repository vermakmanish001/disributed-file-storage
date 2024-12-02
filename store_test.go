package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"
)

func TestPathTransformFunc(t *testing.T) {

	key := "bestpicture"
	pathKey := CASPathTransformFunc(key)
	expectedFileName := "71056ad8aa24742ea41ea36fa2e3452a31636e82"
	expectedPathName := "71056/ad8aa/24742/ea41e/a36fa/2e345/2a316/36e82"
	if pathKey.PathName != expectedPathName {
		t.Errorf("have %s want %s", pathKey.PathName, expectedPathName)
	}
	if pathKey.Filename != expectedFileName {
		t.Errorf("have %s want %s", pathKey.Filename, expectedFileName)
	}

}

func TestStore(t *testing.T) {
	s := newStore()
	defer teardown(t, s)

	t.Log("Starting TestStore")

	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("foo_%d", i)
		data := []byte("some jpg bytes")

		t.Logf("Writing key: %s", key)
		if err := s.writeStream(key, bytes.NewReader(data)); err != nil {
			t.Fatalf("Failed to write stream for key %s: %v", key, err)
		}

		t.Logf("Checking existence of key: %s", key)
		if ok := s.Has(key); !ok {
			t.Errorf("Expected to have key %s", key)
		}

		t.Logf("Reading key: %s", key)
		r, err := s.Read(key)
		if err != nil {
			t.Fatalf("Failed to read key %s: %v", key, err)
		}

		b, _ := ioutil.ReadAll(r)
		if string(b) != string(data) {
			t.Errorf("Expected %s but got %s", data, b)
		}

		t.Logf("Deleting key: %s", key)
		if err := s.Delete(key); err != nil {
			t.Fatalf("Failed to delete key %s: %v", key, err)
		}

		if ok := s.Has(key); ok {
			t.Errorf("Expected NOT to have key %s", key)
		}
	}
}

func newStore() *Store {
	opts := StoreOpts{
		PathTransformFunc: CASPathTransformFunc,
	}
	return NewStore(opts)
}

func teardown(t *testing.T, s *Store) {
	if err := s.Clear(); err != nil {
		t.Error(err)
	}
}
