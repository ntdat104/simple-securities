# 169. Majority Element

## Problem Description

Given an array `nums` of size `n`, return the *majority element*.

The **majority element** is the element that appears more than `⌊n / 2⌋` times. You may assume that the majority element always exists in the array.

### Example 1:

**Input:** `nums = [3,2,3]`  
**Output:** `3`

### Example 2:

**Input:** `nums = [2,2,1,1,1,2,2]`  
**Output:** `2`

### Constraints:

- `n == nums.length`
- `1 <= n <= 5 * 10^4`
- `-10^9 <= nums[i] <= 10^9`

## Solution (Golang)

### Approach: Boyer-Moore Voting Algorithm

This algorithm finds the majority element in optimal time and space complexity by maintaining a candidate and a counter.

```go
func majorityElement(nums []int) int {
    candidate := 0
    count := 0

    for _, num := range nums {
        if count == 0 {
            candidate = num
        }
        
        if num == candidate {
            count++
        } else {
            count--
        }
    }

    return candidate
}
```

## Explanation

1. **Initialization**: We start with no `candidate` and a `count` of 0.
2. **Iteration**: As we iterate through the numbers:
    - If `count` is 0, we set the current number as the new `candidate`.
    - If the current number matches the `candidate`, we increment `count`.
    - If it doesn't match, we decrement `count`.
3. **Logic**: Because the majority element appears more than half the time, it will always "win" the voting process and remain as the candidate at the end of the array traversal.

## Complexity

- **Time Complexity**: $O(n)$, where $n$ is the length of the array. We perform a single pass.
- **Space Complexity**: $O(1)$, as we only use two variables regardless of the input size.
