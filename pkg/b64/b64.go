package b64

import "encoding/base64"

// EncodeToString encodes a byte slice to a base64-encoded string.
func EncodeToString(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// EncodeString encodes a plain string to a base64-encoded string.
func EncodeString(str string) string {
	return EncodeToString([]byte(str))
}

// DecodeString decodes a base64-encoded string into a byte slice.
// Returns an error if the input is not a valid base64 string.
func DecodeString(encoded string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// DecodeToString decodes a base64-encoded string into a plain string.
// Returns an error if the input is not a valid base64 string.
func DecodeToString(encoded string) (string, error) {
	data, err := DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
