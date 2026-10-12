package media

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"os/exec"
	"testing"
)

func TestImageFrameExtractsFirstGIFFrameWithFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}

	first := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Black, color.White})
	first.SetColorIndex(0, 0, 1)
	second := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Black, color.White})
	second.SetColorIndex(1, 1, 1)
	var encoded bytes.Buffer
	if err := gif.EncodeAll(&encoded, &gif.GIF{
		Image: []*image.Paletted{first, second},
		Delay: []int{10, 10},
	}); err != nil {
		t.Fatalf("encode GIF: %v", err)
	}

	frame, err := ImageFrame(context.Background(), encoded.Bytes(), "image/gif", 0)
	if err != nil {
		t.Fatalf("ImageFrame() error = %v", err)
	}
	if !bytes.HasPrefix(frame, []byte{0xff, 0xd8, 0xff}) {
		t.Fatalf("extracted frame is not JPEG: %x", frame[:min(len(frame), 3)])
	}
}
