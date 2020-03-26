package renderer

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/fmpwizard/go-quilljs-delta/delta"
)

func debugDeltaString(t *testing.T, d delta.Delta) string {
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestNormalText(t *testing.T) {
	c := make(chan delta.Delta)
	r := NewRenderer(c)

	sc := make(chan bool)
	ec := make(chan error)

	go func() {
		_, err := r.Write([]byte("abcdefg"))
		if err != nil {
			ec <- err
		} else {
			sc <- true
		}
	}()

	change := <-c
	select {
	case <-sc:
	case err := <-ec:
		t.Fatal(err)
	}
	expectedChange := *delta.New(nil).Insert("abcdefg", nil)
	if !reflect.DeepEqual(change, expectedChange) {
		t.Fatalf("Invalid change, expected: %s, actual: %s",
			debugDeltaString(t, expectedChange), debugDeltaString(t, change))
	}
}

func TestReadPartialRune(t *testing.T) {
	c := make(chan delta.Delta)
	r := NewRenderer(c)

	sc := make(chan bool)
	ec := make(chan error)

	go func() {
		_, err := r.Write([]byte("\xe4\xb8"))
		if err != nil {
			ec <- err
		} else {
			sc <- true
		}
	}()

	select {
	case <-sc:
	case err := <-ec:
		t.Fatal(err)
	}

	go func() {
		_, err := r.Write([]byte("\x96"))
		if err != nil {
			ec <- err
		} else {
			sc <- true
		}
	}()

	change := <-c
	select {
	case <-sc:
	case err := <-ec:
		t.Fatal(err)
	}
	expectedChange := *delta.New(nil).Insert("世", nil)
	if !reflect.DeepEqual(change, expectedChange) {
		t.Fatalf("Invalid change, expected: %s, actual: %s",
			debugDeltaString(t, expectedChange), debugDeltaString(t, change))
	}
}

func TestInvalidRune(t *testing.T) {
	c := make(chan delta.Delta)
	r := NewRenderer(c)

	sc := make(chan bool)
	ec := make(chan error)

	go func() {
		_, err := r.Write([]byte("\xb4\xb4\xb4\xb4"))
		if err != nil {
			ec <- err
		} else {
			sc <- true
		}
	}()

	select {
	case <-sc:
		t.Fatal("An error should be generated!")
	case <-ec:
	}
}

func TestUnrecognizedEscape(t *testing.T) {
	c := make(chan delta.Delta)
	r := NewRenderer(c)

	sc := make(chan bool)
	ec := make(chan error)

	go func() {
		_, err := r.Write([]byte("hello\x1bVworld"))
		if err != nil {
			ec <- err
		} else {
			sc <- true
		}
	}()

	change := <-c
	change = *change.Concat(<-c)
	select {
	case <-sc:
	case err := <-ec:
		t.Fatal(err)
	}
	expectedChange := *delta.New(nil).Insert("hello", nil).Insert("world", nil)
	if !reflect.DeepEqual(change, expectedChange) {
		t.Fatalf("Invalid change, expected: %s, actual: %s",
			debugDeltaString(t, expectedChange), debugDeltaString(t, change))
	}
}

func TestSetColor(t *testing.T) {
	c := make(chan delta.Delta)
	r := NewRenderer(c)

	sc := make(chan bool)
	ec := make(chan error)

	go func() {
		_, err := r.Write([]byte("hello\x1b[31mworld1\x1b[1;33mworld2\x1b[1;35;1mworld3\x1b[0mworld"))
		if err != nil {
			ec <- err
		} else {
			sc <- true
		}
	}()

	change := <-c
	change = *change.Concat(<-c)
	change = *change.Concat(<-c)
	change = *change.Concat(<-c)
	change = *change.Concat(<-c)
	select {
	case <-sc:
	case err := <-ec:
		t.Fatal(err)
	}
	expectedChange := *delta.New(nil).Insert("hello", nil).
		Insert("world1", map[string]interface{}{
			"color": "#cd0000",
		}).
		Insert("world2", map[string]interface{}{
			"color": "#ffff00",
		}).
		Insert("world3", map[string]interface{}{
			"bold":  true,
			"color": "#ff00ff",
		}).
		Insert("world", nil)
	if !reflect.DeepEqual(change, expectedChange) {
		t.Fatalf("Invalid change, expected: %s, actual: %s",
			debugDeltaString(t, expectedChange), debugDeltaString(t, change))
	}
}

func TestSetRichColor(t *testing.T) {
	c := make(chan delta.Delta)
	r := NewRenderer(c)

	sc := make(chan bool)
	ec := make(chan error)

	go func() {
		_, err := r.Write([]byte("hello\x1b[38;5;7ma\x1b[38;5;12mb\x1b[38;5;107mc\x1b[38;5;247md\x1b[38;2;409;10;99me"))
		if err != nil {
			ec <- err
		} else {
			sc <- true
		}
	}()

	change := <-c
	change = *change.Concat(<-c)
	change = *change.Concat(<-c)
	change = *change.Concat(<-c)
	change = *change.Concat(<-c)
	change = *change.Concat(<-c)
	select {
	case <-sc:
	case err := <-ec:
		t.Fatal(err)
	}
	expectedChange := *delta.New(nil).Insert("hello", nil).
		Insert("a", map[string]interface{}{
			"color": "#e5e5e5",
		}).
		Insert("b", map[string]interface{}{
			"color": "#5c5cff",
		}).
		Insert("c", map[string]interface{}{
			"color": "#87af5f",
		}).
		Insert("d", map[string]interface{}{
			"color": "#9e9e9e",
		}).
		Insert("e", map[string]interface{}{
			"color": "#990a63",
		})
	if !reflect.DeepEqual(change, expectedChange) {
		t.Fatalf("Invalid change, expected: %s, actual: %s",
			debugDeltaString(t, expectedChange), debugDeltaString(t, change))
	}
}
