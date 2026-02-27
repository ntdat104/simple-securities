# 75. Sort Colors

## Problem Description

Given an array `nums` with `n` objects colored red, white, or blue, sort them **[in-place](https://en.wikipedia.org/wiki/In-place_algorithm)** so that objects of the same color are adjacent, with the colors in the order red, white, and blue.

We will use the integers `0`, `1`, and `2` to represent the color red, white, and blue, respectively.

You must solve this problem without using the library's sort function.

### Example 1:

**Input:** `nums = [2,0,2,1,1,0]`  
**Output:** `[0,0,1,1,2,2]`

### Example 2:

**Input:** `nums = [2,0,1]`  
**Output:** `[0,1,2]`

### Constraints:

- `n == nums.length`
- `1 <= n <= 300`
- `nums[i]` is either `0`, `1`, or `2`.

## Solution (Golang)

### Approach: Dutch National Flag Algorithm

This approach uses three pointers to partition the array into three sections: 0s, 1s, and 2s.

```go
func sortColors(nums []int) {
    low, mid, high := 0, 0, len(nums)-1
    
    for mid <= high {
        switch nums[mid] {
        case 0:
            nums[low], nums[mid] = nums[mid], nums[low]
            low++
            mid++
        case 1:
            mid++
        case 2:
            nums[mid], nums[high] = nums[high], nums[mid]
            high--
        }
    }
}
```

## Explanation

1. **Pointers**:
   - `low`: Points to the position where the next `0` should be placed.
   - `mid`: Current element being inspected.
   - `high`: Points to the position where the next `2` should be placed.
2. **The Loop**: We iterate until `mid` passes `high`.
   - If `nums[mid]` is `0`: Swap it with the element at `low`, and increment both `low` and `mid`.
   - If `nums[mid]` is `1`: It's in the correct middle section, so just increment `mid`.
   - If `nums[mid]` is `2`: Swap it with the element at `high`, and decrement `high`. We *don't* increment `mid` here because the new swapped element at `mid` still needs to be inspected.

## Complexity

- **Time Complexity**: $O(n)$, where $n$ is the length of the array. We traverse the array at most once.
- **Space Complexity**: $O(1)$, as we perform the sorting in-place without using extra data structures.