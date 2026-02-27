# 41. First Missing Positive

## Problem Description

Given an unsorted integer array `nums`, return the smallest missing positive integer.

You must implement an algorithm that runs in $O(n)$ time and uses $O(1)$ auxiliary space.

### Example 1:

**Input:** `nums = [1,2,0]`  
**Output:** `3`  
**Explanation:** The numbers in the range [1,2] are all in the array.

### Example 2:

**Input:** `nums = [3,4,-1,1]`  
**Output:** `2`  
**Explanation:** 1 is in the array but 2 is missing.

### Example 3:

**Input:** `nums = [7,8,9,11,12]`  
**Output:** `1`  
**Explanation:** The smallest positive integer 1 is missing.

### Constraints:

- `1 <= nums.length <= 10^5`
- `-2^31 <= nums[i] <= 2^31 - 1`

## Solution (Golang)

### Approach: Cyclic Sort (In-place Swap)

To find the smallest missing positive in $O(n)$ time and $O(1)$ space, we try to place every number `x` at its target index `x - 1` (e.g., the number `1` should be at index `0`).

```go
func firstMissingPositive(nums []int) int {
    n := len(nums)
    
    for i := 0; i < n; i++ {
        // While the current number is in the valid range [1, n] 
        // and it is not at its correct position (nums[i]-1), swap it.
        for nums[i] > 0 && nums[i] <= n && nums[nums[i]-1] != nums[i] {
            nums[i], nums[nums[i]-1] = nums[nums[i]-1], nums[i]
        }
    }
    
    // Check which index does not contain the correct number
    for i := 0; i < n; i++ {
        if nums[i] != i+1 {
            return i + 1
        }
    }
    
    return n + 1
}
```

## Explanation

1.  **Rearranging (Cyclic Sort)**: We iterate through the array. For each number `nums[i]`, if it's a positive integer within the range `[1, n]` and it's not already at index `nums[i]-1`, we swap it with the element at that target index. We use a `while` loop (written as `for` in Go) to keep swapping the "swapped-in" element until the condition is no longer met.
2.  **Identifying the Gap**: After rearranging, we traverse the array again. The first index `i` where `nums[i] != i + 1` tells us that the number `i + 1` is missing.
3.  **Edge Case**: If all numbers from `1` to `n` are present and in their correct spots, the smallest missing positive is `n + 1`.

## Complexity

-   **Time Complexity**: $O(n)$. Although there is a nested loop, each swap operation puts at least one number in its correct final position. Since there are $n$ positions, there can be at most $n$ swaps in total across the entire execution.
-   **Space Complexity**: $O(1)$. We modify the input array in-place and use no extra data structures.
