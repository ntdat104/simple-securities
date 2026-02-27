# Reverse String

## Problem Description

Write a function that reverses a string. The input string is given as an array of characters `s`.

You must do this by modifying the input array **[in-place](https://en.wikipedia.org/wiki/In-place_algorithm)** with $O(1)$ extra memory.

### Example 1:

**Input:** `s = ["h","e","l","l","o"]`  
**Output:** `["o","l","l","e","h"]`

### Example 2:

**Input:** `s = ["H","a","n","n","a","h"]`  
**Output:** `["h","a","n","n","a","H"]`

### Constraints:

- `1 <= s.length <= 10^5`
- `s[i]` is a printable ascii character.

## Solution (Golang)

### Approach: Two Pointers (Standard)

```go
func reverseString(s []byte) {
    left, right := 0, len(s)-1
    for left < right {
        s[left], s[right] = s[right], s[left]
        left++
        right--
    }
}
```

## Explanation

1. **Two Pointers**: We initialize one pointer `left` at the beginning of the array (index 0) and one pointer `right` at the end of the array (last index).
2. **Swap**: While `left` is less than `right`, we swap the elements at these two positions.
3. **Move**: We increment `left` and decrement `right` to move towards the center of the array.
4. **Termination**: When `left` is no longer less than `right` (they meet or cross), the entire array has been reversed.

## Complexity

- **Time Complexity**: $O(n)$, where $n$ is the length of the string. We perform $n/2$ swaps.
- **Space Complexity**: $O(1)$, as we perform the reversal in-place and use only two extra variables.
