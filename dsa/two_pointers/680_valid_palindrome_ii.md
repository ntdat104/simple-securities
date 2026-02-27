# 680. Valid Palindrome II

## Problem Description

Given a string `s`, return `true` if the `s` can be palindrome after deleting **at most one** character from it.

### Example 1:

**Input:** `s = "aba"`  
**Output:** `true`

### Example 2:

**Input:** `s = "abca"`  
**Output:** `true`  
**Explanation:** You could delete the character 'c'.

### Example 3:

**Input:** `s = "abc"`  
**Output:** `false`

### Constraints:

- `1 <= s.length <= 10^5`
- `s` consists of lowercase English letters.

## Solution (Golang)

### Approach: Two Pointers

We use two pointers, `left` and `right`, starting at the ends of the string. If the characters match, we move both pointers toward the center. If they don't match, we check two possibilities:
1. Is the remaining string a palindrome if we skip `s[left]`?
2. Is the remaining string a palindrome if we skip `s[right]`?

```go
func validPalindrome(s string) bool {
    left, right := 0, len(s)-1
    
    for left < right {
        if s[left] != s[right] {
            // Try skipping either the left character or the right character
            return isPalindrome(s, left+1, right) || isPalindrome(s, left, right-1)
        }
        left++
        right--
    }
    
    return true
}

func isPalindrome(s string, l, r int) bool {
    for l < r {
        if s[l] != s[r] {
            return false
        }
        l++
        r--
    }
    return true
}
```

## Explanation

1.  **Iterative Check**: We use the standard two-pointer trick to check for a palindrome from both ends.
2.  **Handling Mismatch**: At the first mismatch (`s[left] != s[right]`), we have used our "one deletion" allowance. We check if the rest of the substring (either from `left+1` to `right` OR from `left` to `right-1`) is a valid palindrome.
3.  **Helper Function**: `isPalindrome` simply checks if a substring is a palindrome without any further deletions allowed.

## Complexity

-   **Time Complexity**: $O(n)$, where $n$ is the length of the string. We traverse the string at most twice.
-   **Space Complexity**: $O(1)$, as we only use pointers and no extra data structures.
