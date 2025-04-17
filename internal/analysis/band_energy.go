package analysis

import (
	"audio/internal/transport"
	"log"
	"math"
)

// FrequencyBand defines the name and frequency range for an energy band.
type FrequencyBand struct {
	Name    string
	LowHz   float64
	HighHz  float64
	Energy  float64 // Holds the calculated energy for the current frame
	numBins int     // Internal counter for normalization
}

// BandEnergyProcessor calculates energy across predefined frequency bands from FFT data.
type BandEnergyProcessor struct {
	transport   transport.Transport
	bands       []*FrequencyBand
	fftProvider transport.FFTResultProvider // Use the interface from audio package
}

// NewBandEnergyProcessor creates a new processor for calculating band energy.
func NewBandEnergyProcessor(transport transport.Transport, fftProvider transport.FFTResultProvider) *BandEnergyProcessor {
	if fftProvider == nil {
		log.Panic("BandEnergyProcessor requires a non-nil FFTResultProvider")
	}
	// Define bands (potentially get sampleRate/fftSize from provider if added to interface)
	// For now, assume GetFrequencyForBin is sufficient
	sampleRateEstimate := 44100.0 // Placeholder - ideally get from provider
	bands := []*FrequencyBand{
		{Name: "sub", LowHz: 20, HighHz: 60},
		{Name: "bass", LowHz: 60, HighHz: 250},
		{Name: "lowMid", LowHz: 250, HighHz: 500},
		{Name: "mid", LowHz: 500, HighHz: 2000},
		{Name: "highMid", LowHz: 2000, HighHz: 4000},
		{Name: "treble", LowHz: 4000, HighHz: sampleRateEstimate / 2},
	}
	log.Printf("Analysis: Initializing BandEnergyProcessor with %d bands.", len(bands))
	p := &BandEnergyProcessor{
		transport:   transport,
		bands:       bands,
		fftProvider: fftProvider,
	}
	return p
}

// Process calculates band energies using data from the FFTResultProvider.
// This method might be called *after* the FFTProcessor's Process method
// within the engine's loop, or triggered differently.
// It doesn't take the raw audio buffer directly.
func (p *BandEnergyProcessor) Process() { // Changed signature
	if p.transport == nil || p.fftProvider == nil {
		return
	}

	// 1. Get the latest magnitudes from the provider
	magnitudes := p.fftProvider.GetMagnitudes()
	if magnitudes == nil {
		return // No data available yet
	}

	// 2. Reset energy and bin counts for each band
	for _, band := range p.bands {
		band.Energy = 0
		band.numBins = 0
	}

	// 3. Iterate through FFT magnitudes
	numMagnitudeBins := len(magnitudes)
	for i := 0; i < numMagnitudeBins; i++ {
		// Get the frequency for this bin from the provider
		freq := p.fftProvider.GetFrequencyForBin(i)

		// Find which band this frequency belongs to
		for _, band := range p.bands {
			if freq >= band.LowHz && freq < band.HighHz {
				band.Energy += magnitudes[i] * magnitudes[i] // Sum energy (magnitude squared)
				band.numBins++
				break
			}
		}
	}

	// 4. Prepare data map (normalize energy)
	bandData := map[string]interface{}{"type": "band_energy"}
	for _, band := range p.bands {
		avgBandEnergy := 0.0
		if band.numBins > 0 {
			avgBandEnergy = band.Energy / float64(band.numBins)
		}
		scaledValue := math.Sqrt(avgBandEnergy) * 50.0   // Example scaling
		bandData[band.Name] = math.Min(1.0, scaledValue) // Clamp
	}

	// log.Printf("DEBUG: BandEnergyProcessor calculated: %+v", bandData)

	// 5. Send data
	if err := p.transport.Send(bandData); err != nil {
		log.Printf("BandEnergyProcessor: Error sending band energy data: %v", err)
	}
}

// Remove getFrequencyForBin - use the one from fftProvider
// func (p *BandEnergyProcessor) getFrequencyForBin(binIndex int) float64 { ... }
