package asr

import (
	"encoding/binary"
	"testing"
)

func TestDecodeWAVDownmixesPCM16(t *testing.T) {
	t.Parallel()

	data := makeWAV(2, 16, []byte{0x00, 0x40, 0x00, 0xc0})
	audio, err := DecodeWAV(data)
	if err != nil {
		t.Fatalf("DecodeWAV() error = %v", err)
	}
	if audio.SampleRate != 16000 || len(audio.Samples) != 1 {
		t.Fatalf("DecodeWAV() = %#v", audio)
	}
	if audio.Samples[0] != 0 {
		t.Fatalf("downmixed sample = %v, want 0", audio.Samples[0])
	}
}

func TestDecodeWAVRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	if _, err := DecodeWAV([]byte("not a wave")); err == nil {
		t.Fatal("DecodeWAV() error = nil")
	}
}

func makeWAV(channels, bitsPerSample uint16, samples []byte) []byte {
	data := make([]byte, 44+len(samples))
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	binary.LittleEndian.PutUint32(data[16:20], 16)
	binary.LittleEndian.PutUint16(data[20:22], waveFormatPCM)
	binary.LittleEndian.PutUint16(data[22:24], channels)
	binary.LittleEndian.PutUint32(data[24:28], 16000)
	binary.LittleEndian.PutUint32(data[28:32], 16000*uint32(channels)*uint32(bitsPerSample/8))
	binary.LittleEndian.PutUint16(data[32:34], channels*(bitsPerSample/8))
	binary.LittleEndian.PutUint16(data[34:36], bitsPerSample)
	copy(data[36:40], "data")
	binary.LittleEndian.PutUint32(data[40:44], uint32(len(samples)))
	copy(data[44:], samples)
	return data
}
