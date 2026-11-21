package simplewhitelist

import (
	"os"
	"testing"

	"go.minekube.com/brigodier"
	"go.minekube.com/gate/pkg/util/uuid"
)

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

func TestLoginEvaluationBypass(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "simplewhitelist_login_*")
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	_ = mgr.Load()

	targetUUID, _ := uuid.Parse("11111111-1111-1111-1111-111111111111")
	_ = mgr.SetWhitelistEnabled(true)

	// Unlisted player without bypass is blocked
	p := newMockPlayer("UnlistedPlayer", targetUUID, "1.1.1.1:1234")
	isWhitelisted, _ := mgr.IsWhitelisted(p.Username(), p.ID(), p.RemoteAddr())
	if isWhitelisted {
		t.Errorf("Expected unlisted player to be denied by whitelist")
	}

	// Player with simplewhitelist.bypass permission is allowed
	p.permissions["simplewhitelist.bypass"] = true
	if !p.HasPermission("simplewhitelist.bypass") {
		t.Errorf("Expected bypass permission to be true")
	}
}
