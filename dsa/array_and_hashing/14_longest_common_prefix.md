# 14. Longest Common Prefix

## Problem Description

Write a function to find the longest common prefix string amongst an array of strings.

If there is no common prefix, return an empty string `""`.

### Example 1:

**Input:** `strs = ["flower","flow","flight"]`  
**Output:** `"fl"`

### Example 2:

**Input:** `strs = ["dog","racecar","car"]`  
**Output:** `""`  
**Explanation:** There is no common prefix among the input strings.

### Constraints:

- `1 <= strs.length <= 200`
- `0 <= strs[i].length <= 200`
- `strs[i]` consists of only lowercase English letters.

## Solution (Golang)

### Approach: Horizontal Scanning

```go
import "strings"

func longestCommonPrefix(strs []string) string {
    if len(strs) == 0 {
        return ""
    }
    
    prefix := strs[0]
    for i := 1; i < len(strs); i++ {
        for !strings.HasPrefix(strs[i], prefix) {
            prefix = prefix[:len(prefix)-1]
            if prefix == "" {
                return ""
            }
        }
    }
    return prefix
}
```

## Explanation

1. **Edge Case**: If the input slice is empty, we return an empty string.
2. **Initial Prefix**: We assume the first string `strs[0]` is the common prefix.
3. **Comparison**: We loop through the rest of the strings. For each string:
   - While the current string does not start with the `prefix` (using `strings.HasPrefix`), we shorten the `prefix` by removing its last character.
   - If the `prefix` becomes empty, it means there is no common prefix at all among the strings checked so far.
4. **Result**: After checking all strings, the narrowed-down `prefix` is the longest common prefix.

## Complexity

- **Time Complexity**: $O(S)$, where $S$ is the sum of all characters in all strings. In the worst case, we compare every character of every string.
- **Space Complexity**: $O(1)$. We only use a constant amount of extra space for the prefix variable (the prefix itself is a slice of the existing string).
