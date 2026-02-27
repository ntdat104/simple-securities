# 189. Rotate Array

## Problem Description

Given an integer array `nums`, rotate the array to the right by `k` steps, where `k` is non-negative.

### Example 1:

**Input:** `nums = [1,2,3,4,5,6,7]`, `k = 3`  
**Output:** `[5,6,7,1,2,3,4]`  
**Explanation:**
- rotate 1 steps to the right: `[7,1,2,3,4,5,6]`
- rotate 2 steps to the right: `[6,7,1,2,3,4,5]`
- rotate 3 steps to the right: `[5,6,7,1,2,3,4]`

### Example 2:

**Input:** `nums = [-1,-100,3,99]`, `k = 2`  
**Output:** `[3,99,-1,-100]`  
**Explanation:** 
- rotate 1 steps to the right: `[99,-1,-100,3]`
- rotate 2 steps to the right: `[3,99,-1,-100]`

### Constraints:

- `1 <= nums.length <= 10^5`
- `-2^31 <= nums[i] <= 2^31 - 1`
- `0 <= k <= 10^5`

## Solution (Golang)

### Approach: Triple Reversal

This approach rotates the array in-place by reversing sections of the array.

```go
func rotate(nums []int, k int) {
    n := len(nums)
    k = k % n
    if k == 0 {
        return
    }

    // 1. Reverse the entire array
    reverse(nums, 0, n-1)
    // 2. Reverse the first k elements
    reverse(nums, 0, k-1)
    // 3. Reverse the remaining n-k elements
    reverse(nums, k, n-1)
}

func reverse(nums []int, start, end int) {
    for start < end {
        nums[start], nums[end] = nums[end], nums[start]
        start++
        end--
    }
}
```

## Explanation

1.  **Normalization**: First, we calculate `k % n` because rotating an array of size `n` by `n` steps results in the same array.
2.  **Step 1: Reverse All**: Reversing the entire array brings the elements that should be at the front to the front, but in reverse order. ([1,2,3,4,5] with k=2 becomes [5,4,3,2,1])
3.  **Step 2: Reverse First K**: Reversing the first `k` elements puts them in their correct relative order. ([5,4,3,2,1] -> [4,5,3,2,1])
4.  **Step 3: Reverse the Rest**: Reversing the elements from index `k` to the end restores their original relative order. ([4,5,3,2,1] -> [4,5,1,2,3])

## Complexity

-   **Time Complexity**: $O(n)$, where $n$ is the length of the array. We visit each element a constant number of times (at most twice).
-   **Space Complexity**: $O(1)$, as we perform the rotation in-place without any extra data structures.
