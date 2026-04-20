package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	hn "github.com/heartleo/hn-cli"
	"github.com/heartleo/hn-cli/internal/translate"
)

func runBatchCommandAt(t *testing.T, cmd tea.Cmd, index int) tea.Msg {
	t.Helper()
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected batch command, got %T", msg)
	}
	if index < 0 || index >= len(batch) {
		t.Fatalf("expected batch command index %d in %d commands", index, len(batch))
	}
	return batch[index]()
}

func TestWindowResizePreservesViewportContent(t *testing.T) {
	m := newModel(hn.CategoryTop)
	// Simulate initial size
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(model)

	// Simulate entering detail view with content
	m.state = stateDetail
	m.detail = &hn.Item{ID: 1, Title: "Test", By: "user"}
	m.comments = []*hn.Comment{
		{Item: hn.Item{ID: 2, By: "alice", Text: "hello"}, Depth: 0},
	}
	m.mdCache = make(markdownCache)
	m.rebuildCommentView()
	m.viewport.SetYOffset(3)

	savedContent := m.viewport.View()
	savedOffset := m.viewport.YOffset()

	// Resize height only, same width
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = updated.(model)

	if m.viewport.View() == "" && savedContent != "" {
		t.Fatal("viewport content was lost on resize")
	}
	if m.viewport.YOffset() == 0 && savedOffset != 0 {
		t.Fatal("viewport scroll position was reset on resize")
	}
}

func TestNewModelHidesListPagination(t *testing.T) {
	m := newModel(hn.CategoryTop)
	if m.list.ShowPagination() {
		t.Fatal("expected list pagination dots to be hidden")
	}
}

func TestListViewShowsHelpHintInHeader(t *testing.T) {
	m := newModel(hn.CategoryTop)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 30})
	m = updated.(model)
	m.state = stateList
	m.stories[hn.CategoryTop] = []hn.Story{{Item: hn.Item{ID: 1, Title: "Story"}, Rank: 1}}
	m.setListItems(m.stories[hn.CategoryTop])

	view := m.viewList()
	if strings.Contains(view, "read") {
		t.Fatalf("expected help overlay to be hidden by default, got %q", view)
	}
	if !strings.Contains(view, "? help") {
		t.Fatalf("expected '? help' hint in header gap, got %q", view)
	}
	if got := lipgloss.Height(view); got > m.height {
		t.Fatalf("expected list view height <= %d, got %d", m.height, got)
	}
}

func TestDetailViewShowsHelpHintByDefault(t *testing.T) {
	m := newModel(hn.CategoryTop)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 30})
	m = updated.(model)
	updated, _ = m.Update(topCommentsMsg{
		story: hn.Item{ID: 1, Title: "Story", By: "alice"},
		comments: []*hn.Comment{
			{Item: hn.Item{ID: 2, By: "bob", Text: "hello"}, Depth: 0},
		},
	})
	m = updated.(model)

	view := m.viewDetail()
	if strings.Contains(view, "navigate comments") {
		t.Fatalf("expected help overlay to be hidden by default, got %q", view)
	}
	if !strings.Contains(view, "? help") {
		t.Fatalf("expected '? help' hint in detail view, got %q", view)
	}
	if got := lipgloss.Height(view); got > m.height {
		t.Fatalf("expected detail view height <= %d, got %d", m.height, got)
	}
}

func TestHelpOverlayAppearsOnQuestionMark(t *testing.T) {
	m := newModel(hn.CategoryTop)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 30})
	m = updated.(model)
	m.state = stateList
	m.stories[hn.CategoryTop] = []hn.Story{{Item: hn.Item{ID: 1, Title: "Story"}, Rank: 1}}
	m.setListItems(m.stories[hn.CategoryTop])

	updated, _ = m.Update(tea.KeyPressMsg{Code: '?'})
	m = *(updated.(*model))

	if !m.helpVisible {
		t.Fatal("expected helpVisible=true after pressing ?")
	}
	// Overlay is injected in View(), not viewList(); verify the overlay content is rendered.
	overlay := m.renderHelpOverlay()
	if !strings.Contains(overlay, "read") {
		t.Fatalf("expected key bindings in help overlay, got %q", overlay)
	}
}

func TestRenderStoryCardUsesTerminalWidth(t *testing.T) {
	const width = 80
	card := renderStoryCard(&hn.Item{ID: 1, Title: "Story", By: "alice"}, width, "")
	lines := strings.Split(strings.TrimRight(card, "\n"), "\n")
	if len(lines) == 0 {
		t.Fatal("expected rendered story card")
	}
	if got := lipgloss.Width(lines[0]); got != width {
		t.Fatalf("expected story card width %d, got %d", width, got)
	}
}

func TestDetailStoryCardShowsTranslatedTitleBelowOriginal(t *testing.T) {
	m := newModel(hn.CategoryTop)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = updated.(model)
	m.detail = &hn.Item{ID: 1, Title: "Original title", By: "alice"}
	m.translations[1] = "Translated title"
	m.showTranslation[1] = true

	card := m.renderDetailStoryCard()
	originalIndex := strings.Index(card, "Original title")
	translatedIndex := strings.Index(card, "Translated title")
	if originalIndex < 0 || translatedIndex < 0 {
		t.Fatalf("expected original and translated titles in detail card, got %q", card)
	}
	if originalIndex > translatedIndex {
		t.Fatalf("expected translated title below original title, got %q", card)
	}
}

func TestTranslateSelectedTitleTogglesCachedTitle(t *testing.T) {
	m := newModel(hn.CategoryTop)
	m.list = list.New(nil, m.listDelegate(), 0, 0)
	m.setListItems([]hn.Story{{Item: hn.Item{ID: 1, Title: "Hello"}, Rank: 1}})
	m.translations[1] = "translated hello"

	cmd := m.translateSelectedTitle()
	if cmd != nil {
		t.Fatal("expected cached translation toggle to avoid async command")
	}
	if !m.showTranslation[1] {
		t.Fatal("expected cached translation to be shown")
	}

	cmd = m.translateSelectedTitle()
	if cmd != nil {
		t.Fatal("expected cached translation toggle to avoid async command")
	}
	if m.showTranslation[1] {
		t.Fatal("expected second toggle to show original title")
	}
}

func TestTranslateSelectedTitleWithoutConfigShowsToast(t *testing.T) {
	m := newModel(hn.CategoryTop)
	m.translator = translate.NewClient("", "", "", "")
	m.list = list.New(nil, m.listDelegate(), 0, 0)
	m.setListItems([]hn.Story{{Item: hn.Item{ID: 1, Title: "Hello"}, Rank: 1}})

	cmd := m.translateSelectedTitle()
	if cmd == nil {
		t.Fatal("expected toast timeout command")
	}
	if m.status != "" {
		t.Fatalf("expected missing config to avoid persistent status, got %q", m.status)
	}
	if !strings.Contains(m.toast, "HN_TRANSLATE_API_KEY") {
		t.Fatalf("expected toast message for missing translation config, got %q", m.toast)
	}

	updated, _ := m.Update(toastTimeoutMsg{id: m.toastID})
	m = updated.(model)
	if m.toast != "" {
		t.Fatalf("expected toast to clear after timeout, got %q", m.toast)
	}
}

func TestTranslateAllTitlesUsesSingleBatchRequest(t *testing.T) {
	calls := 0
	var expectedIDs []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(req.Messages) != 2 {
			t.Fatalf("expected two messages, got %#v", req.Messages)
		}
		var titles []struct {
			ID    int    `json:"id"`
			Title string `json:"title"`
		}
		if err := json.Unmarshal([]byte(req.Messages[1].Content), &titles); err != nil {
			t.Fatalf("decode title payload: %v", err)
		}
		if len(titles) != len(expectedIDs) {
			t.Fatalf("expected %d visible uncached titles, got %#v", len(expectedIDs), titles)
		}
		expectedSet := make(map[int]bool)
		for _, id := range expectedIDs {
			expectedSet[id] = true
		}
		response := make(map[int]string)
		for _, title := range titles {
			if !expectedSet[title.ID] {
				t.Fatalf("unexpected translated id %d, expected visible page ids %#v", title.ID, expectedIDs)
			}
			response[title.ID] = title.Title + " translated"
		}
		content, err := json.Marshal(response)
		if err != nil {
			t.Fatalf("marshal response: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": string(content)}},
			},
		})
	}))
	defer server.Close()

	m := newModel(hn.CategoryTop)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 25})
	m = updated.(model)
	m.translator = translate.NewClient(server.URL, "test-key", "test-model", "Chinese")
	m.stories[hn.CategoryTop] = []hn.Story{
		{Item: hn.Item{ID: 1, Title: "First"}, Rank: 1},
		{Item: hn.Item{ID: 2, Title: "Second"}, Rank: 2},
		{Item: hn.Item{ID: 3, Title: "Third"}, Rank: 3},
		{Item: hn.Item{ID: 4, Title: "Fourth"}, Rank: 4},
		{Item: hn.Item{ID: 5, Title: "Fifth"}, Rank: 5},
		{Item: hn.Item{ID: 6, Title: "Sixth"}, Rank: 6},
	}
	m.storyIDs[hn.CategoryTop] = []int{1, 2, 3, 4, 5, 6}
	m.setListItems(m.stories[hn.CategoryTop])
	m.list.Select(1)

	start, end := m.visibleScreenRange()
	for _, item := range m.list.VisibleItems()[start:end] {
		expectedIDs = append(expectedIDs, item.(hn.Story).Item.ID)
	}
	if len(expectedIDs) < 2 || expectedIDs[0] != 1 {
		t.Fatalf("expected visible page to start at first story, got %#v", expectedIDs)
	}

	cmd := m.translateAllTitles()
	if cmd == nil {
		t.Fatal("expected batch translation command")
	}
	if !m.translating[expectedIDs[0]] || !m.translating[expectedIDs[1]] {
		t.Fatalf("expected titles to be marked translating: %#v", m.translating)
	}

	msg := runBatchCommandAt(t, cmd, 1)
	updated, _ = m.Update(msg)
	m = updated.(model)

	if calls != 1 {
		t.Fatalf("expected one API request, got %d", calls)
	}
	for _, id := range expectedIDs {
		if m.translations[id] == "" {
			t.Fatalf("expected visible id %d to be translated, got %#v", id, m.translations)
		}
		if m.translating[id] {
			t.Fatalf("expected translating flag for id %d to be cleared: %#v", id, m.translating)
		}
	}
	if m.translations[6] != "" {
		t.Fatalf("expected off-screen title to remain untranslated, got %#v", m.translations)
	}
}

func TestTranslateAllTitlesTogglesOffWhenAllShowing(t *testing.T) {
	m := newModel(hn.CategoryTop)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 25})
	m = updated.(model)
	m.translator = translate.NewClient("https://example.invalid", "test-key", "test-model", "Chinese")
	m.stories[hn.CategoryTop] = []hn.Story{
		{Item: hn.Item{ID: 1, Title: "First"}, Rank: 1},
		{Item: hn.Item{ID: 2, Title: "Second"}, Rank: 2},
	}
	m.setListItems(m.stories[hn.CategoryTop])
	m.translations[1] = "first translated"
	m.translations[2] = "second translated"
	m.showTranslation[1] = true
	m.showTranslation[2] = true

	cmd := m.translateAllTitles()
	if cmd != nil {
		t.Fatal("expected toggle off to avoid async command")
	}
	if m.showTranslation[1] || m.showTranslation[2] {
		t.Fatalf("expected all translations hidden, got %#v", m.showTranslation)
	}
}

func TestTranslateSelectedCommentUsesMarkdownAndTogglesCachedTranslation(t *testing.T) {
	var gotReq struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"**translated** world"}}]}`))
	}))
	defer server.Close()

	m := newModel(hn.CategoryTop)
	m.translator = translate.NewClient(server.URL, "test-key", "test-model", "Chinese")
	m.width = 80
	updated, _ := m.Update(topCommentsMsg{
		story: hn.Item{ID: 1, Title: "Story"},
		comments: []*hn.Comment{
			{Item: hn.Item{ID: 2, By: "alice", Text: "Hello <i>world</i>"}, Depth: 0},
		},
	})
	m = updated.(model)

	cmd := m.translateSelectedComment()
	if cmd == nil {
		t.Fatal("expected comment translation command")
	}
	if !m.commentTranslating[2] {
		t.Fatalf("expected comment to be marked translating: %#v", m.commentTranslating)
	}
	if len(m.flatComments) == 0 || !strings.Contains(strings.Join(m.flatComments[0].lines, "\n"), "translating") {
		t.Fatalf("expected translating marker in comment header, got %#v", m.flatComments)
	}

	msg := runBatchCommandAt(t, cmd, 1)
	updated, _ = m.Update(msg)
	m = updated.(model)

	if gotReq.Messages[1].Content == "Hello <i>world</i>" {
		t.Fatalf("expected markdown payload, got raw HTML %q", gotReq.Messages[1].Content)
	}
	if m.commentTranslations[2] != "**translated** world" {
		t.Fatalf("expected translated markdown, got %#v", m.commentTranslations)
	}
	if !m.showCommentTranslation[2] {
		t.Fatal("expected translated comment to be shown")
	}
	lines := strings.Join(m.flatComments[0].lines, "\n")
	if !strings.Contains(lines, "Hello") || !strings.Contains(lines, "translated") {
		t.Fatalf("expected original and translated comment in render output, got %#v", m.flatComments[0].lines)
	}
	if !strings.Contains(lines, "\u2500\u2500") {
		t.Fatalf("expected translation divider, got %#v", m.flatComments[0].lines)
	}

	cmd = m.translateSelectedComment()
	if cmd != nil {
		t.Fatal("expected cached comment translation toggle to avoid async command")
	}
	if m.showCommentTranslation[2] {
		t.Fatalf("expected cached comment translation to be hidden: %#v", m.showCommentTranslation)
	}
}

func TestPrefetchTabsMarksNonCurrentCategoriesLoading(t *testing.T) {
	m := newModel(hn.CategoryTop)
	m.state = stateList
	m.storyIDs[hn.CategoryTop] = make([]int, 100)
	m.stories[hn.CategoryTop] = make([]hn.Story, initialStoryLoad)
	m.setListItems(m.stories[hn.CategoryTop])

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 25})
	m = updated.(model)

	if !m.tabsPrefetched {
		t.Fatal("expected tab prefetch to be marked started")
	}
	if m.storiesLoading[hn.CategoryTop] {
		t.Fatal("expected current category to keep foreground loading path")
	}
	for _, cat := range []hn.Category{hn.CategoryNew, hn.CategoryBest, hn.CategoryAsk, hn.CategoryShow} {
		if !m.storiesLoading[cat] {
			t.Fatalf("expected %s to be preloading", cat)
		}
	}
}

func TestWindowSizeDoesNotPrefetchTabsBeforeCurrentTabLoads(t *testing.T) {
	m := newModel(hn.CategoryTop)
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 25})
	m = updated.(model)

	if cmd != nil {
		t.Fatal("expected no background prefetch before current tab loads")
	}
	if m.tabsPrefetched {
		t.Fatal("expected tabs not to be marked prefetched before current tab loads")
	}
	for _, cat := range []hn.Category{hn.CategoryNew, hn.CategoryBest, hn.CategoryAsk, hn.CategoryShow} {
		if m.storiesLoading[cat] {
			t.Fatalf("expected %s not to be preloading before current tab loads", cat)
		}
	}
}

func TestBackgroundStoriesMsgDoesNotReplaceCurrentList(t *testing.T) {
	m := newModel(hn.CategoryTop)
	m.state = stateList
	topStories := []hn.Story{{Item: hn.Item{ID: 1, Title: "top story"}, Rank: 1}}
	m.stories[hn.CategoryTop] = topStories
	m.setListItems(topStories)

	updated, _ := m.Update(storiesMsg{
		cat: hn.CategoryNew,
		ids: []int{2},
		stories: []hn.Story{
			{Item: hn.Item{ID: 2, Title: "new story"}, Rank: 1},
		},
	})
	m = updated.(model)

	if m.category != hn.CategoryTop {
		t.Fatalf("expected current category to remain top, got %s", m.category)
	}
	if got := m.list.Items()[0].(hn.Story).Item.Title; got != "top story" {
		t.Fatalf("expected current list to remain top stories, got %q", got)
	}
	if got := m.stories[hn.CategoryNew][0].Item.Title; got != "new story" {
		t.Fatalf("expected background tab cache to be populated, got %q", got)
	}
}

func TestReplyStatusHasNoBracketsAndTranslationStatusLast(t *testing.T) {
	m := newModel(hn.CategoryTop)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = updated.(model)
	m.commentTranslating[2] = true

	updated, _ = m.Update(topCommentsMsg{
		story: hn.Item{ID: 1, Title: "Story"},
		comments: []*hn.Comment{
			{Item: hn.Item{ID: 2, By: "alice", Text: "parent", Kids: []int{3}}, Depth: 0},
		},
	})
	m = updated.(model)

	if len(m.flatComments) == 0 {
		t.Fatal("expected rendered comments")
	}
	header := m.flatComments[0].lines[0]
	if strings.Contains(header, "[2 replies]") {
		t.Fatalf("expected reply status without brackets, got %q", header)
	}
	replyIndex := strings.Index(header, "2 replies")
	translationIndex := strings.Index(header, "translating")
	if replyIndex < 0 || translationIndex < 0 {
		t.Fatalf("expected reply and translation statuses in header, got %q", header)
	}
	if replyIndex > translationIndex {
		t.Fatalf("expected translation status to be last, got %q", header)
	}
}

func TestReplyStatusCountsRecursiveSubtreeAndRootComment(t *testing.T) {
	m := newModel(hn.CategoryTop)
	m.state = stateDetail
	m.detail = &hn.Item{ID: 1, Title: "Story"}
	m.childrenExpanded = make(map[int]bool)
	m.comments = []*hn.Comment{
		{Item: hn.Item{ID: 2, By: "alice", Text: "root", Kids: []int{3, 4}}, Depth: 0},
	}
	m.rebuildCommentView()

	lines := strings.Join(m.flatComments[0].lines, "\n")
	if !strings.Contains(lines, "3 replies") {
		t.Fatalf("expected unloaded reply count to include root comment, got %#v", m.flatComments[0].lines)
	}

	m.comments[0].Children = []*hn.Comment{
		{
			Item:  hn.Item{ID: 3, By: "bob", Text: "child", Kids: []int{5}},
			Depth: 1,
			Children: []*hn.Comment{
				{Item: hn.Item{ID: 5, By: "carol", Text: "grandchild"}, Depth: 2},
			},
		},
		{Item: hn.Item{ID: 4, By: "dave", Text: "child"}, Depth: 1},
	}
	m.rebuildCommentView()

	lines = strings.Join(m.flatComments[0].lines, "\n")
	if !strings.Contains(lines, "4 replies") {
		t.Fatalf("expected loaded reply count to include recursive descendants and root comment, got %#v", m.flatComments[0].lines)
	}

	m.collapsed[2] = true
	m.rebuildCommentView()

	lines = strings.Join(m.flatComments[0].lines, "\n")
	if !strings.Contains(lines, "[4 more]") {
		t.Fatalf("expected collapsed count to include recursive descendants and root comment, got %#v", m.flatComments[0].lines)
	}
}

func TestRefreshCommentsMsgReplacesComments(t *testing.T) {
	m := newModel(hn.CategoryTop)
	m.state = stateDetail
	m.detail = &hn.Item{ID: 1, Title: "Story", Descendants: 1}
	m.commentsRefreshing = true
	m.comments = []*hn.Comment{
		{Item: hn.Item{ID: 2, By: "alice", Text: "old"}, Depth: 0},
	}

	updated, _ := m.Update(refreshCommentsMsg{
		story: hn.Item{ID: 1, Title: "Story", Descendants: 3},
		comments: []*hn.Comment{
			{Item: hn.Item{ID: 2, By: "alice", Text: "updated"}, Depth: 0},
			{Item: hn.Item{ID: 4, By: "carol", Text: "new"}, Depth: 0},
		},
	})
	m = updated.(model)

	if m.commentsRefreshing {
		t.Fatal("expected refresh flag cleared")
	}
	if m.detail.Descendants != 3 {
		t.Fatalf("expected fresh story metadata, got %#v", m.detail)
	}
	if len(m.comments) != 2 {
		t.Fatalf("expected 2 refreshed comments, got %d", len(m.comments))
	}
	if m.comments[0].Item.Text != "updated" {
		t.Fatalf("expected fresh comment text, got %q", m.comments[0].Item.Text)
	}
}

func TestRefreshCommentsMsgIgnoredForDifferentStory(t *testing.T) {
	m := newModel(hn.CategoryTop)
	m.state = stateDetail
	m.detail = &hn.Item{ID: 1, Title: "Story"}
	m.commentsRefreshing = true
	m.comments = []*hn.Comment{{Item: hn.Item{ID: 2, Text: "old"}, Depth: 0}}

	updated, _ := m.Update(refreshCommentsMsg{
		story:    hn.Item{ID: 9, Title: "Other Story"},
		comments: []*hn.Comment{{Item: hn.Item{ID: 10, Text: "new"}, Depth: 0}},
	})
	m = updated.(model)

	if len(m.comments) != 1 || m.comments[0].Item.ID != 2 {
		t.Fatalf("expected stale refresh ignored, got %#v", m.comments)
	}
	if !m.commentsRefreshing {
		t.Fatal("expected refresh flag unchanged for stale refresh message")
	}
}

func TestScrollToCommentKeepsNextCommentNearBottom(t *testing.T) {
	m := newModel(hn.CategoryTop)
	m.viewport.SetHeight(10)
	m.viewport.SetContent(strings.Repeat("line\n", 30))
	m.viewport.SetYOffset(0)
	m.flatComments = []flatComment{
		{startLine: 0, endLine: 5},
		{startLine: 5, endLine: 9},
		{startLine: 10, endLine: 13},
	}
	m.commentCursor = 2

	m.scrollToComment()

	if got := m.viewport.YOffset(); got != 5 {
		t.Fatalf("expected minimal downward scroll to y=5, got %d", got)
	}
}

func TestScrollToCommentKeepsPreviousCommentNearTop(t *testing.T) {
	m := newModel(hn.CategoryTop)
	m.viewport.SetHeight(10)
	m.viewport.SetContent(strings.Repeat("line\n", 30))
	m.viewport.SetYOffset(10)
	m.flatComments = []flatComment{
		{startLine: 4, endLine: 7},
		{startLine: 10, endLine: 14},
	}
	m.commentCursor = 0

	m.scrollToComment()

	if got := m.viewport.YOffset(); got != 3 {
		t.Fatalf("expected minimal upward scroll to y=3, got %d", got)
	}
}

func TestLazyStoryTargetUsesVisibleWindowAndOneAndHalfScreens(t *testing.T) {
	m := newModel(hn.CategoryTop)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 25})
	m = updated.(model)

	m.storyIDs[hn.CategoryTop] = make([]int, 100)
	m.stories[hn.CategoryTop] = make([]hn.Story, initialStoryLoad)
	m.list = list.New(nil, m.listDelegate(), 80, 20)
	m.setListItems(m.stories[hn.CategoryTop])
	m.list.Select(19)
	m.scrollToStory()

	if got := m.lazyStoryTarget(); got != 23 {
		t.Fatalf("expected target 23, got %d", got)
	}
}

func TestEnsureStoriesLoadedMarksCategoryLoading(t *testing.T) {
	m := newModel(hn.CategoryTop)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 25})
	m = updated.(model)

	m.storyIDs[hn.CategoryTop] = make([]int, 100)
	m.stories[hn.CategoryTop] = make([]hn.Story, initialStoryLoad)
	m.list = list.New(nil, m.listDelegate(), 80, 20)
	m.setListItems(m.stories[hn.CategoryTop])
	m.list.Select(19)
	m.scrollToStory()

	cmd := m.ensureStoriesLoaded()
	if cmd == nil {
		t.Fatal("expected lazy load command")
	}
	if !m.storiesLoading[hn.CategoryTop] {
		t.Fatal("expected category to be marked loading")
	}
}

func TestVisibleStoryRangeUsesCurrentScreenAndHalfScreenAhead(t *testing.T) {
	m := newModel(hn.CategoryTop)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 25})
	m = updated.(model)

	m.storyIDs[hn.CategoryTop] = make([]int, 100)
	m.stories[hn.CategoryTop] = make([]hn.Story, initialStoryLoad)
	m.list = list.New(nil, m.listDelegate(), 80, 20)
	m.setListItems(m.stories[hn.CategoryTop])
	m.list.Select(19)
	m.scrollToStory()

	start, end := m.visibleStoryRange()
	if start != 15 || end != 23 {
		t.Fatalf("expected range [15, 23), got [%d, %d)", start, end)
	}
}

func TestListDownKeepsNextStoryNearBottomAtPageBoundary(t *testing.T) {
	m := newModel(hn.CategoryTop)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 25})
	m = updated.(model)
	m.state = stateList

	stories := make([]hn.Story, 8)
	for i := range stories {
		stories[i] = hn.Story{Item: hn.Item{ID: i + 1, Title: fmt.Sprintf("story %d", i+1)}, Rank: i + 1}
	}
	m.stories[hn.CategoryTop] = stories
	m.storyIDs[hn.CategoryTop] = make([]int, 8)
	m.setListItems(stories)

	visible := m.visibleStoryCount()
	if visible < 2 {
		t.Fatalf("expected at least two visible stories, got %d", visible)
	}
	m.list.Select(visible - 1)
	m.scrollToStory()

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = *(updated.(*model))

	if got := m.list.Index(); got != visible {
		t.Fatalf("expected selected story %d, got %d", visible, got)
	}
	if got := m.storyListOffset; got != 1 {
		t.Fatalf("expected minimal list scroll offset 1, got %d", got)
	}
}

func TestRefreshStoriesMsgReplacesVisibleRange(t *testing.T) {
	m := newModel(hn.CategoryTop)
	stories := []hn.Story{
		{Item: hn.Item{ID: 1, Title: "old 1"}, Rank: 1},
		{Item: hn.Item{ID: 2, Title: "old 2"}, Rank: 2},
		{Item: hn.Item{ID: 3, Title: "old 3"}, Rank: 3},
	}
	m.stories[hn.CategoryTop] = stories
	m.storyIDs[hn.CategoryTop] = []int{1, 2, 3}
	m.setListItems(stories)

	updated, _ := m.Update(refreshStoriesMsg{
		cat:      hn.CategoryTop,
		start:    1,
		selected: 1,
		ids:      []int{1, 22, 33},
		stories: []hn.Story{
			{Item: hn.Item{ID: 22, Title: "new 2"}, Rank: 2},
			{Item: hn.Item{ID: 33, Title: "new 3"}, Rank: 3},
		},
	})
	m = updated.(model)

	if m.stories[hn.CategoryTop][0].Item.Title != "old 1" {
		t.Fatal("expected story before refreshed range to remain unchanged")
	}
	if m.stories[hn.CategoryTop][1].Item.Title != "new 2" || m.stories[hn.CategoryTop][2].Item.Title != "new 3" {
		t.Fatalf("expected refreshed range to be replaced, got %#v", m.stories[hn.CategoryTop])
	}
	if got := m.list.Index(); got != 1 {
		t.Fatalf("expected selected index restored to 1, got %d", got)
	}
}
