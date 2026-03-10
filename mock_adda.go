package main

import (
	"math"
)

// MockADDA generates sine-wave test data for all channels.
type MockADDA struct {
	t     float64
	NumCh int
}

// MockSample holds one channel's raw and physical values.
type MockSample struct {
	Raw int16
	Phy float64
}

// NewMockADDA creates a new mock AD/DA generator.
func NewMockADDA(numCh int) *MockADDA {
	return &MockADDA{NumCh: numCh}
}

// Next advances time by dt and returns samples for all channels.
// Each channel has a different phase offset so the waves are staggered.
func (m *MockADDA) Next(dt float64) []MockSample {
	m.t += dt
	samples := make([]MockSample, m.NumCh)
	for ch := 0; ch < m.NumCh; ch++ {
		phase := float64(ch) * math.Pi / float64(m.NumCh/2)
		sinVal := math.Sin(m.t + phase)
		samples[ch] = MockSample{
			Raw: int16(sinVal * float64(int16Max)),
			Phy: sinVal,
		}
	}
	return samples
}
