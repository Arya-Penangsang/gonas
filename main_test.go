package main

import (
	"strings"
	"testing"
)

// TestSafePathValidation verifies that safePath prevents directory traversal (CWE-22)
func TestSafePathValidation(t *testing.T) {
	tempDir := t.TempDir()
	configMu.Lock()
	cfg.RootPath = tempDir
	configMu.Unlock()

	testCases := []struct {
		inputPath string
		expectErr bool
	}{
		{"", false},
		{"documents/report.pdf", false},
		{"photos/sub/2026.jpg", false},
		{"../etc/passwd", true},
		{"../../windows/system32/cmd.exe", true},
		{"..", true},
		{"subfolder/test.txt", false},
	}

	for _, tc := range testCases {
		p, err := safePath(tc.inputPath)
		hasErr := err != nil
		if hasErr != tc.expectErr {
			t.Errorf("safePath(%q) = %q, err=%v; expected error=%v", tc.inputPath, p, err, tc.expectErr)
		}
	}
}

// TestSaltedPasswordHash verifies salted password hashing & constant time comparison
func TestSaltedPasswordHash(t *testing.T) {
	pass := "SecurePassword123!"
	hashed := hashPassword(pass)

	if !strings.HasPrefix(hashed, "sha256$") {
		t.Fatalf("Expected hash prefix 'sha256$', got %s", hashed)
	}

	if !verifyPassword(pass, hashed) {
		t.Errorf("Failed to verify valid password against hash")
	}

	if verifyPassword("WrongPassword", hashed) {
		t.Errorf("Incorrectly validated wrong password")
	}
}

// TestUserStore verifies user authentication and CRUD operations in UserStore
func TestUserStore(t *testing.T) {
	store := &UserStore{
		users: make(map[string]*UserAccount),
	}

	// Add test user
	err := store.Add(&UserAccount{
		Username:    "tester",
		DisplayName: "Test User",
		Password:    "TesterPass99!",
		Role:        RoleAdmin,
	})
	if err != nil {
		t.Fatalf("Failed to add user: %v", err)
	}

	// Verify authentication
	user := store.Authenticate("tester", "TesterPass99!")
	if user == nil {
		t.Fatalf("Authenticate returned nil for valid user")
	}
	if user.Role != RoleAdmin {
		t.Errorf("Expected role Admin, got %s", user.Role)
	}

	// Update user
	err = store.Update("tester", "Tester Updated", RoleUser, "NewTesterPass123!")
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	// Verify update
	updatedUser := store.Authenticate("tester", "NewTesterPass123!")
	if updatedUser == nil {
		t.Fatalf("Authenticate returned nil after password update")
	}
	if updatedUser.DisplayName != "Tester Updated" || updatedUser.Role != RoleUser {
		t.Errorf("User update values mismatch: %+v", updatedUser)
	}
}
