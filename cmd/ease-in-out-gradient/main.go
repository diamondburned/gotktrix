package main

import (
	"flag"
	"fmt"
	"math"
)

var (
	steps = 5
	f     = "%.2f\n"
	min   = 0.0
	max   = 1.0
)

func main() {
	flag.StringVar(&f, "f", f, "printf string")
	flag.IntVar(&steps, "steps", steps, "number of steps to iterate")
	flag.Float64Var(&min, "min", min, "minimum")
	flag.Float64Var(&max, "max", max, "maximum")
	flag.Parse()

	for i := 0; i < steps; i++ {
		n := easeInOutCubic(float64(i) / float64(steps))
		fmt.Printf(f, (max-min)*n+min)
	}
}

// https://easings.net/#easeInOutCubic
func easeInOutCubic(x float64) float64 {
	if x < 0.5 {
		return 4 * x * x * x
	}
	return 1 - math.Pow((-2*x)+2, 3)/2
}
