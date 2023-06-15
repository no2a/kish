package kish

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func ut(sec int64) *jwt.NumericDate {
	return jwt.NewNumericDate(time.Unix(sec, 0))
}

func TestProxyClaimsValidate(t *testing.T) {
	p := []struct {
		title string
		err   error
		ntb   *jwt.NumericDate
		eat   *jwt.NumericDate
		id    string
	}{
		{"ok", nil, ut(10000), ut(10600), "id"},
		{"too long", ErrLifetimeTooLong, ut(10000), ut(10601), "id"},
		{"without ExpiredAt", ErrNotContainRequiredClaims, ut(23456789), nil, "id"},
		{"without NotBefore", ErrNotContainRequiredClaims, nil, ut(1234567890), "id"},
		{"without ID", ErrNotContainRequiredClaims, ut(10000), ut(10600), ""},
	}
	for _, i := range p {
		t.Run(i.title, func(t *testing.T) {
			claims := proxyClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					NotBefore: i.ntb,
					ExpiresAt: i.eat,
					ID:        i.id,
				},
			}
			err := claims.Validate()
			if i.err == nil {
				if err != nil {
					t.Errorf("err should be nil: %+v", err)
				}
			} else {
				if !errors.Is(err, i.err) {
					t.Errorf("err is not unexpected: %+v", err)
				}
			}
		})
	}
}

func TestValidateToken(t *testing.T) {
	keyValue := "abc"
	keyID := "z"
	ts := TokenSet{
		Tokens: &map[string]string{
			keyID: keyValue,
		},
	}
	tokenStr, err1 := GenerateToken(time.Now(), &ProxyParameters{}, []byte(keyValue), keyID)
	if err1 != nil {
		t.Errorf("err1: %+v", err1)
	}
	_, err2 := validateToken(tokenStr, &ts)
	if err2 != nil {
		t.Errorf("err2: %+v", err2)
	}
}

func TestValidateTokenKeyIDBad(t *testing.T) {
	keyValue := "abc"
	keyID := "z"
	keyIDBad := "zbad"
	ts := TokenSet{
		Tokens: &map[string]string{
			keyID: keyValue,
		},
	}
	tokenStr, err1 := GenerateToken(time.Now(), &ProxyParameters{}, []byte(keyValue), keyIDBad)
	if err1 != nil {
		t.Errorf("err1: %+v", err1)
	}
	_, err2 := validateToken(tokenStr, &ts)
	if !errors.Is(err2, jwt.ErrTokenUnverifiable) {
		t.Errorf("err2 is unexpected: %+v", err2)
	}
}

func TestValidateTokenKeyValueBad(t *testing.T) {
	keyValue := "abc"
	keyValueBad := "bad"
	keyID := "z"
	ts := TokenSet{
		Tokens: &map[string]string{
			keyID: keyValue,
		},
	}
	tokenStr, err1 := GenerateToken(time.Now(), &ProxyParameters{}, []byte(keyValueBad), keyID)
	if err1 != nil {
		t.Errorf("err1: %+v", err1)
	}
	_, err2 := validateToken(tokenStr, &ts)
	if !errors.Is(err2, jwt.ErrSignatureInvalid) {
		t.Errorf("err2 is unexpected: %+v", err2)
	}
}
