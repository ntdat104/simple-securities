# 238. Product of Array Except Self

## Problem Description

Given an integer array `nums`, return an array `answer` such that `answer[i]` is equal to the product of all the elements of `nums` except `nums[i]`.

The product of any prefix or suffix of `nums` is guaranteed to fit in a **32-bit** integer.

You must write an algorithm that runs in $O(n)$ time and without using the division operation.

### Example 1:

**Input:** `nums = [1,2,3,4]`  
**Output:** `[24,12,8,6]`

### Example 2:

**Input:** `nums = [-1,1,0,-3,3]`  
**Output:** `[0,0,9,0,0]`

### Constraints:

- `2 <= nums.length <= 10^5`
- `-30 <= nums[i] <= 30`
- The product of any prefix or suffix of `nums` is **guaranteed** to fit in a **32-bit** integer.

## Solution (Golang)

### Approach: Prefix and Suffix Products

We can solve this by calculating the prefix products and suffix products for each element. To optimize space, we can store the prefix products in the result array and then multiply by the suffix products on the fly.

```go
func productExceptSelf(nums []int) []int {
    n := len(nums)
    res := make([]int, n)
    
    // Step 1: Calculate prefix products
    res[0] = 1
    for i := 1; i < n; i++ {
        res[i] = res[i-1] * nums[i-1]
    }
    
    // Step 2: Calculate suffix products and multiply with prefix products
    right := 1
    for i := n - 1; i >= 0; i-- {
        res[i] *= right
        right *= nums[i]
    }
    
    return res
}
```

## Explanation

1. **Prefix Pass**: We iterate through the array from left to right. At each index `i`, we store the product of all elements to the left of `i` in `res[i]`. `res[0]` is initialized to 1 since there are no elements to its left.
2. **Suffix Pass**: We iterate through the array from right to left. We maintain a variable `right` which stores the product of all elements to the right of the current index. We multiply the existing value in `res[i]` (which is the prefix product) by `right` (which is the suffix product).
3. **Result**: The combination of the product of everything to the left and everything to the right gives the product of everything except the element at index `i`.

## Complexity

- **Time Complexity**: $O(n)$, where $n$ is the length of the array. We make two passes over the input array.
- **Space Complexity**: $O(1)$ if we ignore the output array as per the problem's follow-up suggestion. Otherwise, $O(n)$ for the result array.
