package pcm

import (
	"math"
	"os"
	"strconv"
	"sync"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/pion/rtp"
)

// VoiceBoostDB is the gain applied by VoiceBoost, from the environment
// variable GO2RTC_VOICE_BOOST (in dB, e.g. "15"). Zero (the default) turns
// the feature off, so go2rtc behaves exactly as upstream.
var VoiceBoostDB = func() float64 {
	v, _ := strconv.ParseFloat(os.Getenv("GO2RTC_VOICE_BOOST"), 64)
	return math.Max(0, math.Min(v, 30))
}()

// VoiceBoost makes G.711 camera audio (PCMA/PCMU, 8 kHz) much louder, for
// listening from across a room (e.g. a machine and its alarm):
//
//  1. the signal is amplified by VoiceBoostDB;
//  2. a peak limiter (instant attack, ~150 ms release) keeps it just below
//     full scale, so loud sounds are turned down smoothly instead of
//     wrapping or breaking up.
//
// Nothing is filtered out: low hum can be exactly what is being listened
// to.
//
// Output uses the same codec as the input. Other codecs pass through.
func VoiceBoost(codec *core.Codec, handler core.HandlerFunc) core.HandlerFunc {
	if VoiceBoostDB <= 0 {
		return handler
	}

	var decode func(byte) int16
	var encode func(int16) byte
	switch codec.Name {
	case core.CodecPCMA:
		decode, encode = PCMAtoPCM, PCMtoPCMA
	case core.CodecPCMU:
		decode, encode = PCMUtoPCM, PCMtoPCMU
	default:
		return handler
	}

	rate := float64(codec.ClockRate)
	if rate == 0 {
		rate = 8000
	}
	f := newVoiceFilter(rate, VoiceBoostDB)

	var mu sync.Mutex
	return func(packet *rtp.Packet) {
		mu.Lock()
		out := make([]byte, len(packet.Payload))
		for i, b := range packet.Payload {
			out[i] = encode(f.process(decode(b)))
		}
		mu.Unlock()

		clone := *packet
		clone.Payload = out
		handler(&clone)
	}
}

type voiceFilter struct {
	gain    float64
	ceiling float64
	release float64
	env     float64
}

func newVoiceFilter(rate, gainDB float64) *voiceFilter {
	return &voiceFilter{
		gain:    math.Pow(10, gainDB/20),
		ceiling: math.Pow(10, -0.5/20) * 32767, // -0.5 dBFS
		release: math.Exp(-1 / (0.15 * rate)),  // ~150 ms
	}
}

func (f *voiceFilter) process(s int16) int16 {
	y := float64(s) * f.gain

	// peak envelope: rises at once, falls slowly
	if a := math.Abs(y); a > f.env {
		f.env = a
	} else {
		f.env = f.env*f.release + a*(1-f.release)
	}
	if f.env > f.ceiling {
		y *= f.ceiling / f.env
	}

	// final safety clamp (the limiter keeps us below this)
	if y > 32767 {
		y = 32767
	} else if y < -32768 {
		y = -32768
	}
	return int16(y)
}
