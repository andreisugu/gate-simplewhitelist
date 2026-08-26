package simplewhitelist

import (
	"os"
	"strings"
	"testing"

	"go.minekube.com/brigodier"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/util/uuid"
)

type mockCommandSource struct {
	messages    []string
	permissions map[string]bool
}

func (m *mockCommandSource) SendMessage(msg c.Component) error {
	var b strings.Builder
	if t, ok := msg.(*c.Text); ok {
		b.WriteString(t.Content)
		for _, child := range t.Extra {
			if tc, ok := child.(*c.Text); ok {
				b.WriteString(tc.Content)
			}
		}
	}
	m.messages = append(m.messages, b.String())
	return nil
}

func (m *mockCommandSource) HasPermission(permission string) bool {
	return m.permissions[permission] || m.permissions["*"]
}

func TestCommandsIntegration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "simplewhitelist_cmd_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	if err := mgr.Load(); err != nil {
		t.Fatal(err)
	}

	// 1. Whitelist add / remove / list / on / off
	_, _, added, err := mgr.AddWhitelist("PlayerOne")
	if err != nil || !added {
		t.Fatalf("Failed to add PlayerOne: %v", err)
	}

	st := mgr.WhitelistState()
	if _, ok := st.Nicks["playerone"]; !ok {
		t.Errorf("Expected PlayerOne in whitelist state")
	}

	_, _, removed, err := mgr.RemoveWhitelist("PlayerOne")
	if err != nil || !removed {
		t.Fatalf("Failed to remove PlayerOne: %v", err)
	}

	st = mgr.WhitelistState()
	if _, ok := st.Nicks["playerone"]; ok {
		t.Errorf("Expected PlayerOne removed from whitelist state")
	}

	// 2. Blacklist add / remove / list / on / off
	_, _, added, err = mgr.AddBlacklist("Spammer")
	if err != nil || !added {
		t.Fatalf("Failed to add Spammer to blacklist: %v", err)
	}

	bst := mgr.BlacklistState()
	if _, ok := bst.Nicks["spammer"]; !ok {
		t.Errorf("Expected Spammer in blacklist state")
	}

	_, _, removed, err = mgr.RemoveBlacklist("Spammer")
	if err != nil || !removed {
		t.Fatalf("Failed to remove Spammer: %v", err)
	}

	bst = mgr.BlacklistState()
	if _, ok := bst.Nicks["spammer"]; ok {
		t.Errorf("Expected Spammer removed from blacklist state")
	}
}

func TestSuggestionsBuilderRange(t *testing.T) {
	candidates := []string{"Steve", "Alex", "AdminUser"}
	fn := suggestCandidates(func() []string { return candidates })

	builder := &brigodier.SuggestionsBuilder{
		Input:          "whitelist remove St",
		InputLowerCase: "whitelist remove st",
		Start:          17,
	}

	res := fn.Suggestions(nil, builder)
	if len(res.Suggestions) != 1 || res.Suggestions[0].Text != "Steve" {
		t.Fatalf("Expected suggestion 'Steve', got %v", res.Suggestions)
	}
	if res.Suggestions[0].Range.Start != 17 || res.Suggestions[0].Range.End != 19 {
		t.Fatalf("Expected exact replacement range [17, 19], got [%d, %d]", res.Suggestions[0].Range.Start, res.Suggestions[0].Range.End)
	}
}

func TestLoginEvaluationPrecedence(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "simplewhitelist_login_*")
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	_ = mgr.Load()

	targetUUID, _ := uuid.Parse("11111111-1111-1111-1111-111111111111")
	_ = mgr.SetWhitelistEnabled(true)
	_ = mgr.SetBlacklistEnabled(true)

	_, _, _, _ = mgr.AddWhitelist("WhitelistedPlayer")
	_, _, _, _ = mgr.AddBlacklist("WhitelistedPlayer") // Also blacklisted!

	p := newMockPlayer("WhitelistedPlayer", targetUUID, "1.1.1.1:1234")

	// Blacklist takes precedence over whitelist: player is blocked!
	isBlacklisted, _ := mgr.IsBlacklisted(p.Username(), p.ID(), p.RemoteAddr())
	if !isBlacklisted {
		t.Errorf("Expected blacklisted player to be caught by blacklist")
	}

	// If player has simpleblacklist.bypass, blacklist is bypassed
	p.permissions["simpleblacklist.bypass"] = true
	isWhitelisted, _ := mgr.IsWhitelisted(p.Username(), p.ID(), p.RemoteAddr())
	if !isWhitelisted {
		t.Errorf("Expected whitelisted player with blacklist bypass to be allowed")
	}
}
