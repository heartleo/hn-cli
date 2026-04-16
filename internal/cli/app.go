package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/heartleo/hn"
	"github.com/heartleo/hn/internal/config"
	"github.com/heartleo/hn/internal/translate"
)

type appState int

const (
	stateLoading appState = iota
	stateList
	stateDetail
	stateDetailLoading
)

var categories = []hn.Category{
	hn.CategoryTop, hn.CategoryNew, hn.CategoryBest,
	hn.CategoryAsk, hn.CategoryShow,
}

var categoryLabels = map[hn.Category]string{
	hn.CategoryTop:  "Top",
	hn.CategoryNew:  "New",
	hn.CategoryBest: "Best",
	hn.CategoryAsk:  "Ask",
	hn.CategoryShow: "Show",
}

const initialStoryLoad = 20
const initialTopCommentLoad = 20
const topCommentLoadBatch = 20
const listBottomGap = 3
const toastDuration = 3 * time.Second

const translationNotConfiguredMessage = "Translation disabled. Set HN_TRANSLATE_API_KEY to enable it."

// Messages
type storiesMsg struct {
	cat     hn.Category
	stories []hn.Story
	ids     []int // all story IDs for pagination
}

type storiesErrMsg struct {
	cat hn.Category
	err error
}

type moreStoriesMsg struct {
	cat     hn.Category
	stories []hn.Story
}

type refreshStoriesMsg struct {
	cat      hn.Category
	start    int
	stories  []hn.Story
	ids      []int
	selected int
}

type topCommentsMsg struct {
	story    hn.Item
	comments []*hn.Comment
	loaded   int
}

type moreTopCommentsMsg struct {
	storyID  int
	start    int
	end      int
	comments []*hn.Comment
}

type moreTopCommentsErrMsg struct {
	err error
}

// subtreeDoneMsg signals that the client's two-phase subtree fetch for a
// parent comment has reached a terminal state (Phase 2 success, or Phase 2
// retry budget exhausted with Phase 1 children still attached).
type subtreeDoneMsg struct {
	storyID  int
	parentID int
	children []*hn.Comment
	phase    int // 1 when Phase 2 failed terminally, 2 on success
}

type refreshCommentsMsg struct {
	story    hn.Item
	comments []*hn.Comment
	loaded   int
}

type refreshCommentsErrMsg struct {
	err error
}

type errMsg struct{ err error }

type translateMsg struct {
	itemID     int
	translated string
}

type translateErrMsg struct {
	itemID int
	err    error
}

type translateBatchMsg struct {
	itemIDs      []int
	translations map[int]string
}

type translateBatchErrMsg struct {
	itemIDs []int
	err     error
}

type translateCommentMsg struct {
	itemID     int
	translated string
}

type translateCommentErrMsg struct {
	itemID int
	err    error
}

type toastTimeoutMsg struct {
	id int
}

type model struct {
	state    appState
	category hn.Category
	width    int
	height   int

	list        list.Model
	viewport    viewport.Model
	spinner     spinner.Model
	help        help.Model
	helpVisible bool

	listKeys   listKeyMap
	detailKeys detailKeyMap

	client             *hn.Client
	translator         *translate.Client
	storyIDs           map[hn.Category][]int
	stories            map[hn.Category][]hn.Story
	storiesLoading     map[hn.Category]bool
	storyListOffset    int
	tabsPrefetched     bool
	translations       map[int]string
	translating        map[int]bool
	showTranslation    map[int]bool
	detail             *hn.Item
	detailCtx          context.Context
	detailCancel       context.CancelFunc
	comments           []*hn.Comment
	collapsed          map[int]bool
	childrenLoaded     map[int]bool
	childrenLoading    map[int]bool
	childrenExpanded   map[int]bool
	topCommentsLoading bool
	topCommentsLoaded  int
	commentsRefreshing bool

	// Comment cursor navigation
	flatComments           []flatComment
	commentCursor          int
	storyBodyRendered      string // pre-rendered HTML body for Ask HN / Job posts
	storyBodyOffset        int    // viewport line count occupied by storyBodyRendered
	mdCache                markdownCache
	commentTranslations    map[int]string // itemID → translated markdown (session cache)
	commentTranslating     map[int]bool   // itemID → request in-flight
	showCommentTranslation map[int]bool   // itemID → currently showing translation
	pendingG               bool           // first g of gg sequence received

	err     error
	status  string
	toast   string
	toastID int
}

func newModel(cat hn.Category) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(currentTheme.Accent)
	translateConfig := config.LoadTranslateConfig()
	translator := translate.NewClient(
		translateConfig.APIURL,
		translateConfig.APIKey,
		translateConfig.Model,
		translateConfig.Language,
	)

	translations := make(map[int]string)
	translating := make(map[int]bool)
	showTranslation := make(map[int]bool)

	delegate := storyDelegate{
		translations:    translations,
		translating:     translating,
		showTranslation: showTranslation,
	}
	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.Styles.NoItems = lipgloss.NewStyle().Foreground(currentTheme.Muted).Padding(1, 2)
	l.FilterInput.Prompt = "/ "
	filterStyles := textinput.DefaultStyles(true)
	filterStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(currentTheme.Accent)
	filterStyles.Blurred.Prompt = lipgloss.NewStyle().Foreground(currentTheme.Accent)
	l.FilterInput.SetStyles(filterStyles)

	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(currentTheme.Accent)
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(currentTheme.Muted)
	h.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(currentTheme.Surface)

	return model{
		state:                  stateLoading,
		category:               cat,
		spinner:                s,
		list:                   l,
		help:                   h,
		helpVisible:            false,
		listKeys:               newListKeyMap(),
		detailKeys:             newDetailKeyMap(),
		client:                 hn.NewClient(),
		translator:             translator,
		storyIDs:               make(map[hn.Category][]int),
		stories:                make(map[hn.Category][]hn.Story),
		storiesLoading:         make(map[hn.Category]bool),
		translations:           translations,
		translating:            translating,
		showTranslation:        showTranslation,
		collapsed:              make(map[int]bool),
		childrenLoaded:         make(map[int]bool),
		childrenLoading:        make(map[int]bool),
		childrenExpanded:       make(map[int]bool),
		commentTranslations:    make(map[int]string),
		commentTranslating:     make(map[int]bool),
		showCommentTranslation: make(map[int]bool),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetchStories(m.category))
}

func (m *model) showToast(message string) tea.Cmd {
	m.toastID++
	id := m.toastID
	m.toast = message
	return tea.Tick(toastDuration, func(time.Time) tea.Msg {
		return toastTimeoutMsg{id: id}
	})
}

func (m model) fetchStories(cat hn.Category) tea.Cmd {
	return m.fetchStoriesLimit(cat, initialStoryLoad)
}

func (m model) fetchStoriesLimit(cat hn.Category, limit int) tea.Cmd {
	return func() tea.Msg {
		ids, err := m.client.Stories(cat)
		if err != nil {
			return storiesErrMsg{cat: cat, err: err}
		}

		if limit <= 0 {
			limit = initialStoryLoad
		}
		if limit > len(ids) {
			limit = len(ids)
		}

		items, err := m.client.GetItems(ids[:limit])
		if err != nil {
			return storiesErrMsg{cat: cat, err: err}
		}

		stories := make([]hn.Story, 0, len(items))
		for i, item := range items {
			if item.ID == 0 {
				continue
			}
			stories = append(stories, hn.Story{Item: item, Rank: i + 1})
		}

		return storiesMsg{cat: cat, stories: stories, ids: ids}
	}
}

func (m model) fetchMoreStories(cat hn.Category, target int) tea.Cmd {
	ids := m.storyIDs[cat]
	loaded := len(m.stories[cat])
	if loaded >= len(ids) {
		return nil
	}

	end := target
	if end <= loaded {
		return nil
	}
	if end > len(ids) {
		end = len(ids)
	}
	nextIDs := ids[loaded:end]
	rankOffset := loaded

	return func() tea.Msg {
		items, err := m.client.GetItems(nextIDs)
		if err != nil {
			return errMsg{err}
		}

		stories := make([]hn.Story, 0, len(items))
		for i, item := range items {
			if item.ID == 0 {
				continue
			}
			stories = append(stories, hn.Story{Item: item, Rank: rankOffset + i + 1})
		}

		return moreStoriesMsg{cat: cat, stories: stories}
	}
}

func (m model) refreshVisibleStories(cat hn.Category) tea.Cmd {
	start, end := m.visibleStoryRange()
	selected := m.list.Index()
	return func() tea.Msg {
		ids, err := m.client.Stories(cat)
		if err != nil {
			return errMsg{err}
		}
		if start > len(ids) {
			start = len(ids)
		}
		if end > len(ids) {
			end = len(ids)
		}
		if end < start {
			end = start
		}

		items, err := m.client.GetItems(ids[start:end])
		if err != nil {
			return errMsg{err}
		}

		stories := make([]hn.Story, 0, len(items))
		for i, item := range items {
			if item.ID == 0 {
				continue
			}
			stories = append(stories, hn.Story{Item: item, Rank: start + i + 1})
		}

		return refreshStoriesMsg{cat: cat, start: start, stories: stories, ids: ids, selected: selected}
	}
}

func (m model) fetchTopComments(story hn.Item) tea.Cmd {
	return m.fetchTopCommentsLimit(story, initialTopCommentLoad)
}

func (m model) fetchTopCommentsLimit(story hn.Item, limit int) tea.Cmd {
	return func() tea.Msg {
		end := limit
		if end <= 0 || end > len(story.Kids) {
			end = len(story.Kids)
		}
		comments, err := m.client.GetTopComments(story.Kids[:end])
		if err != nil {
			return errMsg{err}
		}
		return topCommentsMsg{story: story, comments: comments, loaded: end}
	}
}

func (m model) fetchMoreTopComments(storyID int, start, limit int) tea.Cmd {
	if m.detail == nil || storyID != m.detail.ID || start >= len(m.detail.Kids) {
		return nil
	}
	end := start + limit
	if end > len(m.detail.Kids) {
		end = len(m.detail.Kids)
	}
	kids := append([]int(nil), m.detail.Kids[start:end]...)
	return func() tea.Msg {
		comments, err := m.client.GetTopComments(kids)
		if err != nil {
			return moreTopCommentsErrMsg{err: err}
		}
		return moreTopCommentsMsg{storyID: storyID, start: start, end: end, comments: comments}
	}
}

func (m model) refreshComments(storyID int) tea.Cmd {
	limit := m.topCommentsLoaded
	if limit <= 0 {
		limit = initialTopCommentLoad
	}
	return func() tea.Msg {
		story, err := m.client.GetItemFresh(storyID)
		if err != nil {
			return refreshCommentsErrMsg{err: err}
		}
		if limit > len(story.Kids) {
			limit = len(story.Kids)
		}
		comments, err := m.client.GetTopComments(story.Kids[:limit])
		if err != nil {
			return refreshCommentsErrMsg{err: err}
		}
		return refreshCommentsMsg{story: *story, comments: comments, loaded: limit}
	}
}

func (m *model) translateSelectedTitle() tea.Cmd {
	story, ok := m.list.SelectedItem().(hn.Story)
	if !ok {
		return nil
	}

	id := story.Item.ID
	if _, ok := m.translations[id]; ok {
		m.showTranslation[id] = !m.showTranslation[id]
		m.status = ""
		return nil
	}
	if m.translating[id] {
		return nil
	}
	if !m.translator.Configured() {
		return m.showToast(translationNotConfiguredMessage)
	}

	m.status = ""
	m.translating[id] = true
	title := story.Item.Title
	translator := m.translator
	translateCmd := func() tea.Msg {
		translated, err := translator.Translate(context.Background(), title)
		if err != nil {
			return translateErrMsg{itemID: id, err: err}
		}
		return translateMsg{itemID: id, translated: translated}
	}
	return tea.Batch(m.spinner.Tick, translateCmd)
}

func (m *model) translateAllTitles() tea.Cmd {
	stories := m.visibleStories()
	if len(stories) == 0 {
		m.status = "no titles loaded"
		return nil
	}
	if !m.translator.Configured() {
		return m.showToast(translationNotConfiguredMessage)
	}

	allTranslated := true
	allShowing := true
	for _, story := range stories {
		id := story.Item.ID
		if strings.TrimSpace(story.Item.Title) == "" {
			continue
		}
		if m.translations[id] == "" {
			allTranslated = false
		}
		if !m.showTranslation[id] {
			allShowing = false
		}
	}
	if allTranslated && allShowing {
		for _, story := range stories {
			m.showTranslation[story.Item.ID] = false
		}
		m.status = ""
		return nil
	}

	titles := make(map[int]string)
	for _, story := range stories {
		id := story.Item.ID
		if translated := m.translations[id]; translated != "" {
			m.showTranslation[id] = true
			continue
		}
		if m.translating[id] || strings.TrimSpace(story.Item.Title) == "" {
			continue
		}
		titles[id] = story.Item.Title
	}

	if len(titles) == 0 {
		return nil
	}

	itemIDs := make([]int, 0, len(titles))
	for id := range titles {
		m.translating[id] = true
		itemIDs = append(itemIDs, id)
	}

	translator := m.translator
	translateCmd := func() tea.Msg {
		translations, err := translator.TranslateBatch(context.Background(), titles)
		if err != nil {
			return translateBatchErrMsg{itemIDs: itemIDs, err: err}
		}
		return translateBatchMsg{itemIDs: itemIDs, translations: translations}
	}
	return tea.Batch(m.spinner.Tick, translateCmd)
}

func (m *model) translateSelectedComment() tea.Cmd {
	if len(m.flatComments) == 0 || m.commentCursor >= len(m.flatComments) {
		return nil
	}

	comment := m.flatComments[m.commentCursor].comment
	id := comment.Item.ID
	if _, ok := m.commentTranslations[id]; ok {
		m.showCommentTranslation[id] = !m.showCommentTranslation[id]
		m.status = ""
		m.rebuildCommentView()
		return nil
	}
	if m.commentTranslating[id] {
		return nil
	}
	if !m.translator.Configured() {
		return m.showToast(translationNotConfiguredMessage)
	}

	markdown := strings.TrimSpace(m.commentMarkdown(comment))
	if markdown == "" {
		m.status = "comment is empty"
		return nil
	}

	m.status = ""
	m.commentTranslating[id] = true
	m.rebuildCommentView()
	translator := m.translator
	translateCmd := func() tea.Msg {
		translated, err := translator.TranslateMarkdown(context.Background(), markdown)
		if err != nil {
			return translateCommentErrMsg{itemID: id, err: err}
		}
		return translateCommentMsg{itemID: id, translated: translated}
	}
	return tea.Batch(m.spinner.Tick, translateCmd)
}

func (m model) visibleStories() []hn.Story {
	items := m.list.VisibleItems()
	if len(items) == 0 {
		return nil
	}

	start, end := m.visibleScreenRange()
	if start > len(items) {
		start = len(items)
	}
	if end < start {
		end = start
	}
	if end > len(items) {
		end = len(items)
	}

	stories := make([]hn.Story, 0, end-start)
	for _, item := range items[start:end] {
		story, ok := item.(hn.Story)
		if !ok {
			continue
		}
		stories = append(stories, story)
	}
	return stories
}

func (m model) networkBusy() bool {
	if m.state == stateLoading || m.state == stateDetailLoading || m.commentsRefreshing || m.topCommentsLoading {
		return true
	}
	for _, loading := range m.storiesLoading {
		if loading {
			return true
		}
	}
	for _, loading := range m.childrenLoading {
		if loading {
			return true
		}
	}
	for _, translating := range m.translating {
		if translating {
			return true
		}
	}
	for _, translating := range m.commentTranslating {
		if translating {
			return true
		}
	}
	return false
}

func (m model) inlineNetworkIndicator() string {
	if !m.networkBusy() || m.state == stateLoading || m.state == stateDetailLoading {
		return ""
	}
	return m.spinner.View()
}

func (m model) listContentHeight() int {
	headerHeight := 4 // tab top border + tab content + tab bottom border + newline
	contentHeight := m.height - headerHeight - listBottomGap
	if contentHeight < 1 {
		contentHeight = 1
	}
	return contentHeight
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentHeight := m.listContentHeight()
		m.list.SetSize(m.width, contentHeight)
		m.list.SetDelegate(m.listDelegate())
		m.viewport.SetWidth(m.width)
		m.viewport.SetHeight(contentHeight)
		m.help.SetWidth(m.width)

		if m.state == stateDetail {
			m.syncDetailViewport()
			savedOffset := m.viewport.YOffset()
			m.rebuildCommentView()
			m.viewport.SetYOffset(savedOffset)
			m.scrollToComment()
			return m, m.prefetchTabsIfNeeded()
		}
		if m.state == stateList {
			m.scrollToStory()
			return m, m.ensureStoriesLoadedThenPrefetchTabs()
		}
		return m, nil

	case errMsg:
		slog.Debug("errMsg", "err", msg.err)
		m.err = msg.err
		m.state = stateList
		return m, nil

	case storiesErrMsg:
		delete(m.storiesLoading, msg.cat)
		if msg.cat == m.category {
			m.err = msg.err
			m.state = stateList
		}
		return m, nil

	case storiesMsg:
		m.storyIDs[msg.cat] = msg.ids
		m.stories[msg.cat] = msg.stories
		delete(m.storiesLoading, msg.cat)
		if msg.cat == m.category {
			m.setListItems(msg.stories)
			m.storyListOffset = 0
			m.scrollToStory()
			m.state = stateList
			return m, m.ensureStoriesLoadedThenPrefetchTabs()
		}
		return m, nil

	case moreStoriesMsg:
		m.stories[msg.cat] = append(m.stories[msg.cat], msg.stories...)
		delete(m.storiesLoading, msg.cat)
		if msg.cat == m.category {
			m.setListItems(m.stories[msg.cat])
			m.scrollToStory()
			m.state = stateList
			m.status = ""
			return m, m.ensureStoriesLoadedThenPrefetchTabs()
		}
		return m, nil

	case refreshStoriesMsg:
		m.storyIDs[msg.cat] = msg.ids
		delete(m.storiesLoading, msg.cat)
		if msg.cat == m.category {
			current := m.stories[msg.cat]
			needed := msg.start + len(msg.stories)
			if len(current) < needed {
				expanded := make([]hn.Story, needed)
				copy(expanded, current)
				current = expanded
			}
			copy(current[msg.start:], msg.stories)
			m.stories[msg.cat] = current
			m.setListItems(current)
			if len(current) > 0 {
				if msg.selected >= len(current) {
					msg.selected = len(current) - 1
				}
				m.list.Select(msg.selected)
			}
			m.scrollToStory()
			m.state = stateList
			m.status = ""
			return m, m.ensureStoriesLoadedThenPrefetchTabs()
		}
		return m, nil

	case topCommentsMsg:
		slog.Debug("topCommentsMsg", "story", msg.story.ID, "comments", len(msg.comments), "loaded", msg.loaded, "total_kids", len(msg.story.Kids))
		// Cancel any in-flight pre-fetch from a previous story.
		if m.detailCancel != nil {
			m.detailCancel()
		}
		m.detailCtx, m.detailCancel = context.WithCancel(context.Background())
		m.detail = &msg.story
		m.comments = msg.comments
		m.collapsed = make(map[int]bool)
		m.childrenLoaded = make(map[int]bool)
		m.childrenLoading = make(map[int]bool)
		m.childrenExpanded = make(map[int]bool)
		m.topCommentsLoading = false
		m.topCommentsLoaded = msg.loaded
		m.commentCursor = 0
		m.mdCache = make(markdownCache)
		m.state = stateDetail
		m.syncDetailViewport()
		m.rebuildCommentView()
		m.viewport.SetYOffset(0)
		return m, m.loadFirstCommentTree()

	case moreTopCommentsMsg:
		slog.Debug("moreTopCommentsMsg", "story", msg.storyID, "start", msg.start, "end", msg.end, "got", len(msg.comments))
		if m.state != stateDetail || m.detail == nil || msg.storyID != m.detail.ID {
			slog.Debug("moreTopCommentsMsg discarded", "state", m.state)
			return m, nil
		}
		m.topCommentsLoading = false
		if msg.start != m.topCommentsLoaded {
			slog.Debug("moreTopCommentsMsg stale", "msg.start", msg.start, "loaded", m.topCommentsLoaded)
			return m, nil
		}
		m.topCommentsLoaded = msg.end
		m.comments = append(m.comments, msg.comments...)
		m.rebuildCommentView()
		return m, m.ensureTopCommentsLoaded()

	case moreTopCommentsErrMsg:
		slog.Debug("moreTopCommentsErrMsg", "err", msg.err)
		m.topCommentsLoading = false
		if m.state == stateDetail {
			m.status = "load comments failed: " + msg.err.Error()
		}
		return m, nil

	case subtreeDoneMsg:
		slog.Debug("subtreeDoneMsg", "story", msg.storyID, "parent", msg.parentID, "phase", msg.phase, "children", len(msg.children))
		// Always clear loading, even when discarding, so the spinner never
		// gets stuck if UI state drifted while the fetch was in flight.
		delete(m.childrenLoading, msg.parentID)
		if m.state != stateDetail || m.detail == nil || msg.storyID != m.detail.ID {
			return m, nil
		}
		parent := findCommentByID(m.comments, msg.parentID)
		if parent == nil {
			slog.Debug("subtreeDoneMsg parent not found", "parent", msg.parentID)
			return m, nil
		}
		if msg.children != nil {
			parent.Children = msg.children
		}
		// Only mark loaded when Phase 2 completed. Phase 1 only means the
		// client retried Phase 2 to exhaustion; a later user expand will
		// implicitly restart Phase 2.
		if msg.phase == 2 {
			m.childrenLoaded[msg.parentID] = true
		}
		m.rebuildCommentView()
		return m, nil

	case refreshCommentsMsg:
		if m.state != stateDetail || m.detail == nil || msg.story.ID != m.detail.ID {
			return m, nil
		}
		m.commentsRefreshing = false
		m.detail = &msg.story
		m.comments = mergeTopComments(m.comments, msg.comments)
		m.client.InvalidateAllSubtrees()
		m.childrenLoaded = loadedTopCommentIDs(m.comments)
		m.childrenLoading = make(map[int]bool)
		m.topCommentsLoading = false
		m.topCommentsLoaded = msg.loaded
		if m.childrenExpanded == nil {
			m.childrenExpanded = make(map[int]bool)
		}
		m.status = ""
		m.rebuildCommentView()
		return m, m.loadFirstCommentTree()

	case refreshCommentsErrMsg:
		m.commentsRefreshing = false
		m.status = "refresh comments failed: " + msg.err.Error()
		return m, nil

	case translateMsg:
		delete(m.translating, msg.itemID)
		m.translations[msg.itemID] = msg.translated
		m.showTranslation[msg.itemID] = true
		m.status = ""
		return m, nil

	case translateErrMsg:
		delete(m.translating, msg.itemID)
		m.status = "translation failed: " + msg.err.Error()
		return m, nil

	case translateBatchMsg:
		for _, id := range msg.itemIDs {
			delete(m.translating, id)
		}
		for id, translated := range msg.translations {
			m.translations[id] = translated
			m.showTranslation[id] = true
		}
		m.status = ""
		return m, nil

	case translateBatchErrMsg:
		for _, id := range msg.itemIDs {
			delete(m.translating, id)
		}
		m.status = "translation failed: " + msg.err.Error()
		return m, nil

	case translateCommentMsg:
		delete(m.commentTranslating, msg.itemID)
		if m.commentTranslations == nil {
			m.commentTranslations = make(map[int]string)
		}
		if m.showCommentTranslation == nil {
			m.showCommentTranslation = make(map[int]bool)
		}
		m.commentTranslations[msg.itemID] = msg.translated
		m.showCommentTranslation[msg.itemID] = true
		m.status = ""
		m.rebuildCommentView()
		return m, nil

	case translateCommentErrMsg:
		delete(m.commentTranslating, msg.itemID)
		m.status = "translation failed: " + msg.err.Error()
		m.rebuildCommentView()
		return m, nil

	case toastTimeoutMsg:
		if msg.id == m.toastID {
			m.toast = ""
		}
		return m, nil

	case spinner.TickMsg:
		if m.networkBusy() {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			if m.state == stateDetail {
				m.rebuildCommentView()
			}
			return m, cmd
		}
		return m, nil

	case tea.KeyPressMsg:
		// Help overlay intercepts all keys; only ? and esc dismiss it.
		if m.helpVisible {
			if key.Matches(msg, m.listKeys.Help) || key.Matches(msg, m.detailKeys.Help) ||
				msg.String() == "esc" {
				m.helpVisible = false
			}
			return m, nil
		}

		// If list is filtering, pass keys to it
		if m.state == stateList && m.list.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}

		switch m.state {
		case stateList:
			return m.updateList(msg)
		case stateDetail:
			return m.updateDetail(msg)
		case stateLoading, stateDetailLoading:
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}
	}

	// Pass other messages to active component
	switch m.state {
	case stateList:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	case stateDetail:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *model) updateList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.listKeys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.listKeys.Help):
		m.helpVisible = true
		return m, nil

	case key.Matches(msg, m.listKeys.NextTab):
		m.switchTab(1)
		return m, m.loadCategoryIfNeeded()

	case key.Matches(msg, m.listKeys.PrevTab):
		m.switchTab(-1)
		return m, m.loadCategoryIfNeeded()

	case key.Matches(msg, m.listKeys.Refresh):
		if m.storiesLoading[m.category] {
			return m, nil
		}
		m.storiesLoading[m.category] = true
		m.status = ""
		return m, tea.Batch(m.spinner.Tick, m.refreshVisibleStories(m.category))

	case key.Matches(msg, m.listKeys.Translate):
		return m, m.translateSelectedTitle()

	case key.Matches(msg, m.listKeys.TranslateAll):
		return m, m.translateAllTitles()

	case key.Matches(msg, m.listKeys.Open):
		if item, ok := m.list.SelectedItem().(hn.Story); ok {
			url := item.Item.URL
			if url == "" {
				url = fmt.Sprintf("https://news.ycombinator.com/item?id=%d", item.Item.ID)
			}
			if err := openBrowser(url); err != nil && !errors.Is(err, errBrowserOpenerUnavailable) {
				m.err = err
			}
		}
		return m, nil

	case key.Matches(msg, m.listKeys.Read):
		if item, ok := m.list.SelectedItem().(hn.Story); ok {
			m.state = stateDetailLoading
			return m, tea.Batch(m.spinner.Tick, m.fetchTopComments(item.Item))
		}
		return m, nil
	}

	// Default list navigation
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.scrollToStory()
	loadCmd := m.ensureStoriesLoaded()
	return m, tea.Batch(cmd, loadCmd)
}

func (m *model) leaveDetailView() {
	if m.detailCancel != nil {
		m.detailCancel()
		m.detailCancel = nil
	}

	m.state = stateList
	m.detail = nil
	m.comments = nil
	m.flatComments = nil
	m.storyBodyRendered = ""
	m.storyBodyOffset = 0
	m.mdCache = nil
	m.collapsed = make(map[int]bool)
	m.childrenLoaded = make(map[int]bool)
	m.childrenLoading = make(map[int]bool)
	m.childrenExpanded = make(map[int]bool)
	m.topCommentsLoading = false
	m.topCommentsLoaded = 0
	m.commentsRefreshing = false
	m.commentCursor = 0
	m.pendingG = false
	m.commentTranslations = make(map[int]string)
	m.commentTranslating = make(map[int]bool)
	m.showCommentTranslation = make(map[int]bool)
	m.viewport.SetContent("")
	m.viewport.SetYOffset(0)
	m.client.InvalidateAllSubtrees()
}
func (m *model) updateDetail(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Sync viewport height so scrollToComment uses the correct visible area
	m.syncDetailViewport()

	// Handle gg double-key sequence
	if m.pendingG {
		m.pendingG = false
		if msg.String() == "g" {
			m.commentCursor = 0
			m.applyCommentHighlight()
			m.viewport.SetYOffset(0)
			return m, nil
		}
		// Not g: fall through to normal key handling
	}

	switch {
	case key.Matches(msg, m.detailKeys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.detailKeys.Back):
		m.leaveDetailView()
		return m, nil

	case key.Matches(msg, m.detailKeys.Help):
		m.helpVisible = true
		return m, nil

	case key.Matches(msg, m.detailKeys.BackToTop):
		m.pendingG = true
		return m, nil

	case key.Matches(msg, m.detailKeys.Open):
		if m.detail != nil {
			url := m.detail.URL
			if url == "" {
				url = fmt.Sprintf("https://news.ycombinator.com/item?id=%d", m.detail.ID)
			}
			if err := openBrowser(url); err != nil && !errors.Is(err, errBrowserOpenerUnavailable) {
				m.err = err
			}
		}
		return m, nil

	case key.Matches(msg, m.detailKeys.Translate):
		return m, m.translateSelectedComment()

	case key.Matches(msg, m.detailKeys.Replies):
		return m, m.toggleSelectedReplies()

	case key.Matches(msg, m.detailKeys.Refresh):
		if m.detail == nil || m.commentsRefreshing {
			return m, nil
		}
		m.commentsRefreshing = true
		m.status = ""
		return m, tea.Batch(m.spinner.Tick, m.refreshComments(m.detail.ID))

	case key.Matches(msg, m.detailKeys.GotoRoot):
		if len(m.flatComments) > 0 && m.commentCursor < len(m.flatComments) {
			for i := m.commentCursor; i >= 0; i-- {
				if m.flatComments[i].comment.Depth == 0 {
					saved := m.viewport.YOffset()
					m.commentCursor = i
					m.applyCommentHighlight()
					m.viewport.SetYOffset(saved)
					m.scrollToComment()
					break
				}
			}
		}
		return m, nil

	case key.Matches(msg, m.detailKeys.Up):
		if m.commentCursor > 0 {
			saved := m.viewport.YOffset()
			m.commentCursor--
			m.applyCommentHighlight()
			m.viewport.SetYOffset(saved)
			m.scrollToComment()
		}
		return m, m.ensureTopCommentsLoaded()

	case key.Matches(msg, m.detailKeys.Down):
		if m.commentCursor < len(m.flatComments)-1 {
			saved := m.viewport.YOffset()
			m.commentCursor++
			m.applyCommentHighlight()
			m.viewport.SetYOffset(saved)
			m.scrollToComment()
		}
		return m, m.ensureTopCommentsLoaded()

	case key.Matches(msg, m.detailKeys.Collapse):
		if len(m.flatComments) > 0 && m.commentCursor < len(m.flatComments) {
			fc := m.flatComments[m.commentCursor]
			screenRow := fc.startLine - m.viewport.YOffset()
			m.collapsed[fc.comment.Item.ID] = !m.collapsed[fc.comment.Item.ID]
			m.rebuildCommentView()
			if m.commentCursor < len(m.flatComments) {
				newStart := m.flatComments[m.commentCursor].startLine
				target := newStart - screenRow
				if target < 0 {
					target = 0
				}
				m.viewport.SetYOffset(target)
				// If clamped, ensure comment is still visible
				if m.flatComments[m.commentCursor].startLine < m.viewport.YOffset() {
					m.viewport.SetYOffset(m.flatComments[m.commentCursor].startLine)
				}
			}
		}
		return m, nil

	case key.Matches(msg, m.detailKeys.CollapseAll):
		m.collapseAll(m.comments)
		m.rebuildCommentView()
		m.viewport.SetYOffset(0)
		return m, nil

	case key.Matches(msg, m.detailKeys.ExpandAll):
		m.collapsed = make(map[int]bool)
		m.rebuildCommentView()
		m.viewport.SetYOffset(0)
		return m, nil
	}

	// Viewport scrolling (PgUp/PgDn etc)
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, tea.Batch(cmd, m.ensureTopCommentsLoaded())
}

func (m model) View() tea.View {
	var content string
	switch m.state {
	case stateLoading:
		content = m.viewLoading("Loading stories...")
	case stateDetailLoading:
		content = m.viewLoading("Loading comments...")
	case stateList:
		content = m.viewList()
	case stateDetail:
		content = m.viewDetail()
	}
	if m.toast != "" && m.width > 0 && m.height > 0 {
		content = overlayToast(content, m.renderToast(), m.width, m.height)
	}
	if m.helpVisible && m.width > 0 && m.height > 0 {
		content = overlayCenter(content, m.renderHelpOverlay(), m.width, m.height)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m model) viewLoading(msg string) string {
	header := m.renderHeader()
	loading := lipgloss.NewStyle().Padding(2, 2).Render(
		fmt.Sprintf("%s %s", m.spinner.View(), msg),
	)
	return lipgloss.JoinVertical(lipgloss.Left, header, loading)
}

func (m model) viewList() string {
	header := m.renderHeader()
	content := m.listContentView()

	if m.err != nil {
		errView := lipgloss.NewStyle().Padding(1, 2).Render(
			colorRed(symbolError+" Error:") + " " + m.err.Error(),
		)
		return joinVisibleVertical(header, errView)
	}

	if len(m.translating) > 0 {
		spinnerView := lipgloss.NewStyle().Foreground(currentTheme.Muted).Padding(0, 2).Render(
			m.spinner.View() + " translating...",
		)
		return joinVisibleVertical(header, content, spinnerView)
	}

	if m.status != "" {
		statusView := lipgloss.NewStyle().Foreground(currentTheme.Warning).Padding(0, 2).Render(m.status)
		return joinVisibleVertical(header, content, statusView)
	}

	return joinVisibleVertical(header, content)
}

func (m model) listContentView() string {
	if m.list.FilterState() == list.Filtering {
		return m.list.View()
	}

	items := m.list.VisibleItems()
	if len(items) == 0 {
		return m.list.View()
	}

	visible := m.visibleStoryCount()
	if visible < 1 {
		visible = 1
	}

	selected := m.list.Index()
	if selected < 0 {
		selected = 0
	}
	if selected >= len(items) {
		selected = len(items) - 1
	}

	offset := m.storyListOffset
	if offset < 0 {
		offset = 0
	}
	maxOffset := len(items) - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	if selected < offset {
		offset = selected
	}
	if selected >= offset+visible {
		offset = selected - visible + 1
	}

	end := offset + visible
	if end > len(items) {
		end = len(items)
	}

	var b strings.Builder
	delegate := m.listDelegate()
	for i := offset; i < end; i++ {
		delegate.Render(&b, m.list, i, items[i])
		if i < end-1 {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

func joinVisibleVertical(parts ...string) string {
	visible := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			visible = append(visible, part)
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, visible...)
}

// renderStoryCard builds the story card header for the detail view.
func renderStoryCard(detail *hn.Item, width int, indicator string) string {
	return renderStoryCardWithTranslation(detail, width, indicator, "")
}

func (m model) renderDetailStoryCard() string {
	if m.detail == nil {
		return ""
	}
	translated := ""
	if m.showTranslation[m.detail.ID] {
		translated = strings.TrimSpace(m.translations[m.detail.ID])
		if translated == strings.TrimSpace(m.detail.Title) {
			translated = ""
		}
	}
	return renderStoryCardWithTranslation(m.detail, m.width, m.inlineNetworkIndicator(), translated)
}

func renderStoryCardWithTranslation(detail *hn.Item, width int, indicator string, translatedTitle string) string {
	titleStyle := lipgloss.NewStyle().Foreground(currentTheme.Title).Bold(true)
	translationStyle := lipgloss.NewStyle().Foreground(currentTheme.Muted)
	metaStyle := lipgloss.NewStyle().Foreground(currentTheme.Muted)
	urlStyle := lipgloss.NewStyle().Foreground(currentTheme.Link)
	scoreStyle := lipgloss.NewStyle().Foreground(currentTheme.Score).Bold(true)

	var cardContent strings.Builder
	title := titleStyle.Render(detail.Title)
	if indicator != "" {
		contentWidth := width - 6 // border(2) + horizontal padding(4)
		if contentWidth > 0 {
			gap := contentWidth - lipgloss.Width(title) - lipgloss.Width(indicator)
			if gap > 0 {
				title += strings.Repeat(" ", gap) + indicator
			}
		}
	}
	cardContent.WriteString(title + "\n")
	if strings.TrimSpace(translatedTitle) != "" {
		cardContent.WriteString(translationStyle.Render(translatedTitle) + "\n")
	}

	meta := fmt.Sprintf("%s points by %s %s | %d comments",
		scoreStyle.Render(fmt.Sprintf("▲ %d", detail.Score)),
		detail.By, detail.RelativeTime(), detail.Descendants)
	cardContent.WriteString(metaStyle.Render(meta))

	if detail.URL != "" {
		cardContent.WriteString("\n" + urlStyle.Render(detail.URL))
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(currentTheme.Accent).
		Padding(1, 2).
		Width(width)

	return cardStyle.Render(cardContent.String()) + "\n"
}

// renderStoryBody renders the HTML text body of an Ask HN / Job post for display
// inside the detail viewport, above the comment list.
func renderStoryBody(text string, width int) string {
	markdown := htmlToMarkdown(text)
	contentWidth := width - 4
	if contentWidth < 30 {
		contentWidth = 30
	}
	rendered := renderMarkdown(markdown, contentWidth)
	return strings.TrimRight(rendered, "\n") + "\n\n"
}

// detailProgressBarHeight is the fixed number of rows reserved below the viewport
// for the scroll progress bar in detail view.
const detailProgressBarHeight = 1

// detailViewportHeight computes the viewport content height for the detail view
// from pre-measured component heights. Single source of truth shared by
// syncDetailViewport and viewDetail so the two never diverge.
func detailViewportHeight(total, headerLines, statusLines int) int {
	h := total - headerLines - statusLines - detailProgressBarHeight
	if h < 3 {
		h = 3
	}
	return h
}

// syncDetailViewport sets the viewport height to match the actual visible area
// in detail view. Must be called before scrollToComment for accurate calculations.
func (m *model) syncDetailViewport() {
	if m.detail == nil || m.height == 0 {
		return
	}
	storyHeader := m.renderDetailStoryCard()
	headerLines := lipgloss.Height(storyHeader)

	statusLines := 0
	if m.status != "" {
		statusLines = lipgloss.Height(
			lipgloss.NewStyle().Foreground(currentTheme.Warning).Padding(0, 2).Render(m.status),
		)
	}

	m.viewport.SetHeight(detailViewportHeight(m.height, headerLines, statusLines))
	m.viewport.SetWidth(m.width)
}

func (m model) viewDetail() string {
	if m.detail == nil {
		return ""
	}

	storyHeader := m.renderDetailStoryCard()
	headerLines := lipgloss.Height(storyHeader)

	statusView := ""
	statusLines := 0
	if m.status != "" {
		statusView = lipgloss.NewStyle().Foreground(currentTheme.Warning).Padding(0, 2).Render(m.status)
		statusLines = lipgloss.Height(statusView)
	}

	m.viewport.SetHeight(detailViewportHeight(m.height, headerLines, statusLines))
	m.viewport.SetWidth(m.width)

	// Bottom indicator: "? help" injected into the last viewport line (ANSI-aware).
	hint := lipgloss.NewStyle().Foreground(currentTheme.Muted).Render("? help")
	content := m.viewport.View()
	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		suffix := " " + hint
		suffixW := lipgloss.Width(suffix)
		col := m.width - suffixW
		if col > 0 {
			left := ansi.Truncate(lines[len(lines)-1], col, "")
			leftW := lipgloss.Width(left)
			if leftW < col {
				left += strings.Repeat(" ", col-leftW)
			}
			lines[len(lines)-1] = left + suffix
		}
		content = strings.Join(lines, "\n")
	}

	progressBar := m.renderScrollBar()
	if statusView != "" {
		return joinVisibleVertical(storyHeader, content, progressBar, statusView)
	}
	return joinVisibleVertical(storyHeader, content, progressBar)
}

// renderScrollBar renders a 1-row horizontal progress bar reflecting the
// viewport scroll position. Hidden (blank line) when all content fits on screen.
func (m model) renderScrollBar() string {
	total := m.viewport.TotalLineCount()
	viewH := m.viewport.Height()
	if total <= viewH || m.width <= 0 {
		return strings.Repeat(" ", m.width)
	}
	filled := int(m.viewport.ScrollPercent() * float64(m.width))
	if filled > m.width {
		filled = m.width
	}
	// ▁ (U+2581) occupies the bottom eighth of the cell — renders as a thin line.
	const cell = "▁"
	filledStyle := lipgloss.NewStyle().Foreground(currentTheme.Accent)
	emptyStyle := lipgloss.NewStyle().Foreground(currentTheme.Surface)
	return filledStyle.Render(strings.Repeat(cell, filled)) +
		emptyStyle.Render(strings.Repeat(cell, m.width-filled))
}

func (m model) renderHeader() string {
	// Active tab: top + sides border, bottom open
	activeTabBorder := lipgloss.Border{
		Top: "─", Bottom: " ", Left: "│", Right: "│",
		TopLeft: "╭", TopRight: "╮",
		BottomLeft: "╯", BottomRight: "╰",
	}
	activeTabStyle := lipgloss.NewStyle().
		Border(activeTabBorder).
		BorderForeground(currentTheme.Accent).
		Foreground(currentTheme.Accent).
		Bold(true).
		Padding(0, 1)

	// Inactive tab: bottom border only (merges with separator line)
	inactiveTabBorder := lipgloss.Border{
		Top: " ", Bottom: "─", Left: " ", Right: " ",
		TopLeft: " ", TopRight: " ",
		BottomLeft: "─", BottomRight: "─",
	}
	inactiveTabStyle := lipgloss.NewStyle().
		Border(inactiveTabBorder).
		BorderForeground(currentTheme.Accent).
		Foreground(currentTheme.Muted).
		Padding(0, 1)

	// Title block: bottom border only
	titleStyle := lipgloss.NewStyle().
		Border(inactiveTabBorder).
		BorderForeground(currentTheme.Accent).
		Foreground(currentTheme.Score).
		Bold(true).
		Padding(0, 1)
	title := titleStyle.Render("Hacker News")

	// Build tab blocks
	var tabBlocks []string
	tabBlocks = append(tabBlocks, title)
	for _, cat := range categories {
		label := categoryLabels[cat]
		if cat == m.category {
			tabBlocks = append(tabBlocks, activeTabStyle.Render(label))
		} else {
			tabBlocks = append(tabBlocks, inactiveTabStyle.Render(label))
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, tabBlocks...)

	// Fill remaining width with bottom border
	rowWidth := lipgloss.Width(row)
	if gap := m.width - rowWidth; gap > 0 {
		gapBorder := lipgloss.Border{
			Top: " ", Bottom: "─",
			TopLeft: " ", TopRight: " ",
			BottomLeft: "─", BottomRight: "─",
		}
		gapStyle := lipgloss.NewStyle().
			Border(gapBorder).
			BorderForeground(currentTheme.Accent)
		gapContentWidth := gap - 2
		gapContent := ""
		if gapContentWidth > 0 {
			hint := lipgloss.NewStyle().Foreground(currentTheme.Muted).Render("? help")
			indicator := m.inlineNetworkIndicator()
			// Show spinner (when loading) and/or "? help" hint right-aligned in the gap.
			var inner string
			switch {
			case indicator != "" && lipgloss.Width(indicator)+1+lipgloss.Width(hint) <= gapContentWidth:
				inner = indicator + " " + hint
			case indicator != "":
				inner = indicator
			default:
				inner = hint
			}
			if lipgloss.Width(inner) <= gapContentWidth {
				gapContent = strings.Repeat(" ", gapContentWidth-lipgloss.Width(inner)) + inner
			}
		}
		row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, gapStyle.Render(gapContent))
	}

	return row + "\n"
}

func (m model) listDelegate() storyDelegate {
	return storyDelegate{
		width:           m.width,
		translations:    m.translations,
		translating:     m.translating,
		showTranslation: m.showTranslation,
	}
}

func (m *model) setListItems(stories []hn.Story) {
	items := make([]list.Item, len(stories))
	for i, s := range stories {
		items[i] = s
	}
	m.list.SetItems(items)
}

func (m *model) switchTab(dir int) {
	idx := 0
	for i, c := range categories {
		if c == m.category {
			idx = i
			break
		}
	}
	idx = (idx + dir + len(categories)) % len(categories)
	m.category = categories[idx]
}

func (m *model) loadCategoryIfNeeded() tea.Cmd {
	if cached, ok := m.stories[m.category]; ok {
		m.setListItems(cached)
		m.list.Select(0)
		m.storyListOffset = 0
		m.state = stateList
		return m.ensureStoriesLoaded()
	}
	if m.storiesLoading[m.category] {
		m.state = stateLoading
		return m.spinner.Tick
	}
	m.storiesLoading[m.category] = true
	m.state = stateLoading
	return tea.Batch(m.spinner.Tick, m.fetchStories(m.category))
}

func (m *model) prefetchTabsIfNeeded() tea.Cmd {
	if m.tabsPrefetched || m.height == 0 {
		return nil
	}

	target := m.oneAndHalfScreenStoryCount()
	if target <= 0 {
		target = initialStoryLoad
	}

	var cmds []tea.Cmd
	for _, cat := range categories {
		if cat == m.category {
			continue
		}
		if _, ok := m.stories[cat]; ok {
			continue
		}
		if m.storiesLoading[cat] {
			continue
		}
		m.storiesLoading[cat] = true
		cmds = append(cmds, m.fetchStoriesLimit(cat, target))
	}

	m.tabsPrefetched = true
	return tea.Batch(append([]tea.Cmd{m.spinner.Tick}, cmds...)...)
}

func (m *model) ensureStoriesLoaded() tea.Cmd {
	if m.storiesLoading[m.category] {
		return nil
	}
	target := m.lazyStoryTarget()
	loaded := len(m.stories[m.category])
	total := len(m.storyIDs[m.category])
	if total == 0 || loaded >= total || target <= loaded {
		return nil
	}
	m.storiesLoading[m.category] = true
	m.status = ""
	return tea.Batch(m.spinner.Tick, m.fetchMoreStories(m.category, target))
}

func (m *model) ensureStoriesLoadedThenPrefetchTabs() tea.Cmd {
	if cmd := m.ensureStoriesLoaded(); cmd != nil {
		return cmd
	}
	return m.prefetchTabsIfNeeded()
}

func (m *model) ensureTopCommentsLoaded() tea.Cmd {
	if m.state != stateDetail || m.detail == nil || m.topCommentsLoading {
		return nil
	}
	if m.topCommentsLoaded >= len(m.detail.Kids) {
		return nil
	}
	if m.viewport.Height() <= 0 || len(m.flatComments) == 0 {
		return nil
	}

	nearCursorEnd := m.commentCursor >= len(m.flatComments)-3
	visibleBottom := m.viewport.YOffset() + m.viewport.Height()
	nearScrollEnd := visibleBottom+(m.viewport.Height()/2) >= m.viewport.TotalLineCount()
	if !nearCursorEnd && !nearScrollEnd {
		return nil
	}

	start := m.topCommentsLoaded
	slog.Debug("ensureTopCommentsLoaded firing",
		"loaded", start, "total_kids", len(m.detail.Kids),
		"cursor", m.commentCursor, "flat", len(m.flatComments),
		"nearCursor", nearCursorEnd, "nearScroll", nearScrollEnd,
	)
	m.topCommentsLoading = true
	return tea.Batch(m.spinner.Tick, m.fetchMoreTopComments(m.detail.ID, start, topCommentLoadBatch))
}

func (m model) lazyStoryTarget() int {
	target := initialStoryLoad
	visible := m.visibleStoryCount()
	if visible > 0 {
		target = m.storyListOffset + m.oneAndHalfScreenStoryCount()
		if target < initialStoryLoad {
			target = initialStoryLoad
		}
	}
	if total := len(m.storyIDs[m.category]); total > 0 && target > total {
		target = total
	}
	return target
}

func (m model) visibleStoryRange() (int, int) {
	start := m.storyListOffset
	if start < 0 {
		start = 0
	}
	loaded := len(m.stories[m.category])
	if loaded > 0 && start >= loaded {
		start = loaded - 1
	}

	count := m.oneAndHalfScreenStoryCount()
	if count < 1 {
		count = 1
	}
	end := start + count

	total := len(m.storyIDs[m.category])
	if total > 0 && end > total {
		end = total
	}
	if end < start {
		end = start
	}
	return start, end
}

func (m model) visibleScreenRange() (int, int) {
	items := m.list.VisibleItems()
	if len(items) == 0 {
		return 0, 0
	}
	perPage := m.visibleStoryCount()
	if perPage < 1 {
		perPage = 1
	}

	start := m.storyListOffset
	if start > len(items) {
		start = len(items)
	}
	end := start + perPage
	if end > len(items) {
		end = len(items)
	}
	if end < start {
		end = start
	}
	return start, end
}

func (m model) oneAndHalfScreenStoryCount() int {
	visible := m.visibleStoryCount()
	return (visible*3 + 1) / 2
}

func (m model) visibleStoryCount() int {
	contentHeight := m.height - 4 - listBottomGap
	if contentHeight <= 0 {
		return 0
	}
	delegate := m.listDelegate()
	itemHeight := delegate.Height() + delegate.Spacing()
	if itemHeight <= 0 {
		return 0
	}
	return (contentHeight + itemHeight - 1) / itemHeight
}

func (m *model) scrollToStory() {
	items := m.list.VisibleItems()
	if len(items) == 0 {
		m.storyListOffset = 0
		return
	}

	visible := m.visibleStoryCount()
	if visible < 1 {
		visible = 1
	}

	selected := m.list.Index()
	if selected < 0 {
		selected = 0
	}
	if selected >= len(items) {
		selected = len(items) - 1
	}

	maxOffset := len(items) - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.storyListOffset > maxOffset {
		m.storyListOffset = maxOffset
	}
	if m.storyListOffset < 0 {
		m.storyListOffset = 0
	}

	if selected < m.storyListOffset {
		m.storyListOffset = selected
		return
	}
	if selected >= m.storyListOffset+visible {
		m.storyListOffset = selected - visible + 1
	}
}

// rebuildCommentView re-renders all comments (expensive). Call on load/collapse only.
func (m *model) rebuildCommentView() {
	selectedID := 0
	if len(m.flatComments) > 0 && m.commentCursor < len(m.flatComments) {
		selectedID = m.flatComments[m.commentCursor].comment.Item.ID
	}

	// Pre-render story body (Ask HN / Job posts only).
	m.storyBodyRendered = ""
	m.storyBodyOffset = 0
	if m.detail != nil && strings.TrimSpace(m.detail.Text) != "" {
		m.storyBodyRendered = renderStoryBody(m.detail.Text, m.width)
		m.storyBodyOffset = strings.Count(m.storyBodyRendered, "\n")
	}

	bodySource := func(c *hn.Comment) string {
		return m.commentMarkdown(c)
	}
	translationSource := func(c *hn.Comment) (string, bool) {
		id := c.Item.ID
		if !m.showCommentTranslation[id] {
			return "", false
		}
		translated, ok := m.commentTranslations[id]
		return translated, ok && strings.TrimSpace(translated) != ""
	}
	commentStatus := func(c *hn.Comment) string {
		id := c.Item.ID
		if m.commentTranslating[id] {
			return lipgloss.NewStyle().Foreground(currentTheme.Muted).Render(m.spinner.View() + " translating")
		}
		return ""
	}
	repliesStatus := func(c *hn.Comment) string {
		if len(c.Item.Kids) == 0 {
			return ""
		}
		id := c.Item.ID
		replyWord := "replies"
		count := len(c.Item.Kids) + 1
		if count == 1 {
			replyWord = "reply"
		}
		style := lipgloss.NewStyle().Foreground(currentTheme.Muted)
		label := fmt.Sprintf("%d %s", count, replyWord)
		if m.childrenLoading[id] {
			return style.Render(m.spinner.View() + " " + label)
		}
		return style.Render(label)
	}
	childrenExpanded := func(c *hn.Comment) bool {
		return m.childrenExpanded[c.Item.ID]
	}
	m.flatComments = buildFlatComments(m.comments, m.collapsed, m.width-2, bodySource, translationSource, commentStatus, repliesStatus, childrenExpanded)

	// After collapse, cursor may be out of range
	if m.commentCursor >= len(m.flatComments) {
		m.commentCursor = len(m.flatComments) - 1
	}
	if m.commentCursor < 0 {
		m.commentCursor = 0
	}

	// Re-resolve cursor position by ID
	if selectedID > 0 {
		for i, fc := range m.flatComments {
			if fc.comment.Item.ID == selectedID {
				m.commentCursor = i
				break
			}
		}
	}

	m.applyCommentHighlight()
}

func (m *model) commentMarkdown(c *hn.Comment) string {
	if m.mdCache == nil {
		m.mdCache = make(markdownCache)
	}
	id := c.Item.ID
	if cached, ok := m.mdCache[id]; ok {
		return cached
	}
	markdown := htmlToMarkdown(c.Item.Text)
	m.mdCache[id] = markdown
	return markdown
}

func (m *model) toggleSelectedReplies() tea.Cmd {
	if m.state != stateDetail || len(m.flatComments) == 0 || m.commentCursor >= len(m.flatComments) {
		return nil
	}
	if m.childrenLoaded == nil {
		m.childrenLoaded = make(map[int]bool)
	}
	if m.childrenLoading == nil {
		m.childrenLoading = make(map[int]bool)
	}
	if m.childrenExpanded == nil {
		m.childrenExpanded = make(map[int]bool)
	}

	comment := m.flatComments[m.commentCursor].comment
	if comment == nil || len(comment.Item.Kids) == 0 {
		return nil
	}

	id := comment.Item.ID
	if m.childrenLoaded[id] {
		m.childrenExpanded[id] = !m.childrenExpanded[id]
		m.rebuildCommentView()
		return nil
	}

	m.childrenExpanded[id] = true
	if m.childrenLoading[id] {
		m.rebuildCommentView()
		return nil
	}
	m.rebuildCommentView()
	return m.ensureSubtree(comment)
}

func (m *model) loadFirstCommentTree() tea.Cmd {
	if m.state != stateDetail || len(m.comments) == 0 {
		return nil
	}
	if m.childrenLoaded == nil {
		m.childrenLoaded = make(map[int]bool)
	}
	if m.childrenLoading == nil {
		m.childrenLoading = make(map[int]bool)
	}
	if m.childrenExpanded == nil {
		m.childrenExpanded = make(map[int]bool)
	}

	first := m.comments[0]
	if first == nil || len(first.Item.Kids) == 0 {
		slog.Debug("loadFirstCommentTree skip", "reason", "no kids")
		return nil
	}
	id := first.Item.ID
	if m.childrenLoaded[id] || m.childrenLoading[id] {
		slog.Debug("loadFirstCommentTree skip", "id", id, "loaded", m.childrenLoaded[id], "loading", m.childrenLoading[id])
		return nil
	}

	slog.Debug("loadFirstCommentTree firing", "id", id, "kids", len(first.Item.Kids))
	return m.ensureSubtree(first)
}

// ensureSubtree asks the client for (or subscribes to) a two-phase fetch of
// the given parent's subtree. If the client already has a fully loaded
// snapshot, we attach it synchronously and return nil. Otherwise we mark
// loading and return a command that waits for the client's done signal
// and then emits a subtreeDoneMsg. Shared by first-comment preload and
// user-initiated expand.
func (m *model) ensureSubtree(parent *hn.Comment) tea.Cmd {
	if parent == nil || m.detail == nil {
		return nil
	}
	if m.childrenLoaded == nil {
		m.childrenLoaded = make(map[int]bool)
	}
	if m.childrenLoading == nil {
		m.childrenLoading = make(map[int]bool)
	}

	parentID := parent.Item.ID
	kids := append([]int(nil), parent.Item.Kids...)
	storyID := m.detail.ID
	snapshot, done, complete := m.client.EnsureSubtree(parentID, kids, parent.Depth+1)

	if complete {
		parent.Children = snapshot
		m.childrenLoaded[parentID] = true
		delete(m.childrenLoading, parentID)
		m.rebuildCommentView()
		return nil
	}

	m.childrenLoading[parentID] = true
	m.rebuildCommentView()
	client := m.client
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		<-done
		children, phase := client.SubtreeSnapshot(parentID)
		return subtreeDoneMsg{
			storyID:  storyID,
			parentID: parentID,
			children: children,
			phase:    phase,
		}
	})
}

func findCommentByID(comments []*hn.Comment, id int) *hn.Comment {
	for _, c := range comments {
		if c == nil {
			continue
		}
		if c.Item.ID == id {
			return c
		}
		if found := findCommentByID(c.Children, id); found != nil {
			return found
		}
	}
	return nil
}

func mergeTopComments(current, fresh []*hn.Comment) []*hn.Comment {
	byID := make(map[int]*hn.Comment, len(current))
	for _, c := range current {
		if c != nil {
			byID[c.Item.ID] = c
		}
	}

	merged := make([]*hn.Comment, 0, len(fresh))
	for _, c := range fresh {
		if c == nil {
			continue
		}
		if existing := byID[c.Item.ID]; existing != nil {
			c.Children = existing.Children
		}
		merged = append(merged, c)
	}
	return merged
}

func loadedTopCommentIDs(comments []*hn.Comment) map[int]bool {
	loaded := make(map[int]bool)
	for _, c := range comments {
		if c != nil && len(c.Children) > 0 {
			loaded[c.Item.ID] = true
		}
	}
	return loaded
}

// applyCommentHighlight assembles the viewport content with selection bar.
// This is cheap — no re-rendering of HTML/Glamour.
// Does NOT touch scroll position — callers handle scrolling.
func (m *model) applyCommentHighlight() {
	content := assembleView(m.flatComments, m.commentCursor, m.width)
	if m.storyBodyRendered != "" {
		content = m.storyBodyRendered + content
	}
	m.viewport.SetContent(content)
}

func (m *model) collapseAll(comments []*hn.Comment) {
	for _, c := range comments {
		m.collapsed[c.Item.ID] = true
		m.collapseAll(c.Children)
	}
}

// scrollToComment keeps the selected comment visible with minimal movement.
// This follows the less/vim-style scrolloff behavior: preserve context when
// moving by comment, only scrolling enough to bring the target into view.
// storyBodyOffset accounts for any story body (Ask HN / Job) prepended above comments.
func (m *model) scrollToComment() {
	if m.commentCursor >= len(m.flatComments) {
		return
	}
	fc := m.flatComments[m.commentCursor]
	startLine := fc.startLine + m.storyBodyOffset
	endLine := fc.endLine + m.storyBodyOffset

	top := m.viewport.YOffset()
	height := m.viewport.Height()
	if height <= 0 {
		return
	}
	bottom := top + height

	const (
		marginTop    = 1
		marginBottom = 2
	)

	commentHeight := endLine - startLine
	if commentHeight >= height-marginTop-marginBottom {
		if startLine < top || startLine >= bottom {
			m.viewport.SetYOffset(max(0, startLine-marginTop))
		}
		return
	}

	if startLine < top+marginTop {
		m.viewport.SetYOffset(max(0, startLine-marginTop))
	} else if endLine > bottom-marginBottom {
		m.viewport.SetYOffset(max(0, endLine-height+marginBottom))
	}
}

func (m model) renderToast() string {
	maxWidth := m.width - 8
	if maxWidth < 12 {
		maxWidth = 12
	}
	if maxWidth > 72 {
		maxWidth = 72
	}

	title := lipgloss.NewStyle().Foreground(currentTheme.Warning).Bold(true).Render("Translation unavailable")
	message := lipgloss.NewStyle().Foreground(currentTheme.Muted).Width(maxWidth).Render(m.toast)
	body := lipgloss.JoinVertical(lipgloss.Left, title, message)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(currentTheme.Warning).
		Padding(0, 2).
		Render(body)
}

// renderHelpOverlay builds the centered help modal content.
func (m model) renderHelpOverlay() string {
	// Leave room for border (2) + padding (2×3=6) + margin (4)
	maxContentW := m.width - 12
	if maxContentW < 36 {
		maxContentW = 36
	}

	h := m.help
	h.ShowAll = true
	h.SetWidth(maxContentW)

	titleStyle := lipgloss.NewStyle().Foreground(currentTheme.Accent).Bold(true)
	dismissStyle := lipgloss.NewStyle().Foreground(currentTheme.Muted)

	title := titleStyle.Render("Key Bindings")
	dismiss := dismissStyle.Render("press ? or esc to close")

	var keys string
	if m.state == stateDetail {
		keys = h.View(m.detailKeys)
	} else {
		keys = h.View(m.listKeys)
	}

	body := lipgloss.JoinVertical(lipgloss.Left, title, "", keys, "", dismiss)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(currentTheme.Accent).
		Padding(1, 3).
		Render(body)
}

// overlayCenter centers popup on top of base within a width×height terminal canvas.
// The base string is treated as a fixed grid; popup lines replace the corresponding
// columns of base lines at the center position.
func overlayCenter(base, popup string, width, height int) string {
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}

	popupLines := strings.Split(popup, "\n")
	popupW := 0
	for _, l := range popupLines {
		if w := lipgloss.Width(l); w > popupW {
			popupW = w
		}
	}
	popupH := len(popupLines)

	startRow := (height - popupH) / 2
	startCol := (width - popupW) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	for i, pLine := range popupLines {
		row := startRow + i
		if row >= len(baseLines) {
			break
		}
		// Truncate base line at startCol (ANSI-aware), pad to exact column, append popup line.
		left := ansi.Truncate(baseLines[row], startCol, "")
		leftW := lipgloss.Width(left)
		if leftW < startCol {
			left += strings.Repeat(" ", startCol-leftW)
		}
		baseLines[row] = left + pLine
	}

	return strings.Join(baseLines, "\n")
}

func overlayToast(base, popup string, width, height int) string {
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}

	popupLines := strings.Split(popup, "\n")
	popupW := 0
	for _, l := range popupLines {
		if w := lipgloss.Width(l); w > popupW {
			popupW = w
		}
	}
	popupH := len(popupLines)

	startRow := height - popupH - 2
	startCol := (width - popupW) / 2
	if startRow < 1 {
		startRow = 1
	}
	if startCol < 0 {
		startCol = 0
	}

	for i, pLine := range popupLines {
		row := startRow + i
		if row >= len(baseLines) {
			break
		}
		left := ansi.Truncate(baseLines[row], startCol, "")
		leftW := lipgloss.Width(left)
		if leftW < startCol {
			left += strings.Repeat(" ", startCol-leftW)
		}
		baseLines[row] = left + pLine
	}

	return strings.Join(baseLines, "\n")
}

func runApp(cat hn.Category) error {
	p := tea.NewProgram(newModel(cat))
	_, err := p.Run()
	return err
}
