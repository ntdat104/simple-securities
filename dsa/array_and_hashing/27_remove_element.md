# 27. Remove Element

## Problem Description

Given an integer array `nums` and an integer `val`, remove all occurrences of `val` in `nums` **in-place**. The order of the elements may be changed. Then return the number of elements in `nums` which are not equal to `val`.

Consider the number of elements in `nums` which are not equal to `val` be `k`, to get accepted, you need to do the following things:

1.  Change the array `nums` such that the first `k` elements of `nums` contain the elements which are not equal to `val`. The remaining elements of `nums` are not important as well as the size of `nums`.
2.  Return `k`.

### Example 1:

**Input:** `nums = [3,2,2,3]`, `val = 3`  
**Output:** `2, nums = [2,2,_,_]`  
**Explanation:** Your function should return `k = 2`, with the first two elements of `nums` being 2.

### Example 2:

**Input:** `nums = [0,1,2,2,3,0,4,2]`, `val = 2`  
**Output:** `5, nums = [0,1,3,0,4,_,_,_]`  
**Explanation:** Your function should return `k = 5`, with the first five elements of `nums` containing 0, 1, 3, 0, and 4.

### Constraints:

- `0 <= nums.length <= 100`
- `0 <= nums[i] <= 50`
- `0 <= val <= 100`

## Solution (Golang)

### Approach: Two Pointers

```go
func removeElement(nums []int, val int) int {
    k := 0
    for i := 0; i < len(nums); i++ {
        if nums[i] != val {
            nums[k] = nums[i]
            k++
        }
    }
    return k
}
```

## Explanation

1.  We maintain a pointer `k` which tracks the index where the next "valid" element (not equal to `val`) should be placed.
2.  We iterate through the array with a second pointer `i`.
3.  When `nums[i]` is not equal to `val`, we copy `nums[i]` to `nums[k]` and increment `k`.
4.  By the end of the loop, all elements not equal to `val` have been moved to the front of the array, and `k` is the count of those elements.

## Complexity

- **Time Complexity**: $O(n)$, where $n$ is the length of the array. We traverse the array once.
- **Space Complexity**: $O(1)$ since we modify the array in-place and use only one extra variable.
