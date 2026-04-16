package cli

import "charm.land/bubbles/v2/key"

type listKeyMap struct {
	Open         key.Binding
	Read         key.Binding
	Translate    key.Binding
	TranslateAll key.Binding
	NextTab      key.Binding
	PrevTab      key.Binding
	Refresh      key.Binding
	Help         key.Binding
	Quit         key.Binding
}

func newListKeyMap() listKeyMap {
	return listKeyMap{
		Open: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open"),
		),
		Read: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "read"),
		),
		Translate: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "translate"),
		),
		TranslateAll: key.NewBinding(
			key.WithKeys("T"),
			key.WithHelp("T", "translate all"),
		),
		NextTab: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("←/→", "switch tab"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("", ""),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

func (k listKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Read, k.Open, k.Translate, k.TranslateAll, k.NextTab, k.Refresh, k.Help, k.Quit}
}

func (k listKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Read, k.Open, k.Translate, k.TranslateAll},
		{k.NextTab, k.Refresh},
		{k.Help, k.Quit},
	}
}

type detailKeyMap struct {
	Up          key.Binding
	Down        key.Binding
	BackToTop   key.Binding
	GotoRoot    key.Binding
	Open        key.Binding
	Translate   key.Binding
	Replies     key.Binding
	Refresh     key.Binding
	Back        key.Binding
	Collapse    key.Binding
	CollapseAll key.Binding
	ExpandAll   key.Binding
	Help        key.Binding
	Quit        key.Binding
}

func newDetailKeyMap() detailKeyMap {
	return detailKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k ↓/j", "navigate comments"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("", ""),
		),
		BackToTop: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("gg", "top"),
		),
		GotoRoot: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "go to root"),
		),
		Open: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open in browser"),
		),
		Translate: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "translate"),
		),
		Replies: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "replies"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "refresh"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Collapse: key.NewBinding(
			key.WithKeys("space"),
			key.WithHelp("space", "fold/unfold"),
		),
		CollapseAll: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C/E", "fold/unfold all"),
		),
		ExpandAll: key.NewBinding(
			key.WithKeys("E"),
			key.WithHelp("", ""),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("Q", "ctrl+c"),
			key.WithHelp("Q", "quit"),
		),
	}
}

func (k detailKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.BackToTop, k.GotoRoot, k.Open, k.Translate, k.Replies, k.Refresh, k.Collapse, k.CollapseAll, k.Back, k.Quit}
}

func (k detailKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.BackToTop, k.GotoRoot, k.Open, k.Translate, k.Replies, k.Refresh, k.Collapse, k.CollapseAll},
		{k.Back},
		{k.Help, k.Quit},
	}
}
