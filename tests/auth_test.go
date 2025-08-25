package tests

import (
	"net/http"
	"testing"
	"time"

	"github.com/R0Xps/chirpy/internal/auth"
	"github.com/google/uuid"
)

func TestHashPassword(t *testing.T) {
	password := "T3st$Passw0rd_1231wq"
	hash1, err := auth.HashPassword(password)
	if err != nil {
		t.Errorf("TestHashPassword (fist hash): %s", err)
		return
	}

	hash2, err := auth.HashPassword(password)
	if err != nil {
		t.Errorf("TestHashPassword (second hash): %s", err)
		return
	}

	err = auth.CheckPasswordHash(password, hash1)
	if err != nil {
		t.Errorf("hash1 doesnt match password")
		return
	}

	err = auth.CheckPasswordHash(password, hash2)
	if err != nil {
		t.Errorf("hash2 doesnt match password")
		return
	}
}

func TestJWT(t *testing.T) {
	correctSecret := "Correct Secret"
	incorrectSecret := "Incorrect Secret"
	userID := uuid.MustParse("73751986-2d53-4ac8-9d02-3889c71879db")

	longToken, err := auth.MakeJWT(userID, correctSecret, time.Hour)
	if err != nil {
		t.Errorf("Error making long token: %s", err)
		return
	}

	uuid, err := auth.ValidateJWT(longToken, correctSecret)
	if err != nil {
		t.Errorf("Error validating long token with correct secret: %s", err)
		return
	}

	if uuid != userID {
		t.Errorf("Error validating long token with correct secret: UUIDs do not match:\noriginal: %s\nreturned: %s", userID, uuid)
		return
	}

	uuid, err = auth.ValidateJWT(longToken, incorrectSecret)
	if err == nil {
		t.Errorf("Error validating long token with incorrect secret: Validation succeeded (should have failed), returned UUID: %s", uuid)
		return
	}

	shortToken, err := auth.MakeJWT(userID, correctSecret, time.Second)
	if err != nil {
		t.Errorf("Error making short token: %s", err)
		return
	}
	time.Sleep(time.Second * 3)

	uuid, err = auth.ValidateJWT(shortToken, correctSecret)
	if err == nil {
		t.Errorf("Error validating short token: Validation succeeded (token should have expired), returned UUID: %s", uuid)
		return
	}
}

func TestGetBearerToken(t *testing.T) {
	correctToken := "TestToken"
	headers := http.Header{}

	token, err := auth.GetBearerToken(headers)
	if err == nil {
		t.Errorf("Somehow found token when it was not provided: %s", token)
		return
	}

	headers.Set("Authorization", "Bearer "+correctToken)
	token, err = auth.GetBearerToken(headers)
	if err != nil {
		t.Errorf("Error getting bearer token: %s", err)
		return
	}

	if token != correctToken {
		t.Errorf("Error getting bearer token: incorrect token returned\nexpected: %s\nfound: %s", correctToken, token)
		return
	}
}
