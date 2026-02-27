# Encode and Decode Strings

## Problem Description

Design an algorithm to encode a list of strings to a string. The encoded string is then sent over the network and is decoded back to the original list of strings.

Please implement `encode` and `decode`.

### Example 1:

**Input:** `strs = ["lint","code","love","you"]`  
**Output:** `["lint","code","love","you"]`  
**Explanation:** One possible encoding is `"4#lint4#code4#love3#you"`

### Example 2:

**Input:** `strs = ["we", "say", ":", "yes"]`  
**Output:** `["we", "say", ":", "yes"]`

### Constraints:

- `0 <= strs.length <= 100`
- `0 <= strs[i].length <= 200`
- `strs[i]` contains any possible characters out of 256 valid ASCII characters.

## Solution (Golang)

### Approach: Length-Based Prefixing

The core challenge is to distinguish between the content of the strings and the delimiters used to separate them. By prefixing each string with its length followed by a special character (like `#`), we create a reliable way to know exactly how many characters to read for the next string.

```go
import (
	"strconv"
	"strings"
)

type Codec struct{}

// Encode encodes a list of strings to a single string.
func (codec *Codec) Encode(strs []string) string {
	var res strings.Builder
	for _, s := range strs {
		res.WriteString(strconv.Itoa(len(s)) + "#" + s)
	}
	return res.String()
}

// Decode decodes a single string to a list of strings.
func (codec *Codec) Decode(str string) []string {
	var res []string
	i := 0
	for i < len(str) {
		j := i
		for str[j] != '#' {
			j++
		}
		length, _ := strconv.Atoi(str[i:j])
		i = j + 1
		res = append(res, str[i:i+length])
		i += length
	}
	return res
}
```

## Explanation

1.  **Encode**: For every string in the list, we take its length, convert it to a string, append a delimiter `#`, and then append the actual string. This creates a "header" for each piece of data.
2.  **Decode**: We use a pointer `i` to traverse the encoded string:
    - We find the next `#` to determine where the length descriptor ends.
    - We parse the integer between `i` and the delimiter to get the `length`.
    - We extract the substring of size `length` starting immediately after the `#`.
    - We move the pointer `i` to the start of the next encoded segment.

## Complexity

-   **Time Complexity**: $O(n)$, where $n$ is the total number of characters across all strings. Each character is processed once during encoding and once during decoding.
-   **Space Complexity**: $O(n)$ if we count the output (the encoded string or the decoded slice). The auxiliary space used is $O(1)$.
