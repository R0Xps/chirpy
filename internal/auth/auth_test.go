package auth

import "testing"

func TestHashPassword(t *testing.T) {
	password := "T3st$Passw0rd_1231wq"
	hash1, err := HashPassword(password)
	if err != nil {
		t.Errorf("TestHashPassword (fist hash): %s", err)
	}

	hash2, err := HashPassword(password)
	if err != nil {
		t.Errorf("TestHashPassword (second hash): %s", err)
	}

	err = CheckPasswordHash(password, hash1)
	if err != nil {
		t.Errorf("hash1 doesnt match password")
	}

	err = CheckPasswordHash(password, hash2)
	if err != nil {
		t.Errorf("hash2 doesnt match password")
	}
}
