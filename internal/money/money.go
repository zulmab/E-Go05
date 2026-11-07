package money

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseCents(value string) (int64, error) {
	whole, fraction, found := strings.Cut(value, ".")
	if !found || len(fraction) != 2 {
		return 0, fmt.Errorf("se esperaba un monto con dos decimales: %q", value)
	}

	// The generator produces non-negative amounts with two decimal places.
	for _, digit := range whole + fraction {
		if digit < '0' || digit > '9' {
			return 0, fmt.Errorf("monto inválido: %q", value)
		}
	}

	if whole == "" {
		return 0, fmt.Errorf("monto inválido: %q", value)
	}

	cents, err := strconv.ParseInt(whole+fraction, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("no se pudo convertir el monto %q: %w", value, err)
	}

	return cents, nil
}
