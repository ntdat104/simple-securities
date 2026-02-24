package conversion

import "encoding/base64"

// Base64Encode encodes a byte slice to a base64-encoded string.
func Base64Encode(input []byte) string {
	return base64.StdEncoding.EncodeToString(input)
}

// Base64StrEncode encodes a plain string to a base64-encoded string.
func Base64StrEncode(str string) string {
	return Base64Encode([]byte(str))
}

// Base64Decode decodes a base64-encoded string into a byte slice.
// Returns an error if the input is not a valid base64 string.
func Base64Decode(encoded string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// DecodeToString decodes a base64-encoded string into a plain string.
// Returns an error if the input is not a valid base64 string.
func Base64StrDecode(encoded string) (string, error) {
	data, err := Base64Decode(encoded)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
