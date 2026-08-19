package tax

import (
	"errors"
	"math/big"
)

func CalculateLine(amount int64, rateBPS int, category, mode string) (base, taxAmount, total int64, err error) {
	if amount < 0 {
		return 0, 0, 0, errors.New("tax amount cannot be negative")
	}
	if rateBPS < 0 || rateBPS > 10000 {
		return 0, 0, 0, errors.New("tax rate must be between 0 and 10000 bps")
	}
	switch category {
	case "exempt", "non_taxable":
		return amount, 0, amount, nil
	case "taxable":
	default:
		return 0, 0, 0, errors.New("invalid tax category")
	}
	if rateBPS == 0 {
		return amount, 0, amount, nil
	}
	switch mode {
	case "exclusive":
		taxAmount, err = roundedRatio(amount, int64(rateBPS), 10000)
		if err != nil {
			return 0, 0, 0, err
		}
		if taxAmount > 0 && amount > int64(^uint64(0)>>1)-taxAmount {
			return 0, 0, 0, errors.New("tax total overflow")
		}
		return amount, taxAmount, amount + taxAmount, nil
	case "inclusive":
		taxAmount, err = roundedRatio(amount, int64(rateBPS), int64(10000+rateBPS))
		if err != nil {
			return 0, 0, 0, err
		}
		if taxAmount > amount {
			return 0, 0, 0, errors.New("invalid inclusive tax result")
		}
		return amount - taxAmount, taxAmount, amount, nil
	default:
		return 0, 0, 0, errors.New("calculation_mode must be exclusive or inclusive")
	}
}

// roundedRatio returns round-half-up(amount*numerator/denominator) using integer
// arithmetic so monetary calculations never lose precision through float64.
func roundedRatio(amount, numerator, denominator int64) (int64, error) {
	if amount < 0 || numerator < 0 || denominator <= 0 {
		return 0, errors.New("invalid ratio")
	}
	product := new(big.Int).Mul(big.NewInt(amount), big.NewInt(numerator))
	divisor := big.NewInt(denominator)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(product, divisor, remainder)
	doubledRemainder := new(big.Int).Lsh(remainder, 1)
	if doubledRemainder.Cmp(divisor) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, errors.New("tax amount overflow")
	}
	return quotient.Int64(), nil
}
