package imageinfo

import (
	"encoding/binary"
	"testing"
)

func TestParsePNG(t *testing.T) {
	data := []byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', // signature
		0, 0, 0, 13, 'I', 'H', 'D', 'R', // IHDR chunk header
	}
	width := make([]byte, 4)
	height := make([]byte, 4)
	binary.BigEndian.PutUint32(width, 1920)
	binary.BigEndian.PutUint32(height, 1080)
	data = append(data, width...)
	data = append(data, height...)
	data = append(data, 8, 6, 0, 0, 0) // bit depth, color type, compression, filter, interlace
	data = append(data, 0, 0, 0, 0)   // CRC (unparsed)

	w, h, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse(png): %v", err)
	}
	if w != 1920 || h != 1080 {
		t.Fatalf("Parse(png) = (%d,%d), want (1920,1080)", w, h)
	}
}

func TestParseJPEG(t *testing.T) {
	// FFD8 FFE0 APP0 segment, then FFC0 SOF0 with 16-bit height/width.
	data := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	app0Len := make([]byte, 2)
	binary.BigEndian.PutUint16(app0Len, 16)
	data = append(data, app0Len...)
	data = append(data, 'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1, 0, 1, 0, 0) // 16 bytes total
	data = append(data, 0xFF, 0xC0)
	sofLen := make([]byte, 2)
	binary.BigEndian.PutUint16(sofLen, 11)
	data = append(data, sofLen...)
	data = append(data, 8) // precision
	height := make([]byte, 2)
	width := make([]byte, 2)
	binary.BigEndian.PutUint16(height, 720)
	binary.BigEndian.PutUint16(width, 1280)
	data = append(data, height...)
	data = append(data, width...)
	data = append(data, 3, 1, 0x11, 0, 1, 0x11, 0, 1, 0x11, 0) // components (len 11)

	w, h, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse(jpeg): %v", err)
	}
	if w != 1280 || h != 720 {
		t.Fatalf("Parse(jpeg) = (%d,%d), want (1280,720)", w, h)
	}
}

func TestParseWebP(t *testing.T) {
	// VP8 (lossy) frame: chunk tag, 3-byte frame tag, start code, then
	// 14-bit little-endian width/height.
	data := []byte("RIFF\x24\x00\x00\x00WEBPVP8 ")
	data = append(data, 0x01, 0x02, 0x03) // frame tag
	data = append(data, 0x9D, 0x01, 0x2A) // start code
	dim := make([]byte, 4)
	binary.LittleEndian.PutUint16(dim[0:2], 640)
	binary.LittleEndian.PutUint16(dim[2:4], 360)
	data = append(data, dim...)

	w, h, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse(webp): %v", err)
	}
	if w != 640 || h != 360 {
		t.Fatalf("Parse(webp) = (%d,%d), want (640,360)", w, h)
	}
}

func TestParseRejectsUnknownAndTruncated(t *testing.T) {
	if _, _, err := Parse([]byte("not an image at all")); err != ErrUnsupportedFormat {
		t.Fatalf("unknown format err = %v, want ErrUnsupportedFormat", err)
	}
	if _, _, err := Parse([]byte{0x89, 'P', 'N', 'G'}); err == nil {
		t.Fatal("truncated PNG must fail")
	}
	if _, _, err := Parse(nil); err == nil {
		t.Fatal("empty input must fail")
	}
}
