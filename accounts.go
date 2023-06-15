package kish

import (
	"io"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type TokenSet struct {
	Path    string
	ModTime time.Time
	Tokens  *map[string]string
}

func loadStringStringMapYAML(f *os.File) (*map[string]string, time.Time, error) {
	stat, err := f.Stat()
	if err != nil {
		return nil, time.Time{}, err
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, time.Time{}, err
	}
	var m map[string]string
	err = yaml.Unmarshal(b, &m)
	if err != nil {
		return nil, time.Time{}, err
	}
	return &m, stat.ModTime(), nil
}

func (ts *TokenSet) ensureLoaded() {
	var err error
	defer func() {
		if err != nil {
			log.Printf("TokenSet loadStringStringMapYAML: %s", err)
		}
	}()
	if ts.Path == "" {
		return
	}
	f, err := os.Open(ts.Path)
	if err != nil {
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return
	}
	if !ts.ModTime.Equal(stat.ModTime()) {
		m, modtime, err := loadStringStringMapYAML(f)
		if err != nil {
			return
		}
		ts.Tokens = m
		ts.ModTime = modtime
		log.Printf("TokenSet was successfully loaded from %s", ts.Path)
	}
}

func (ts *TokenSet) Get(keyID string) []byte {
	ts.ensureLoaded()
	if ts.Tokens == nil {
		return nil
	}
	key := (*ts.Tokens)[keyID]
	if key == "" {
		return nil
	}
	return []byte(key)
}

func (ts *TokenSet) validate(token string) bool {
	ts.ensureLoaded()
	if ts.Tokens == nil {
		return false
	}
	for _, key := range *ts.Tokens {
		if key == token {
			return true
		}
	}
	return false
}
