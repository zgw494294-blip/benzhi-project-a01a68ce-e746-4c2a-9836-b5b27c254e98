package domain

import (
	"math"
	"sort"
)

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cpy := append([]float64(nil), values...)
	sort.Float64s(cpy)
	mid := len(cpy) / 2
	if len(cpy)%2 == 1 {
		return cpy[mid]
	}
	return (cpy[mid-1] + cpy[mid]) / 2
}

func mad(values []float64, center float64) float64 {
	deviations := make([]float64, len(values))
	for i, v := range values {
		deviations[i] = math.Abs(v - center)
	}
	return median(deviations)
}

func rounded(value float64) float64 { return math.Round(value*100) / 100 }

func activeEvaluations(evals []Evaluation) []Evaluation {
	out := make([]Evaluation, 0, len(evals))
	for _, e := range evals {
		if e.ValidityStatus == ValidityValid {
			out = append(out, e)
		}
	}
	return out
}
