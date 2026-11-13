package publicid

import (
	"strings"

	nanoid "github.com/matoous/go-nanoid/v2"
)

type Alphabet = string

const (
	AlphabetNumbersCharactersWithUppercase Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

type Size = int

const (
	SizeShort  Size = 12
	SizeMedium Size = 32
	SizeLong   Size = 64
)

type NewParams struct {
	Size     Size
	Alphabet Alphabet
}

func New(params NewParams) (string, error) {
	size := SizeShort
	if params.Size != 0 {
		size = params.Size
	}

	alphabet := AlphabetNumbersCharactersWithUppercase
	if params.Alphabet != "" {
		alphabet = params.Alphabet
	}
	return nanoid.Generate(alphabet, size)
}

func Validate(publicID string, alphabet Alphabet, size Size) bool {
	if len(publicID) != size {
		return false
	}

	for _, char := range publicID {
		if !strings.ContainsRune(alphabet, char) {
			return false
		}
	}
	return true
}
