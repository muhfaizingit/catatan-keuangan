package util

import (
	"strconv"
	"strings"
	"time"
)

// FormatThousands memformat angka dengan pemisah ribuan titik (gaya Indonesia).
// Contoh: 1500000 -> "1.500.000".
func FormatThousands(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)

	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	// Sisipkan titik setiap 3 digit dari belakang.
	rem := len(s) % 3
	if rem > 0 {
		b.WriteString(s[:rem])
		if len(s) > rem {
			b.WriteByte('.')
		}
	}
	for i := rem; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte('.')
		}
	}
	return b.String()
}

// Rupiah memformat angka menjadi "Rp 1.500.000".
func Rupiah(n int64) string {
	return "Rp " + FormatThousands(n)
}

// ParseRupiah mengambil seluruh digit dari input (mengabaikan titik, koma,
// spasi, "Rp") lalu mengembalikannya sebagai int64.
func ParseRupiah(s string) int64 {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return 0
	}
	n, _ := strconv.ParseInt(b.String(), 10, 64)
	return n
}

// ParseDate mengurai tanggal format "2006-01-02" (dari input type=date).
// Bila kosong/invalid, mengembalikan tanggal hari ini (lokal).
func ParseDate(s string, now time.Time) time.Time {
	t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(s), time.Local)
	if err != nil {
		return now
	}
	return t
}

// NamaBulan mengembalikan nama bulan Indonesia untuk angka 1-12.
func NamaBulan(b int) string {
	names := []string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	if b < 1 || b > 12 {
		return ""
	}
	return names[b]
}
