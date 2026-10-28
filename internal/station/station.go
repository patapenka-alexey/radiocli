// Package station defines the radio station domain type shared by the
// application layers.
package station

// Station is a single named radio stream.
type Station struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}
