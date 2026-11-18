package chat

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
)

func jpegSwapsAxes(src io.Reader) bool {
	r := bufio.NewReader(src)
	var header [2]byte
	if _, err := io.ReadFull(r, header[:]); err != nil || header != [2]byte{0xff, 0xd8} {
		return false
	}
	for {
		prefix, err := r.ReadByte()
		if err != nil || prefix != 0xff {
			return false
		}
		marker, err := r.ReadByte()
		for err == nil && marker == 0xff {
			marker, err = r.ReadByte()
		}
		if err != nil || marker == 0xda || marker == 0xd9 {
			return false
		}
		if _, err := io.ReadFull(r, header[:]); err != nil {
			return false
		}
		size := int(binary.BigEndian.Uint16(header[:])) - 2
		if size < 0 {
			return false
		}
		if marker != 0xe1 {
			if _, err := r.Discard(size); err != nil {
				return false
			}
			continue
		}
		segment := make([]byte, size)
		if _, err := io.ReadFull(r, segment); err != nil {
			return false
		}
		if bytes.HasPrefix(segment, []byte("Exif\x00\x00")) {
			return exifSwapsAxes(segment[6:])
		}
	}
}

func exifSwapsAxes(tiff []byte) bool {
	if len(tiff) < 8 {
		return false
	}
	var order binary.ByteOrder
	switch string(tiff[:4]) {
	case "II\x2a\x00":
		order = binary.LittleEndian
	case "MM\x00\x2a":
		order = binary.BigEndian
	default:
		return false
	}
	offset := order.Uint32(tiff[4:8])
	if offset < 8 || offset > uint32(len(tiff)-2) {
		return false
	}
	entries := tiff[offset:]
	count := int(order.Uint16(entries[:2]))
	entries = entries[2:]
	if count > len(entries)/12 {
		return false
	}
	for i := 0; i < count; i++ {
		entry := entries[i*12 : (i+1)*12]
		if order.Uint16(entry[:2]) == 0x0112 && order.Uint16(entry[2:4]) == 3 && order.Uint32(entry[4:8]) == 1 {
			orientation := order.Uint16(entry[8:10])
			return orientation >= 5 && orientation <= 8
		}
	}
	return false
}
