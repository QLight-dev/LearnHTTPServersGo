package auth

import (
	"errors"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

func CheckPasswordHash(password string, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		Subject:   userID.String(),
	},
	)

	jwt, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}

	return jwt, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	type Claims struct {
		jwt.RegisteredClaims
	}

	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(*jwt.Token) (any, error) {
			return []byte(tokenSecret), nil
		},
	)
	if err != nil {
		return uuid.Nil, err
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return uuid.Nil, errors.New("token is invalid")
	}

	UserID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, errors.New("UserID is invalid")
	}

	return UserID, nil
}
