# 242. Valid Anagram

## Problem Description

Given two strings `s` and `t`, return `true` if `t` is an anagram of `s`, and `false` otherwise.

An **Anagram** is a word or phrase formed by rearranging the letters of a different word or phrase, typically using all the original letters exactly once.

### Example 1:

**Input:** `s = "anagram"`, `t = "nagaram"`  
**Output:** `true`

### Example 2:

**Input:** `s = "rat"`, `t = "car"`  
**Output:** `false`

### Constraints:

- `1 <= s.length, t.length <= 5 * 10^4`
- `s` and `t` consist of lowercase English letters.

## Solution (Golang)

### Approach: Frequency Counter (Array)

Since the input only contains lowercase English letters, we can use an array of size 26 to count the frequency of each character.

```go
func isAnagram(s string, t string) bool {
    if len(s) != len(t) {
        return false
    }

    var counts [26]int
    
    for i := 0; i < len(s); i++ {
        counts[s[i]-'a']++
        counts[t[i]-'a']--
    }

    for _, count := range counts {
        if count != 0 {
            return false
        }
    }

    return true
}
```

## Explanation

1. **Length Check**: If the lengths of `s` and `t` are different, they cannot be anagrams, so we return `false` immediately.
2. **Frequency Array**: We initialize an array `counts` of size 26 (for 'a' through 'z').
3. **Single Pass**: We iterate through both strings simultaneously. We increment the count for the character in `s` and decrement the count for the character in `t`.
4. **Final Check**: After the loop, every element in `counts` must be 0 if the strings are anagrams. If any element is non-zero, it means there was a mismatch in character frequency.

## Complexity

- **Time Complexity**: $O(n)$, where $n$ is the length of the strings. We traverse the strings once.
- **Space Complexity**: $O(1)$ (or $O(k)$ where $k=26$). Since the alphabet size is fixed, the space used does not grow with the input size.
