package hashing

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
)

func Md5Hash(message []byte) string {
	hash := md5.Sum([]byte(message))
	return hex.EncodeToString(hash[:])
}

func Sha1Hash(message []byte) string {
	hash := sha1.Sum([]byte(message))
	return hex.EncodeToString(hash[:])
}

func Sha256Hash(message []byte) string {
	hash := sha256.Sum256([]byte(message))
	return hex.EncodeToString(hash[:])
}

func Sha512Hash(message []byte) string {
	hash := sha512.Sum512([]byte(message))
	return hex.EncodeToString(hash[:])
}
