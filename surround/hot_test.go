package surround

import (
	"log"
	"testing"
)

func TestT(t *testing.T) {
	s, err := LoadHotPairs()
	log.Println(s, err)
}
