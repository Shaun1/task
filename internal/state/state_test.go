package state

import (
	"errors"
	"testing"
	"time"
)

func TestStoreRoundTrip(t *testing.T) {
	s := Store{Dir: t.TempDir() + "/state"}
	if tasks, err := s.List(); err != nil || len(tasks) != 0 {
		t.Fatalf("empty list: %v %v", tasks, err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	a := Task{Name: "a", Repo: "r", CreatedAt: now}
	b := Task{Name: "b", Repo: "r", CreatedAt: now.Add(-time.Hour)}
	for _, x := range []Task{a, b} {
		if err := s.Save(x); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Load("a")
	if err != nil || got.Name != "a" || !got.CreatedAt.Equal(now) {
		t.Fatalf("load: %+v %v", got, err)
	}
	tasks, _ := s.List()
	if len(tasks) != 2 || tasks[0].Name != "b" {
		t.Fatalf("list order: %+v", tasks)
	}
	if err := s.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("a"); err != nil {
		t.Fatalf("double delete: %v", err)
	}
	if _, err := s.Load("a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
