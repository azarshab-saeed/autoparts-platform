package pricing

import "math"

type Break struct {
	MinQty    float64 `json:"min_qty"`
	UnitPrice int64   `json:"unit_price"`
}

func SelectBreak(breaks []Break, qty float64) (Break, bool) {
	var best Break
	found := false
	for _, b := range breaks {
		if b.MinQty <= 0 || b.UnitPrice < 0 || qty+1e-9 < b.MinQty {
			continue
		}
		if !found || b.MinQty > best.MinQty {
			best, found = b, true
		}
	}
	return best, found
}

// MinimumPriceForMargin returns the lowest sale price that preserves gross
// margin = (price-cost)/price at the configured basis points.
func MinimumPriceForMargin(cost int64, marginBPS int) int64 {
	if cost <= 0 || marginBPS <= 0 {
		return max64(cost, 0)
	}
	if marginBPS >= 10000 {
		return math.MaxInt64
	}
	den := int64(10000 - marginBPS)
	if cost > math.MaxInt64/10000 {
		return math.MaxInt64
	}
	num := cost * 10000
	if num > math.MaxInt64-(den-1) {
		return math.MaxInt64
	}
	return (num + den - 1) / den
}

func MarginBPS(price, cost int64) int {
	if price <= 0 {
		if cost <= 0 {
			return 0
		}
		return -10000
	}
	return int(math.Round(float64(price-cost) * 10000 / float64(price)))
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
