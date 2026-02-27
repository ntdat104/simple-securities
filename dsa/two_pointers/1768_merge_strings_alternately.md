# 1768. Merge Strings Alternately

## Problem Description

You are given two strings `word1` and `word2`. Merge the strings by adding letters in alternating order, starting with `word1`. If a string is longer than the other, append the additional letters onto the end of the merged string.

Return *the merged string*.

### Example 1:

**Input:** `word1 = "abc", word2 = "pqr"`  
**Output:** `"apbqcr"`  
**Explanation:** The merged string will be merged as so:
word1:  a   b   c
word2:    p   q   r
merged: a p b q c r

### Example 2:

**Input:** `word1 = "ab", word2 = "pqrs"`  
**Output:** `"apbqrs"`  
**Explanation:** Notice that as word2 is longer, "rs" is appended to the end.
word1:  a   b 
word2:    p   q   r   s
merged: a p b q   r   s

### Constraints:

- `1 <= word1.length, word2.length <= 100`
- `word1` and `word2` consist of lowercase English letters.

## Solution (Golang)

### Approach: Two Pointers / Single Loop

```go
import "strings"

func mergeAlternately(word1 string, word2 string) string {
    var res strings.Builder
    n1, n2 := len(word1), len(word2)
    i, j := 0, 0
    
    for i < n1 || j < n2 {
        if i < n1 {
            res.WriteByte(word1[i])
            i++
        }
        if j < n2 {
            res.WriteByte(word2[j])
            j++
        }
    }
    
    return res.String()
}
```

## Explanation

1.  **Iterate through both strings**: We use a loop that continues as long as there are characters left in *either* `word1` or `word2`.
2.  **Alternating Append**: 
    - Inside the loop, we first check if the current index `i` is within the bounds of `word1`. If so, we append `word1[i]` to our result and increment `i`.
    - Next, we check if the current index `j` is within the bounds of `word2`. If so, we append `word2[j]` to our result and increment `j`.
3.  **Result**: Using `strings.Builder` is more efficient in Go for concatenation than repeatedly using the `+` operator.

## Complexity

-   **Time Complexity**: $O(n + m)$, where $n$ is the length of `word1` and $m$ is the length of `word2`. We visit every character once.
-   **Space Complexity**: $O(n + m)$ for the output string.