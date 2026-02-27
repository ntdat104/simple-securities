# 125. Valid Palindrome

## Problem Description

A phrase is a **palindrome** if, after converting all uppercase letters into lowercase letters and removing all non-alphanumeric characters, it reads the same forward and backward. Alphanumeric characters include letters and numbers.

Given a string `s`, return `true` if it is a **palindrome**, or `false` otherwise.

### Example 1:

**Input:** `s = "A man, a plan, a canal: Panama"`  
**Output:** `true`  
**Explanation:** "amanaplanacanalpanama" is a palindrome.

### Example 2:

**Input:** `s = "race a car"`  
**Output:** `false`  
**Explanation:** "raceacar" is not a palindrome.

### Example 3:

**Input:** `s = " "`  
**Output:** `true`  
**Explanation:** `s` is an empty string `""` after removing non-alphanumeric characters. Since an empty string reads the same forward and backward, it is a palindrome.

### Constraints:

- `1 <= s.length <= 2 * 10^5`
- `s` consists only of printable ASCII characters.

## Solution (Golang)

### Approach: Two Pointers

```go
func isPalindrome(s string) bool {
    left, right := 0, len(s)-1
    
    for left < right {
        // Skip non-alphanumeric from left
        for left < right && !isAlphanumeric(s[left]) {
            left++
        }
        // Skip non-alphanumeric from right
        for left < right && !isAlphanumeric(s[right]) {
            right--
        }
        
        // Compare (case-insensitive)
        if toLower(s[left]) != toLower(s[right]) {
            return false
        }
        
        left++
        right--
    }
    
    return true
}

func isAlphanumeric(b byte) bool {
    return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func toLower(b byte) byte {
    if b >= 'A' && b <= 'Z' {
        return b + ('a' - 'A')
    }
    return b
}
```

## Explanation

1.  **Two Pointers**: We initialize `left` at the start of the string and `right` at the end.
2.  **Skipping Characters**: We move the `left` pointer forward and the `right` pointer backward until both point to an alphanumeric character.
3.  **Comparison**: We compare the characters pointed to by `left` and `right` after converting them to lowercase. If they don't match, the string is not a palindrome.
4.  **Repeat**: If they match, we move both pointers closer to the center and repeat until they meet or cross.
5.  **Helper Functions**:
    - `isAlphanumeric`: Checks if a character is a letter or a number.
    - `toLower`: Converts uppercase ASCII letters to lowercase. This is faster than using `strings.ToLower` on the whole string as it doesn't allocate new memory.

## Complexity

-   **Time Complexity**: $O(n)$, where $n$ is the length of the string. Each character is visited at most twice.
-   **Space Complexity**: $O(1)$, as we process the string in-place using pointers without creating a filtered version of the string.
