# 217. Contains Duplicate

## Problem Description

Given an integer array `nums`, return `true` if any value appears **at least twice** in the array, and return `false` if every element is distinct.

### Example 1:

**Input:** `nums = [1,2,3,1]`  
**Output:** `true`  
**Explanation:** The element 1 occurs at indices 0 and 3.

### Example 2:

**Input:** `nums = [1,2,3,4]`  
**Output:** `false`  
**Explanation:** All elements are distinct.

### Example 3:

**Input:** `nums = [1,1,1,3,3,4,3,2,4,2]`  
**Output:** `true`

### Constraints:

- `1 <= nums.length <= 10^5`
- `-10^9 <= nums[i] <= 10^9`

## Solution (Golang)

### Approach: Using a Hash Map (Set)

The most efficient way to check for duplicates is to store elements we've already seen in a hash map.

```go
func containsDuplicate(nums []int) bool {
    seen := make(map[int]bool)
    
    for _, num := range nums {
        if seen[num] {
            return true
        }
        seen[num] = true
    }
    
    return false
}
```

## Explanation

1. We initialize an empty hash map (dictionary) called `seen` where the keys are the integers from the array and the values are booleans.
2. We iterate through the array `nums`.
3. For each number `num`, we check if it already exists in the `seen` map:
   - If it does, it means we've encountered this number before, so we found a duplicate. We return `true`.
   - If it doesn't, we add `num` to the map.
4. If we finish the loop without returning `true`, it means all elements are unique, so we return `false`.

## Complexity

- **Time Complexity**: $O(n)$, where $n$ is the length of the input array. We only iterate through the array once, and hash map lookups/insertions are $O(1)$ on average.
- **Space Complexity**: $O(n)$ in the worst case where all elements are distinct, as we store every element in the hash map.
