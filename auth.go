package kish

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type ProxyParameters struct {
	Host      string            `json:"host"`
	AllowIP   []string          `json:"allowIP"`
	BasicAuth map[string]string `json:"basicAuth"`
	AllowMyIP bool              `json:"allowMyIP"`
}

type proxyClaims struct {
	KeyID string `json:"keyID"`
	ProxyParameters
	jwt.RegisteredClaims
}

func (c *proxyClaims) GetKeyID() string {
	return c.KeyID
}

func validateToken(t string, ts *TokenSet) (*ProxyParameters, error) {
	keyfunc := func(token *jwt.Token) (interface{}, error) {
		keyID := token.Claims.(HasKeyID).GetKeyID()
		key := ts.Get(keyID)
		if key == nil {
			return nil, errors.New("key not found")
		}
		return key, nil
	}
	token, err := jwt.ParseWithClaims(t, &proxyClaims{}, keyfunc)
	if err != nil {
		return nil, err
	}
	claims := token.Claims.(*proxyClaims)
	if claims.ExpiresAt == nil || claims.NotBefore == nil || claims.ID == "" {
		return nil, errors.New("invalid token")
	}
	if claims.ExpiresAt.Time.Unix()-claims.NotBefore.Time.Unix() > 600 {
		// too long
		return nil, errors.New("invalid token")
	}
	err = claims.Valid()
	if err != nil {
		return nil, errors.New("invalid token")
	}
	return &claims.ProxyParameters, nil
}

func GenerateToken(now time.Time, params *ProxyParameters, key []byte, keyID string) (string, error) {
	claims := proxyClaims{
		keyID,
		*params,
		jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(300 * time.Second)),
			NotBefore: jwt.NewNumericDate(now.Add(-300 * time.Second)),
			ID:        uuid.New().String(),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
}
