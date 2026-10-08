package game

// Prop is one piece of scenery. Model is the GLB basename under
// assets/Mini-Forest/. Radius 0 means decoration only (no collision).
type Prop struct {
	Model                        string
	X, Z, RotYDeg, Scale, Radius float64
}

// tileCentres returns -5.5, -4.5, ..., 5.5.
func tileCentres() []float64 {
	c := make([]float64, 0, 12)
	for i := 0; i < 12; i++ {
		c = append(c, float64(i)-5.5)
	}
	return c
}

// Ground returns the 144 patch-grass tiles.
func Ground() []Prop {
	var props []Prop
	for _, x := range tileCentres() {
		for _, z := range tileCentres() {
			props = append(props, Prop{Model: "patch-grass", X: x, Z: z, Scale: 1})
		}
	}
	return props
}

// Fence returns the 48 fence pieces around the arena plus 4 stone corner caps.
func Fence() []Prop {
	var props []Prop
	for _, v := range tileCentres() {
		props = append(props,
			Prop{Model: "fence", X: v, Z: -6.1, Scale: 1},
			Prop{Model: "fence", X: v, Z: 6.1, Scale: 1},
			Prop{Model: "fence", X: -6.1, Z: v, RotYDeg: 90, Scale: 1},
			Prop{Model: "fence", X: 6.1, Z: v, RotYDeg: 90, Scale: 1},
		)
	}
	for _, x := range []float64{-6.1, 6.1} {
		for _, z := range []float64{-6.1, 6.1} {
			props = append(props, Prop{Model: "stones", X: x, Z: z, Scale: 0.8})
		}
	}
	return props
}

// Obstacles are the props players collide with.
var Obstacles = []Prop{
	{Model: "tree", X: -4, Z: -4, RotYDeg: 0, Scale: 1.6, Radius: 0.5},
	{Model: "tree-high", X: 4, Z: -3.5, RotYDeg: 30, Scale: 1.6, Radius: 0.5},
	{Model: "tree", X: 3.5, Z: 4, RotYDeg: 60, Scale: 1.6, Radius: 0.5},
	{Model: "tree-high", X: -3.5, Z: 3.5, RotYDeg: 0, Scale: 1.6, Radius: 0.5},
	{Model: "tree", X: 0, Z: -4.6, RotYDeg: 0, Scale: 1.6, Radius: 0.5},
	{Model: "rocks-low", X: -1.5, Z: 1.5, RotYDeg: 0, Scale: 0.9, Radius: 0.45},
	{Model: "stones", X: 2, Z: 0.5, RotYDeg: 45, Scale: 1.0, Radius: 0.4},
	{Model: "rocks-low", X: -4.6, Z: 0, RotYDeg: 90, Scale: 0.9, Radius: 0.45},
	{Model: "stones", X: 4.6, Z: 1.5, RotYDeg: 0, Scale: 1.0, Radius: 0.4},
}

// Decor is decoration without collision.
var Decor = []Prop{
	{Model: "plant", X: 1, Z: -2, RotYDeg: 0, Scale: 1.5},
	{Model: "plant", X: -2, Z: -2.5, RotYDeg: 90, Scale: 1.5},
	{Model: "plant", X: 4.5, Z: -0.5, RotYDeg: 0, Scale: 1.5},
	{Model: "plant", X: -1, Z: 4.5, RotYDeg: 0, Scale: 1.5},
}

// ForestModels returns the unique Mini-Forest model names used by the layout.
func ForestModels() []string {
	return []string{"patch-grass", "fence", "tree", "tree-high", "rocks-low", "stones", "plant"}
}
