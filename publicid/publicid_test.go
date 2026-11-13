package publicid

import "testing"

func TestNewDefaults(t *testing.T) {
	id, err := New(NewParams{})
	if err != nil {
		t.Fatal(err)
	}
	if !Validate(id, AlphabetNumbersCharactersWithUppercase, SizeShort) {
		t.Fatalf("default id %q is not a valid short id", id)
	}
}

func TestNewCustom(t *testing.T) {
	id, err := New(NewParams{Size: SizeLong, Alphabet: "ab"})
	if err != nil {
		t.Fatal(err)
	}
	if !Validate(id, "ab", SizeLong) {
		t.Fatalf("id %q does not match custom size/alphabet", id)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"valid", "laimfSFwxPNt", true},
		{"too short", "laimfSFwxPN", false},
		{"too long", "laimfSFwxPNtX", false},
		{"invalid char", "laimfSFwx-Nt", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Validate(tt.id, AlphabetNumbersCharactersWithUppercase, SizeShort); got != tt.want {
				t.Errorf("Validate(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}
