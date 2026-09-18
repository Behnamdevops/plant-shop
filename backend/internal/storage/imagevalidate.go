package storage

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg" // register JPEG decoder for image.DecodeConfig sniffing
	_ "image/png"  // register PNG decoder for image.DecodeConfig sniffing
	"io"

	_ "golang.org/x/image/webp" // register WebP decoder (decode-only) for sniffing
)

// MaxImageBytes is the maximum accepted upload size for a single product
// image (V1 conservative limit).
const MaxImageBytes = 5 * 1024 * 1024 // 5 MB

// ErrEmptyFile, ErrTooLarge, and ErrUnsupportedFormat are returned by
// ValidateImage to let callers map validation failures to specific HTTP
// error responses.
var (
	ErrEmptyFile         = errors.New("storage: file is empty")
	ErrTooLarge          = errors.New("storage: file exceeds maximum allowed size")
	ErrUnsupportedFormat = errors.New("storage: unsupported or invalid image format")
)

// formatExtensions maps the format name reported by image.DecodeConfig
// (via the registered decoders above) to the file extension we store the
// object under. Only formats in this map are accepted — this is an
// allow-list, so decoders we don't register (e.g. gif, bmp) are never
// reachable even if some other package's init() registers them.
var formatExtensions = map[string]string{
	"jpeg": ".jpg",
	"png":  ".png",
	"webp": ".webp",
}

// ValidatedImage holds the sniffed result of a successful ValidateImage
// call: the normalized extension to store the file under, and a reader
// positioned at the start of the full file content (so callers can still
// persist the original bytes without re-uploading).
type ValidatedImage struct {
	Ext  string
	Data []byte
}

// ValidateImage proves that data is a genuine, decodable image of a
// supported format, within the size limit. It never trusts a
// client-supplied filename or Content-Type header — the format is
// determined purely by decoding the content (via image.DecodeConfig,
// which reads only the header/metadata needed to identify the format,
// not the full pixel data).
//
// data must already be fully read into memory by the caller with a
// hard cap of MaxImageBytes+1 bytes, so an attacker cannot exhaust memory
// with an unbounded request body before validation even runs.
func ValidateImage(data []byte) (ValidatedImage, error) {
	if len(data) == 0 {
		return ValidatedImage{}, ErrEmptyFile
	}
	if len(data) > MaxImageBytes {
		return ValidatedImage{}, ErrTooLarge
	}

	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ValidatedImage{}, ErrUnsupportedFormat
	}

	ext, ok := formatExtensions[format]
	if !ok {
		return ValidatedImage{}, ErrUnsupportedFormat
	}

	return ValidatedImage{Ext: ext, Data: data}, nil
}

// ReadLimited reads from r up to limit+1 bytes and reports ErrTooLarge if
// more than limit bytes were available, without ever buffering unbounded
// attacker-controlled input. Use this to cap a multipart file part before
// calling ValidateImage.
func ReadLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, ErrTooLarge
	}
	return data, nil
}
