package kish

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrNotContainRequiredClaims = errors.New("not contain required claims")
	ErrLifetimeTooLong          = errors.New("lifetime is too long")
	ErrKeyNotFound              = errors.New("key not found")
	ErrInvalidToken             = errors.New("invalid token")
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

func (c *proxyClaims) Validate() error {
	if c.ExpiresAt == nil || c.NotBefore == nil || c.ID == "" {
		return ErrNotContainRequiredClaims
	}
	if c.ExpiresAt.Time.Unix()-c.NotBefore.Time.Unix() > 600 {
		return ErrLifetimeTooLong
	}
	return nil
}

func validateToken(t string, ts *TokenSet) (*ProxyParameters, error) {
	keyfunc := func(token *jwt.Token) (interface{}, error) {
		keyID := token.Claims.(HasKeyID).GetKeyID()
		key := ts.Get(keyID)
		if key == nil {
			return nil, ErrKeyNotFound
		}
		return key, nil
	}
	claims := proxyClaims{}
	token, err := jwt.ParseWithClaims(t, &claims, keyfunc)
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, ErrInvalidToken
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
