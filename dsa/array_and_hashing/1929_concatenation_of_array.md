# 1929. Concatenation of Array

## Problem Description

Given an integer array `nums` of length `n`, you want to create an array `ans` of length `2n` where `ans[i] == nums[i]` and `ans[i + n] == nums[i]` for `0 <= i < n` (0-indexed).

Specifically, `ans` is the concatenation of two `nums` arrays.

Return the array `ans`.

### Example 1:

**Input:** `nums = [1,2,1]`  
**Output:** `[1,2,1,1,2,1]`  
**Explanation:** The array `ans` is formed as follows:
- `ans = [nums[0],nums[1],nums[2],nums[0],nums[1],nums[2]]`
- `ans = [1,2,1,1,2,1]`

### Example 2:

**Input:** `nums = [1,3,2,1]`  
**Output:** `[1,3,2,1,1,3,2,1]`  
**Explanation:** The array `ans` is formed as follows:
- `ans = [nums[0],nums[1],nums[2],nums[3],nums[0],nums[1],nums[2],nums[3]]`
- `ans = [1,3,2,1,1,3,2,1]`

### Constraints:

- `n == nums.length`
- `1 <= n <= 1000`
- `1 <= nums[i] <= 1000`

## Solution (Golang)

### Approach 1: Using `currentIdx`

```go
func solve(nums []int, loop int) []int {
	size := len(nums)
	finalSize := size * loop
	final := make([]int, finalSize)
	
	currentIdx := 0
	
	for i := 0; i < finalSize; i++ {
		if currentIdx > (size - 1) {
			currentIdx = 0
		}
		final[i] = nums[currentIdx]
		currentIdx++
	}
	
	return final
}
```

### Approach 2: Using `i,j*n`

```go
func solve(nums []int, loop int) []int {
	n := len(nums)
	final := make([]int, loop*n)

	for i := 0; i < n; i++ {
		for j := 0; j < loop; j++ {
			final[i+(j*n)] = nums[i]
		}
	}

	return final
}
```

### Approach 3: Using `i%size`

```go
func solve(nums []int, loop int) []int {
	size := len(nums)
	finalSize := size * loop
	final := make([]int, finalSize)

	for i := range final {
		final[i] = nums[i%size]
	}

	return final
}
```

### Approach 4: Using `append`

```go
func solve(nums []int, loop int) []int {
	final := make([]int, 0, len(nums)*loop)

	for i := 0; i < loop; i++ {
		for _, v := range nums {
			final = append(final, v)
		}
	}

	return final
}
```
