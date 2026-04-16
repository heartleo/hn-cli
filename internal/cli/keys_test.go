package cli

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func TestDetailQuitRequiresUppercaseQ(t *testing.T) {
	keys := newDetailKeyMap()
	if key.Matches(tea.KeyPressMsg{Code: 'q'}, keys.Quit) {
		t.Fatal("expected lowercase q not to quit in detail view")
	}
	if !key.Matches(tea.KeyPressMsg{Code: 'Q'}, keys.Quit) {
		t.Fatal("expected uppercase Q to quit in detail view")
	}
}

func TestListQuitStillAllowsLowercaseQ(t *testing.T) {
	keys := newListKeyMap()
	if !key.Matches(tea.KeyPressMsg{Code: 'q'}, keys.Quit) {
		t.Fatal("expected lowercase q to quit in list view")
	}
}

func TestHelpUsesQuestionMark(t *testing.T) {
	listKeys := newListKeyMap()
	detailKeys := newDetailKeyMap()
	if !key.Matches(tea.KeyPressMsg{Code: '?'}, listKeys.Help) {
		t.Fatal("expected ? to toggle list help")
	}
	if !key.Matches(tea.KeyPressMsg{Code: '?'}, detailKeys.Help) {
		t.Fatal("expected ? to toggle detail help")
	}
	if key.Matches(tea.KeyPressMsg{Code: 'h'}, listKeys.Help) || key.Matches(tea.KeyPressMsg{Code: 'h'}, detailKeys.Help) {
		t.Fatal("expected h not to toggle help")
	}
}
