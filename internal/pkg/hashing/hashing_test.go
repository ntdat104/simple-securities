package hashing

import "testing"

func TestMd5Hash(t *testing.T) {
	var tests = []struct {
		input    string
		expected string
	}{
		{"", "d41d8cd98f00b204e9800998ecf8427e"},
		{"a", "0cc175b9c0f1b6a831c399e269772661"},
		{"abc", "900150983cd24fb0d6963f7d28e17f72"},
		{"message digest", "f96b697d7cb7938d525a2f31aaf161d0"},
		{"abcdefghijklmnopqrstuvwxyz", "c3fcd3d76192e4007dfb496cca67e13b"},
		{"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789", "d174ab98d277d9f5a5611c2c9f419d9f"},
		{"12345678901234567890123456789012345678901234567890123456789012345678901234567890", "57edf4a22be3c955ac49da2e2107b67a"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			resultHex := Md5Hash([]byte(tt.input))
			if resultHex != tt.expected {
				t.Errorf("MD5(%q) = %s; want %s", tt.input, resultHex, tt.expected)
			}
		})
	}
}

func TestSha1Hash(t *testing.T) {
	var tests = []struct {
		input    string
		expected string
	}{
		{"", "da39a3ee5e6b4b0d3255bfef95601890afd80709"},
		{"a", "86f7e437faa5a7fce15d1ddcb9eaeaea377667b8"},
		{"abc", "a9993e364706816aba3e25717850c26c9cd0d89d"},
		{"message digest", "c12252ceda8be8994d5fa0290a47231c1d16aae3"},
		{"abcdefghijklmnopqrstuvwxyz", "32d10c7b8cf96570ca04ce37f2a19d84240d3a89"},
		{"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789", "761c457bf73b14d27e9e9265c46f4b4dda11f940"},
		{"1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890", "fecfd28bbc9345891a66d7c1b8ff46e60192d284"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			resultHex := Sha1Hash([]byte(tt.input))
			if resultHex != tt.expected {
				t.Errorf("SHA-1(%q) = %s; want %s", tt.input, resultHex, tt.expected)
			}
		})
	}
}

func TestSha256Hash(t *testing.T) {
	testCases := []struct {
		in       string
		expected string
	}{
		{"hello world", "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"},
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"a", "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb"},
		{"The quick brown fox jumps over the lazy dog", "d7a8fbb307d7809469ca9abcb0082e4f8d5651e46d3cdb762d02d0bf37c9e592"},
		{"The quick brown fox jumps over the lazy dog.", "ef537f25c895bfa782526529a9b63d97aa631564d5d789c2b765448c8635fb6c"},
		{"Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.", "2d8c2f6d978ca21712b5f6de36c9d31fa8e96a4fa5d8ff8b0188dfb9e7c171bb"},
	}
	for _, tc := range testCases {
		result := Sha256Hash([]byte(tc.in))
		if result != tc.expected {
			t.Fatalf("Hash(%s) = %s, expected %s", tc.in, result, tc.expected)
		}
	}
}

func BenchmarkHash(b *testing.B) {
	testCases := []struct {
		name string
		in   string
	}{
		{"hello world", "hello world"},
		{"empty", ""},
		{"a", "a"},
		{"The quick brown fox jumps over the lazy dog", "The quick brown fox jumps over the lazy dog"},
		{"The quick brown fox jumps over the lazy dog.", "The quick brown fox jumps over the lazy dog."},
		{"Lorem ipsum", "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum."},
	}
	for _, testCase := range testCases {
		b.Run(testCase.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Sha256Hash([]byte(testCase.in))
			}
		})
	}
}

func TestSha512Hash(t *testing.T) {
	testCases := []struct {
		in       string
		expected string
	}{
		{"hello world", "309ecc489c12d6eb4cc40f50c902f2b4d0ed77ee511a7c7a9bcd3ca86d4cd86f989dd35bc5ff499670da34255b45b0cfd830e81f605dcf7dc5542e93ae9cd76f"},
		{"", "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e"},
		{"a", "1f40fc92da241694750979ee6cf582f2d5d7d28e18335de05abc54d0560e0f5302860c652bf08d560252aa5e74210546f369fbbbce8c12cfc7957b2652fe9a75"},
		{"The quick brown fox jumps over the lazy dog", "07e547d9586f6a73f73fbac0435ed76951218fb7d0c8d788a309d785436bbb642e93a252a954f23912547d1e8a3b5ed6e1bfd7097821233fa0538f3db854fee6"},
		{"The quick brown fox jumps over the lazy dog.", "91ea1245f20d46ae9a037a989f54f1f790f0a47607eeb8a14d12890cea77a1bbc6c7ed9cf205e67b7f2b8fd4c7dfd3a7a8617e45f3c463d481c7e586c39ac1ed"},
		{"Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.", "8ba760cac29cb2b2ce66858ead169174057aa1298ccd581514e6db6dee3285280ee6e3a54c9319071dc8165ff061d77783100d449c937ff1fb4cd1bb516a69b9"},
	}
	for _, tc := range testCases {
		result := Sha512Hash([]byte(tc.in))
		if result != tc.expected {
			t.Fatalf("Hash(%s) = %s, expected %s", tc.in, result, tc.expected)
		}
	}
}
