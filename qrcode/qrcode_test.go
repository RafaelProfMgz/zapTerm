package qrcode

import "testing"

// Matrix must keep the quiet zone: without the 4-module border phones fail to
// read the code, which was the reason the terminal rendering never scanned.
func TestMatrixKeepsQuietZone(t *testing.T) {
	rows, err := Matrix("https://example.com/whatscli-pair-code")
	if err != nil {
		t.Fatalf("Matrix: %v", err)
	}
	if len(rows) < 21+8 {
		t.Fatalf("matriz pequena demais: %d linhas", len(rows))
	}
	for i, row := range rows {
		if len(row) != len(rows) {
			t.Fatalf("linha %d tem %d colunas, esperado %d (quadrado)", i, len(row), len(rows))
		}
		for _, c := range row {
			if c != '0' && c != '1' {
				t.Fatalf("linha %d tem caractere inválido %q", i, c)
			}
		}
	}
	quiet := func(row string) bool {
		for _, c := range row {
			if c != '0' {
				return false
			}
		}
		return true
	}
	for i := 0; i < 4; i++ {
		if !quiet(rows[i]) {
			t.Fatalf("linha %d deveria ser zona silenciosa", i)
		}
		if !quiet(rows[len(rows)-1-i]) {
			t.Fatalf("linha %d (do fim) deveria ser zona silenciosa", i)
		}
		for _, row := range rows {
			if row[i] != '0' || row[len(row)-1-i] != '0' {
				t.Fatal("as colunas da borda deveriam ser zona silenciosa")
			}
		}
	}
}
