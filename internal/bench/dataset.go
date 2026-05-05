package bench

import (
	"fmt"
	"math/rand"
)

var tierSize = map[string]int{
	"small":  1000,
	"medium": 10000,
}

func BuildDataset(tier string, seed int64) ([]Record, error) {
	size, ok := tierSize[tier]
	if !ok {
		return nil, fmt.Errorf("unsupported tier %q", tier)
	}
	rng := rand.New(rand.NewSource(seed))
	records := make([]Record, 0, size)
	for i := 0; i < size; i++ {
		domain := "alpha"
		if i%3 == 1 {
			domain = "beta"
		} else if i%3 == 2 {
			domain = "gamma"
		}
		key := fmt.Sprintf("%s:%06d", domain, i)
		value := fmt.Sprintf("v%d-%08x", i, rng.Uint32())
		records = append(records, Record{Key: key, Value: value})
	}
	return records, nil
}
