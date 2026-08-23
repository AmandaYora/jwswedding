// Package terbilang converts a Rupiah amount into Indonesian words — the
// "Terbilang" line standard on an official Indonesian kwitansi. Pure string
// utility, no dependency, hence its home in shared/ rather than any module's
// domain (.claude/rules/backend-modular-monolith.md).
package terbilang

import "strings"

var units = [...]string{
	"", "Satu", "Dua", "Tiga", "Empat", "Lima", "Enam", "Tujuh", "Delapan", "Sembilan", "Sepuluh", "Sebelas",
}

// words recursively renders a non-negative integer as Indonesian words,
// following the standard terbilang grouping (belas/puluh/ratus/ribu/juta/
// miliar/triliun).
func words(n int64) string {
	switch {
	case n < 12:
		return units[n]
	case n < 20:
		return words(n-10) + " Belas"
	case n < 100:
		return joinNonEmpty(words(n/10), "Puluh", tail(n%10))
	case n < 200:
		return joinNonEmpty("Seratus", tail(n%100))
	case n < 1000:
		return joinNonEmpty(words(n/100), "Ratus", tail(n%100))
	case n < 2000:
		return joinNonEmpty("Seribu", tail(n%1000))
	case n < 1_000_000:
		return joinNonEmpty(words(n/1000), "Ribu", tail(n%1000))
	case n < 1_000_000_000:
		return joinNonEmpty(words(n/1_000_000), "Juta", tail(n%1_000_000))
	case n < 1_000_000_000_000:
		return joinNonEmpty(words(n/1_000_000_000), "Miliar", tail(n%1_000_000_000))
	default:
		return joinNonEmpty(words(n/1_000_000_000_000), "Triliun", tail(n%1_000_000_000_000))
	}
}

func tail(n int64) string {
	if n == 0 {
		return ""
	}
	return words(n)
}

func joinNonEmpty(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}

// Rupiah renders a non-negative Rupiah amount as Indonesian words, e.g.
// 2500000 -> "Dua Juta Lima Ratus Ribu Rupiah" — the "Terbilang" line on the
// Kwitansi PDF. amount is expected non-negative (client_payments.amount is
// BIGINT UNSIGNED); a non-positive input renders "Nol Rupiah" rather than
// recursing on a negative/zero value.
func Rupiah(amount int64) string {
	if amount <= 0 {
		return "Nol Rupiah"
	}
	return words(amount) + " Rupiah"
}
