package pcm

import (
	"math"
	"testing"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/pion/rtp"
)

// level of a tone after VoiceBoost, in dBFS (RMS of the last half second)
func boosted(t *testing.T, codec string, freq, dBFS, gainDB float64) float64 {
	t.Helper()
	old := VoiceBoostDB
	VoiceBoostDB = gainDB
	defer func() { VoiceBoostDB = old }()

	enc, dec := PCMtoPCMA, PCMAtoPCM
	if codec == core.CodecPCMU {
		enc, dec = PCMtoPCMU, PCMUtoPCM
	}
	var got []int16
	h := VoiceBoost(&core.Codec{Name: codec, ClockRate: 8000}, func(p *rtp.Packet) {
		for _, b := range p.Payload {
			got = append(got, dec(b))
		}
	})
	amp := math.Pow(10, dBFS/20) * 32767 * math.Sqrt2 // RMS → peak
	for n := 0; n < 8000*2; n += 160 {
		payload := make([]byte, 160)
		for i := range payload {
			v := amp * math.Sin(2*math.Pi*freq*float64(n+i)/8000)
			v = math.Max(-32768, math.Min(32767, v))
			payload[i] = enc(int16(v))
		}
		h(&rtp.Packet{Payload: payload})
	}
	tail := got[len(got)-4000:]
	var sum float64
	for _, s := range tail {
		sum += float64(s) * float64(s)
	}
	return 20 * math.Log10(math.Sqrt(sum/float64(len(tail)))/32767)
}

func TestVoiceBoostOffByDefault(t *testing.T) {
	h := func(*rtp.Packet) {}
	old := VoiceBoostDB
	VoiceBoostDB = 0
	defer func() { VoiceBoostDB = old }()
	if VoiceBoost(&core.Codec{Name: core.CodecPCMA}, h) == nil {
		t.Fatal("nil handler")
	}
}

func TestVoiceBoostAmplifiesSpeech(t *testing.T) {
	for _, codec := range []string{core.CodecPCMA, core.CodecPCMU} {
		got := boosted(t, codec, 1000, -36, 15)
		if math.Abs(got-(-21)) > 1.5 {
			t.Errorf("%s: 1 kHz at -36 dBFS +15 dB = %.1f dBFS, want about -21", codec, got)
		}
	}
}

func TestVoiceBoostKeepsLowSounds(t *testing.T) {
	// A machine's 100 Hz hum is what may be listened to: boosted like the rest.
	if got := boosted(t, core.CodecPCMA, 100, -36, 15); math.Abs(got-(-21)) > 1.5 {
		t.Errorf("100 Hz at -36 dBFS +15 dB = %.1f dBFS, want about -21", got)
	}
}

func TestVoiceBoostLimitsLoudSounds(t *testing.T) {
	// Near full scale + 15 dB must not clip: the limiter holds it under
	// the -1 dBFS ceiling (a full-scale sine is -3 dBFS RMS).
	got := boosted(t, core.CodecPCMA, 1000, -6, 15)
	if got > -3 || got < -8 {
		t.Errorf("loud tone came out at %.1f dBFS RMS, want limited below -1 dBFS peak", got)
	}
}
