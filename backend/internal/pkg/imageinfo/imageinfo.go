// Package imageinfo derives pixel dimensions from common image container
// headers (PNG, JPEG, WebP) without decoding the full image. It is used to
// make cover/poster dimensions server-side trusted: clients cannot assert
// dimensions for objects they upload.
package imageinfo

import (
	"encoding/binary"
	"errors"
)

var (
	ErrUnsupportedFormat = errors.New("unsupported image format")
	ErrMalformedImage    = errors.New("malformed image data")
)

// Parse returns the pixel dimensions of the image in data. It only reads the
// container header, so callers may supply a prefix of the object (e.g. the
// first 64 KiB from a ranged OSS GET).
func Parse(data []byte) (width, height int, err error) {
	if len(data) < 12 {
		return 0, 0, ErrMalformedImage
	}
	switch {
	case len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n":
		return parsePNG(data)
	case data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return parseJPEG(data)
	case string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return parseWebP(data)
	}
	return 0, 0, ErrUnsupportedFormat
}

// parsePNG reads the width/height from the IHDR chunk that immediately
// follows the 8-byte signature.
func parsePNG(data []byte) (int, int, error) {
	if len(data) < 24 {
		return 0, 0, ErrMalformedImage
	}
	if string(data[12:16]) != "IHDR" {
		return 0, 0, ErrMalformedImage
	}
	return int(binary.BigEndian.Uint32(data[16:20])), int(binary.BigEndian.Uint32(data[20:24])), nil
}

// parseJPEG scans the marker segments for the first SOF marker, which carries
// the image height and width directly after the segment length.
func parseJPEG(data []byte) (int, int, error) {
	i := 2
	for i+9 <= len(data) {
		if data[i] != 0xFF {
			i++
			continue
		}
		marker := data[i+1]
		if marker == 0xFF {
			i++
			continue
		}
		// SOF0..SOF15 excluding DHT(C4), JPG(C8), DAC(CC).
		if marker >= 0xC0 && marker <= 0xCF && marker != 0xC4 && marker != 0xC8 && marker != 0xCC {
			return int(binary.BigEndian.Uint16(data[i+7 : i+9])), int(binary.BigEndian.Uint16(data[i+5 : i+7])), nil
		}
		// Standalone markers have no length field.
		if marker == 0xD8 || marker == 0xD9 || (marker >= 0xD0 && marker <= 0xD7) {
			i += 2
			continue
		}
		if i+4 > len(data) {
			return 0, 0, ErrMalformedImage
		}
		segLen := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		i += 2 + segLen
	}
	return 0, 0, ErrMalformedImage
}

// parseWebP supports VP8 (lossy), VP8L (lossless) and VP8X (extended) chunks.
func parseWebP(data []byte) (int, int, error) {
	if len(data) < 26 {
		return 0, 0, ErrMalformedImage
	}
	switch string(data[12:16]) {
	case "VP8 ":
		// Lossy frame: chunk data starts with a 3-byte frame tag, a 3-byte
		// start code, then 14-bit little-endian width and height.
		if len(data) < 26 {
			return 0, 0, ErrMalformedImage
		}
		return int(binary.LittleEndian.Uint16(data[22:24]) & 0x3FFF), int(binary.LittleEndian.Uint16(data[24:26]) & 0x3FFF), nil
	case "VP8L":
		if data[20] != 0x2F {
			return 0, 0, ErrMalformedImage
		}
		bits := binary.LittleEndian.Uint32(data[21:25])
		return int(bits&0x3FFF) + 1, int((bits>>14)&0x3FFF) + 1, nil
	case "VP8X":
		if len(data) < 30 {
			return 0, 0, ErrMalformedImage
		}
		w := int(data[24]) | int(data[25])<<8 | int(data[26])<<16
		h := int(data[27]) | int(data[28])<<8 | int(data[29])<<16
		return w + 1, h + 1, nil
	}
	return 0, 0, ErrUnsupportedFormat
}
