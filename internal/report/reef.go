package report

import (
	"math"
	"strings"
)

func value[T any](v *T) any {
	if v == nil {
		return nil
	}
	return *v
}
func observed(status string) bool { return status != "not_recorded" }
func num(v float64) *float64      { return &v }
func recordedCount(n int, status string) any {
	if !observed(status) {
		return nil
	}
	return n
}

func stats(v []float64) (total, mean, sd, se any) {
	if len(v) == 0 {
		return
	}
	sum := 0.0
	for _, x := range v {
		sum += x
	}
	m := sum / float64(len(v))
	total, mean = sum, m
	if len(v) > 1 {
		ss := 0.0
		for _, x := range v {
			ss += (x - m) * (x - m)
		}
		s := math.Sqrt(ss / float64(len(v)-1))
		sd, se = s, s/math.Sqrt(float64(len(v)))
	}
	return
}
func substrate(code string) (base, morph, under string) {
	base = code
	if strings.HasPrefix(code, "SI(") && strings.HasSuffix(code, ")") {
		base = "SI"
		under = strings.TrimSuffix(strings.TrimPrefix(code, "SI("), ")")
		if strings.HasPrefix(under, "HC-") {
			morph = strings.TrimPrefix(under, "HC-")
			under = "HC"
		}
	} else if strings.HasPrefix(code, "HC-") {
		base = "HC"
		morph = strings.TrimPrefix(code, "HC-")
	}
	return
}
