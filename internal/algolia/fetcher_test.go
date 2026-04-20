package algolia

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	hn "github.com/heartleo/hn-cli"
)

const sampleThread = `{
  "id": 1,
  "created_at_i": 1700000000,
  "type": "story",
  "author": "op",
  "title": "root",
  "children": [
    {
      "id": 10,
      "created_at_i": 1700000100,
      "type": "comment",
      "author": "alice",
      "text": "top-level one",
      "parent_id": 1,
      "story_id": 1,
      "children": [
        {
          "id": 11,
          "created_at_i": 1700000200,
          "type": "comment",
          "author": "bob",
          "text": "reply to alice",
          "parent_id": 10,
          "story_id": 1,
          "children": []
        },
        {
          "id": 12,
          "created_at_i": 1700000300,
          "type": "comment",
          "author": null,
          "text": null,
          "parent_id": 10,
          "story_id": 1,
          "children": [
            {"id": 13, "created_at_i": 1700000400, "type": "comment", "author": "orphan", "text": "should be dropped with parent", "parent_id": 12, "story_id": 1, "children": []}
          ]
        }
      ]
    },
    {
      "id": 20,
      "created_at_i": 1700000500,
      "type": "comment",
      "author": "carol",
      "text": "top-level two",
      "parent_id": 1,
      "story_id": 1,
      "children": []
    }
  ]
}`

func TestThread_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/items/1" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, sampleThread)
	}))
	defer srv.Close()

	f := &Fetcher{HTTP: srv.Client(), BaseURL: srv.URL}
	comments, err := f.Thread(context.Background(), 1)
	if err != nil {
		t.Fatalf("Thread: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("top-level count = %d, want 2", len(comments))
	}

	alice := comments[0]
	if alice.Item.ID != 10 || alice.Item.By != "alice" || alice.Depth != 0 {
		t.Errorf("first comment mismatch: %+v", alice.Item)
	}
	if len(alice.Children) != 1 {
		t.Fatalf("alice children = %d, want 1 (dead child 12 + orphan 13 dropped)", len(alice.Children))
	}
	if alice.Children[0].Item.ID != 11 || alice.Children[0].Depth != 1 {
		t.Errorf("alice reply mismatch: %+v", alice.Children[0].Item)
	}
	if got, want := alice.Item.Kids, []int{11}; len(got) != len(want) || got[0] != want[0] {
		t.Errorf("alice kids = %v, want %v", got, want)
	}

	carol := comments[1]
	if carol.Item.ID != 20 || len(carol.Children) != 0 {
		t.Errorf("carol mismatch: %+v children=%d", carol.Item, len(carol.Children))
	}
}

func TestThread_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	f := &Fetcher{HTTP: srv.Client(), BaseURL: srv.URL}
	if _, err := f.Thread(context.Background(), 1); err == nil {
		t.Fatal("expected error on 404")
	}
}

func TestReorderByKids(t *testing.T) {
	mk := func(id int) *hn.Comment { return &hn.Comment{Item: hn.Item{ID: id}} }

	tests := []struct {
		name     string
		comments []*hn.Comment
		kids     []int
		want     []int
	}{
		{
			name:     "reorders to match kids",
			comments: []*hn.Comment{mk(10), mk(20), mk(30)},
			kids:     []int{30, 10, 20},
			want:     []int{30, 10, 20},
		},
		{
			name:     "appends extras not in kids",
			comments: []*hn.Comment{mk(10), mk(20), mk(30)},
			kids:     []int{20},
			want:     []int{20, 10, 30},
		},
		{
			name:     "skips kids without matching comment",
			comments: []*hn.Comment{mk(10), mk(20)},
			kids:     []int{99, 20, 10},
			want:     []int{20, 10},
		},
		{
			name:     "empty kids returns input untouched",
			comments: []*hn.Comment{mk(10), mk(20)},
			kids:     nil,
			want:     []int{10, 20},
		},
		{
			name:     "single comment short-circuits",
			comments: []*hn.Comment{mk(10)},
			kids:     []int{99, 10},
			want:     []int{10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReorderByKids(tt.comments, tt.kids)
			if len(got) != len(tt.want) {
				t.Fatalf("len=%d, want %d", len(got), len(tt.want))
			}
			for i, c := range got {
				if c.Item.ID != tt.want[i] {
					t.Errorf("idx %d: got %d, want %d", i, c.Item.ID, tt.want[i])
				}
			}
		})
	}
}

func TestThread_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	f := &Fetcher{HTTP: srv.Client(), BaseURL: srv.URL}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.Thread(ctx, 1); err == nil {
		t.Fatal("expected error on canceled context")
	}
}
