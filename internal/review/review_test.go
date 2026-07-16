package review

import (
	"os"
	"path/filepath"
	"testing"
)

func TestList_MissingDirReturnsEmptyNotError(t *testing.T) {
	dir := t.TempDir()
	got, err := List(dir)
	if err != nil {
		t.Fatalf("List on missing reviews/: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no reviews, got %+v", got)
	}
}

func TestAddThenList(t *testing.T) {
	dir := t.TempDir()
	r := Review{
		ID:        "r-1",
		RecordRef: RecordRef{Type: RecordTypeTransition, ID: "T-1"},
		Body:      "why",
		Source:    SourceAI,
		CreatedAt: "2026-01-01T00:00:00Z",
	}
	if err := Add(dir, r); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "reviews", "r-1.json")); err != nil {
		t.Fatalf("reviews/r-1.json not written: %v", err)
	}

	got, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0] != r {
		t.Fatalf("List = %+v, want [%+v]", got, r)
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	r := Review{
		ID:        "r-1",
		RecordRef: RecordRef{Type: RecordTypeTransition, ID: "T-1"},
		Body:      "why",
		Source:    SourceAI,
		CreatedAt: "2026-01-01T00:00:00Z",
	}
	if err := Add(dir, r); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := Get(dir, "r-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != r {
		t.Fatalf("Get = %+v, want %+v", got, r)
	}
}

func TestGet_MissingIsError(t *testing.T) {
	dir := t.TempDir()
	if _, err := Get(dir, "does-not-exist"); err == nil {
		t.Fatalf("expected error for missing review")
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	r := Review{ID: "r-1", RecordRef: RecordRef{Type: RecordTypeTransition, ID: "T-1"}, Body: "why", Source: SourceAI}
	if err := Add(dir, r); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := Delete(dir, "r-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "reviews", "r-1.json")); !os.IsNotExist(err) {
		t.Fatalf("reviews/r-1.json should be gone, stat err = %v", err)
	}
}

func TestDelete_MissingIsError(t *testing.T) {
	dir := t.TempDir()
	if err := Delete(dir, "does-not-exist"); err == nil {
		t.Fatalf("expected error deleting a nonexistent review")
	}
}

func TestList_SortedByID(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"r-2", "r-1", "r-3"} {
		if err := Add(dir, Review{ID: id, RecordRef: RecordRef{Type: RecordTypeTag, ID: "t"}, Body: "b", Source: SourceAI}); err != nil {
			t.Fatalf("Add %s: %v", id, err)
		}
	}
	got, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"r-1", "r-2", "r-3"}
	if len(got) != len(want) {
		t.Fatalf("List = %+v, want ids %v", got, want)
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("List[%d].ID = %q, want %q (got=%+v)", i, got[i].ID, id, got)
		}
	}
}
