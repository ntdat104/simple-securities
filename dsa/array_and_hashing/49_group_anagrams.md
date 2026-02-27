# 49. Group Anagrams

## Problem Description

Given an array of strings `strs`, group the **anagrams** together. You can return the answer in **any order**.

An **Anagram** is a word or phrase formed by rearranging the letters of a different word or phrase, typically using all the original letters exactly once.

### Example 1:

**Input:** `strs = ["eat","tea","tan","ate","nat","bat"]`  
**Output:** `[["bat"],["nat","tan"],["ate","eat","tea"]]`

### Example 2:

**Input:** `strs = [""]`  
**Output:** `[[""]]`

### Example 3:

**Input:** `strs = ["a"]`  
**Output:** `[["a"]]`

### Constraints:

- `1 <= strs.length <= 10^4`
- `0 <= strs[i].length <= 100`
- `strs[i]` consists of lowercase English letters.

## Solution (Golang)

### Approach: Hash Map with Character Frequency

Since anagrams have the same character frequencies, we can use a frequency array of size 26 as the key for our hash map.

```go
func groupAnagrams(strs []string) [][]string {
    groups := make(map[[26]int][]string)
    
    for _, s := range strs {
        var count [26]int
        for i := 0; i < len(s); i++ {
            count[s[i]-'a']++
        }
        groups[count] = append(groups[count], s)
    }
    
    result := make([][]string, 0, len(groups))
    for _, group := range groups {
        result = append(result, group)
    }
    
    return result
}
```

## Explanation

1. **Hash Map initialization**: We create a map `groups` where the key is an array of 26 integers `[26]int`. In Go, arrays are comparable, so they can be used as map keys.
2. **Frequency Count**: For each string `s` in `strs`:
   - We initialize a `count` array of size 26.
   - we iterate through the characters of `s` and increment the corresponding index in `count`.
3. **Grouping**: We use the `count` array as a key in our map and append the original string `s` to the slice associated with that key. All strings that are anagrams of each other will produce the same `count` array and thus be grouped together.
4. **Result formation**: Finally, we iterate through the values of the map and collect the slices into our `result` `[][]string`.

## Complexity

- **Time Complexity**: $O(n \cdot k)$, where $n$ is the number of strings and $k$ is the maximum length of a string. We process each character of every string exactly once.
- **Space Complexity**: $O(n \cdot k)$ to store the grouped strings in the hash map.
