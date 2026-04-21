package scrape

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// sampleThread mirrors the real HN item page layout: tr.athing.comtr rows
// as siblings with indent encoded via td.ind > img[width]. Includes a
// flagged row (no commtext) and a multi-paragraph commtext to exercise
// the innerHTML path.
const sampleThread = `<html><body><table><tbody>
<tr class="athing comtr" id="100">
  <td><table><tbody><tr>
    <td class="ind"><img src="s.gif" width="0" height="1"></td>
    <td class="default">
      <div class="comhead">
        <a class="hnuser">alice</a>
        <span class="age" title="2025-11-05T18:00:00 1700000000"><a href="item?id=100">3 hours ago</a></span>
      </div>
      <div class="comment"><div class="commtext c00">alice top-level</div></div>
    </td>
  </tr></tbody></table></td>
</tr>
<tr class="athing comtr" id="101">
  <td><table><tbody><tr>
    <td class="ind"><img src="s.gif" width="40" height="1"></td>
    <td class="default">
      <div class="comhead">
        <a class="hnuser">bob</a>
        <span class="age" title="2025-11-05T19:00:00 1700003600"><a href="item?id=101">2 hours ago</a></span>
      </div>
      <div class="comment"><div class="commtext c00">reply from bob<p>second paragraph</div></div>
    </td>
  </tr></tbody></table></td>
</tr>
<tr class="athing comtr" id="102">
  <td><table><tbody><tr>
    <td class="ind"><img src="s.gif" width="80" height="1"></td>
    <td class="default">
      <div class="comhead">
        <a class="hnuser">carol</a>
        <span class="age" title="2025-11-05T20:00:00 1700007200"><a href="item?id=102">1 hour ago</a></span>
      </div>
      <div class="comment"><div class="commtext c00">deep nested</div></div>
    </td>
  </tr></tbody></table></td>
</tr>
<tr class="athing comtr" id="103">
  <td><table><tbody><tr>
    <td class="ind"><img src="s.gif" width="0" height="1"></td>
    <td class="default">
      <div class="comhead"><span class="comhead">[flagged]</span></div>
    </td>
  </tr></tbody></table></td>
</tr>
<tr class="athing comtr" id="104">
  <td><table><tbody><tr>
    <td class="ind"><img src="s.gif" width="0" height="1"></td>
    <td class="default">
      <div class="comhead">
        <a class="hnuser">dave</a>
        <span class="age" title="2025-11-05T21:00:00 1700010800"><a href="item?id=104">now</a></span>
      </div>
      <div class="comment"><div class="commtext c00">dave top-level</div></div>
    </td>
  </tr></tbody></table></td>
</tr>
</tbody></table></body></html>`

func TestThread_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/item" || r.URL.Query().Get("id") != "1" {
			t.Errorf("unexpected request %q", r.URL.String())
		}
		if ua := r.Header.Get("User-Agent"); !strings.Contains(ua, "hn-cli") {
			t.Errorf("user-agent = %q, want containing 'hn-cli'", ua)
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, sampleThread)
	}))
	defer srv.Close()

	s := &Scraper{HTTP: srv.Client(), BaseURL: srv.URL, UserAgent: "hn-cli test"}
	comments, err := s.Thread(context.Background(), 1)
	if err != nil {
		t.Fatalf("Thread: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("top-level count = %d, want 2 (100, 104; 103 flagged dropped)", len(comments))
	}

	alice := comments[0]
	if alice.Item.ID != 100 || alice.Item.By != "alice" || alice.Item.Time != 1700000000 {
		t.Errorf("alice mismatch: %+v", alice.Item)
	}
	if alice.Item.Parent != 1 {
		t.Errorf("alice parent = %d, want storyID 1", alice.Item.Parent)
	}
	if alice.Depth != 0 {
		t.Errorf("alice depth = %d, want 0", alice.Depth)
	}
	if len(alice.Children) != 1 || alice.Children[0].Item.ID != 101 {
		t.Fatalf("alice children = %+v", alice.Children)
	}
	if ids := alice.Item.Kids; len(ids) != 1 || ids[0] != 101 {
		t.Errorf("alice kids = %v", ids)
	}

	bob := alice.Children[0]
	if bob.Depth != 1 || bob.Item.Parent != 100 {
		t.Errorf("bob mismatch: depth=%d parent=%d", bob.Depth, bob.Item.Parent)
	}
	if !strings.Contains(bob.Item.Text, "reply from bob") || !strings.Contains(bob.Item.Text, "<p>second paragraph</p>") {
		t.Errorf("bob text = %q (expected to include both paragraphs with closing p)", bob.Item.Text)
	}
	if len(bob.Children) != 1 || bob.Children[0].Item.ID != 102 {
		t.Fatalf("bob children = %+v", bob.Children)
	}
	if bob.Children[0].Depth != 2 {
		t.Errorf("carol depth = %d, want 2", bob.Children[0].Depth)
	}

	dave := comments[1]
	if dave.Item.ID != 104 || dave.Depth != 0 || len(dave.Children) != 0 {
		t.Errorf("dave mismatch: %+v children=%d", dave.Item, len(dave.Children))
	}
}

func TestThread_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	s := &Scraper{HTTP: srv.Client(), BaseURL: srv.URL}
	if _, err := s.Thread(context.Background(), 1); err == nil {
		t.Fatal("expected error on 404")
	}
}

func TestThread_CtxCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	s := &Scraper{HTTP: srv.Client(), BaseURL: srv.URL}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Thread(ctx, 1); err == nil {
		t.Fatal("expected error on canceled context")
	}
}

func TestParseAgeTitle(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"2025-11-05T18:22:11 1762369331", 1762369331},
		{"", 0},
		{"no-space", 0},
		{"bad-format garbage", 0},
	}
	for _, tt := range tests {
		if got := parseAgeTitle(tt.in); got != tt.want {
			t.Errorf("parseAgeTitle(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
