package bcrypt

import (
	"testing"
)

func TestHashAndCompare(t *testing.T) {
	password := "mySecret123!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("unexpected error hashing password: %v", err)
	}

	if len(hash) < 50 {
		t.Errorf("hash length too short: got %d", len(hash))
	}

	if err := ComparePassword(hash, password); err != nil {
		t.Errorf("expected password to match, got error: %v", err)
	}

	if err := ComparePassword(hash, "wrongPassword"); err == nil {
		t.Error("expected mismatch error, got nil")
	}
}

func TestNeedsRehash(t *testing.T) {
	password := "rehashTest"
	hash, err := HashPasswordWithCost(password, DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	need, err := NeedsRehash(hash, DefaultCost)
	if err != nil {
		t.Fatalf("unexpected error from NeedsRehash: %v", err)
	}
	if need {
		t.Error("expected no rehash need for same cost")
	}

	need, err = NeedsRehash(hash, DefaultCost+1)
	if err != nil {
		t.Fatalf("unexpected error from NeedsRehash: %v", err)
	}
	if !need {
		t.Error("expected rehash need for different cost")
	}
}

func TestCheckAndRehash(t *testing.T) {
	password := "checkAndRehashTest"
	hash, err := HashPasswordWithCost(password, DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	// No rehash expected when cost same
	newHash, rehashed, err := CheckAndRehash(hash, password, DefaultCost)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rehashed {
		t.Error("expected no rehash when cost same")
	}
	if newHash != "" {
		t.Error("expected empty new hash when no rehash done")
	}

	// Force rehash with new cost
	newHash, rehashed, err = CheckAndRehash(hash, password, DefaultCost+1)
	if err != nil {
		t.Fatalf("unexpected error during rehash: %v", err)
	}
	if !rehashed {
		t.Error("expected rehash true when cost differs")
	}
	if newHash == "" {
		t.Error("expected non-empty new hash")
	}
}

func TestIsBcryptHash(t *testing.T) {
	valid, _ := HashPassword("validTest")
	if !IsBcryptHash(valid) {
		t.Error("expected valid bcrypt hash to return true")
	}

	invalid := "not-a-bcrypt-hash"
	if IsBcryptHash(invalid) {
		t.Error("expected invalid string to return false")
	}
}

func TestMustHash(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()
	_ = MustHash("panicTest")
}

func TestCompareSafe(t *testing.T) {
	password := "safeTest"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !CompareSafe(hash, password) {
		t.Error("expected CompareSafe to return true for correct password")
	}
	if CompareSafe(hash, "wrong") {
		t.Error("expected CompareSafe to return false for wrong password")
	}
}
