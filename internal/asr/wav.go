package asr

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

const (
	waveFormatPCM   = 1
	waveFormatFloat = 3
)

func DecodeWAV(data []byte) (Audio, error) {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return Audio{}, errors.New("invalid WAV header")
	}

	var format, channels, bitsPerSample uint16
	var sampleRate uint32
	var samples []byte
	for offset := 12; offset+8 <= len(data); {
		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8
		if chunkSize < 0 || chunkSize > len(data)-offset {
			return Audio{}, errors.New("invalid WAV chunk size")
		}
		chunk := data[offset : offset+chunkSize]
		switch chunkID {
		case "fmt ":
			if len(chunk) < 16 {
				return Audio{}, errors.New("invalid WAV format chunk")
			}
			format = binary.LittleEndian.Uint16(chunk[0:2])
			channels = binary.LittleEndian.Uint16(chunk[2:4])
			sampleRate = binary.LittleEndian.Uint32(chunk[4:8])
			bitsPerSample = binary.LittleEndian.Uint16(chunk[14:16])
		case "data":
			samples = chunk
		}
		offset += chunkSize
		if chunkSize%2 != 0 {
			offset++
		}
	}
	if channels == 0 || sampleRate == 0 || len(samples) == 0 {
		return Audio{}, errors.New("WAV must contain format and audio samples")
	}
	bytesPerSample := int(bitsPerSample / 8)
	if bytesPerSample == 0 || len(samples)%(int(channels)*bytesPerSample) != 0 {
		return Audio{}, errors.New("invalid WAV sample data")
	}
	if format != waveFormatPCM && format != waveFormatFloat {
		return Audio{}, fmt.Errorf("unsupported WAV format %d", format)
	}
	if format == waveFormatFloat && bitsPerSample != 32 {
		return Audio{}, fmt.Errorf("unsupported WAV floating point bit depth %d", bitsPerSample)
	}
	if format == waveFormatPCM && bitsPerSample != 16 && bitsPerSample != 24 && bitsPerSample != 32 {
		return Audio{}, fmt.Errorf("unsupported WAV PCM bit depth %d", bitsPerSample)
	}

	frames := len(samples) / (int(channels) * bytesPerSample)
	mono := make([]float32, frames)
	for frame := range frames {
		var mixed float64
		for channel := range int(channels) {
			offset := (frame*int(channels) + channel) * bytesPerSample
			mixed += float64(decodeSample(samples[offset:offset+bytesPerSample], format, bitsPerSample))
		}
		mono[frame] = float32(mixed / float64(channels))
	}
	return Audio{Samples: mono, SampleRate: int(sampleRate)}, nil
}

func decodeSample(data []byte, format, bitsPerSample uint16) float32 {
	if format == waveFormatFloat {
		return math.Float32frombits(binary.LittleEndian.Uint32(data))
	}
	switch bitsPerSample {
	case 16:
		return float32(int16(binary.LittleEndian.Uint16(data))) / 32768
	case 24:
		value := int32(data[0]) | int32(data[1])<<8 | int32(data[2])<<16
		if value&0x800000 != 0 {
			value |= ^0xffffff
		}
		return float32(value) / 8388608
	default:
		return float32(int32(binary.LittleEndian.Uint32(data))) / 2147483648
	}
}
