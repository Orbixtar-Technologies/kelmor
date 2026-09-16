package store

import (
	"testing"
	"time"

	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/id"
)

func TestResetAdminPasswordUpdatesExistingHash(t *testing.T) {
	st := NewMemory()
	if err := SeedAdmin(st, "admin", "FirstPassword!26", "ops@example.net", true); err != nil {
		t.Fatal(err)
	}
	user := st.UserByUsername("admin")
	if user == nil || !user.MustChangePassword {
		t.Fatal("expected seeded admin that must change password")
	}
	oldHash := user.PasswordHash
	session := &Session{ID: id.New(), UserID: user.ID, TokenHash: []byte("token"), ExpiresAt: time.Now().Add(time.Hour)}
	st.PutSession(session)

	if err := ResetAdminPassword(st, "admin", "SecondPassword!26"); err != nil {
		t.Fatal(err)
	}
	updated := st.UserByUsername("admin")
	if updated.PasswordHash == oldHash {
		t.Fatal("password hash was not replaced")
	}
	if updated.MustChangePassword {
		t.Fatal("must_change_password should be cleared")
	}
	if !auth.VerifyPassword(updated.PasswordHash, "SecondPassword!26") {
		t.Fatal("new password does not verify")
	}
	if auth.VerifyPassword(updated.PasswordHash, "FirstPassword!26") {
		t.Fatal("old password still verifies")
	}
	got := st.SessionByHash([]byte("token"))
	if got != nil {
		t.Fatal("expected sessions to be revoked")
	}
}

func TestResetAdminPasswordMissingUser(t *testing.T) {
	st := NewMemory()
	if err := ResetAdminPassword(st, "admin", "SecondPassword!26"); err != ErrAdminMissing {
		t.Fatalf("got %v", err)
	}
}

func TestSeedAdminRefreshesMustChangePassword(t *testing.T) {
	st := NewMemory()
	if err := SeedAdmin(st, "admin", "FirstPassword!26", "ops@example.net", true); err != nil {
		t.Fatal(err)
	}
	if err := SeedAdmin(st, "admin", "RotatedPassword!26", "ops@example.net", true); err != nil {
		t.Fatal(err)
	}
	user := st.UserByUsername("admin")
	if user.MustChangePassword {
		t.Fatal("bootstrap rotation should allow Director login")
	}
	if !auth.VerifyPassword(user.PasswordHash, "RotatedPassword!26") {
		t.Fatal("existing must-change admin was not synced from bootstrap")
	}
}

func TestSeedAdminLeavesChangedPasswordAlone(t *testing.T) {
	st := NewMemory()
	if err := SeedAdmin(st, "admin", "FirstPassword!26", "ops@example.net", false); err != nil {
		t.Fatal(err)
	}
	if err := SeedAdmin(st, "admin", "ShouldNotApply!26", "ops@example.net", false); err != nil {
		t.Fatal(err)
	}
	user := st.UserByUsername("admin")
	if !auth.VerifyPassword(user.PasswordHash, "FirstPassword!26") {
		t.Fatal("already-usable admin password was overwritten")
	}
}
