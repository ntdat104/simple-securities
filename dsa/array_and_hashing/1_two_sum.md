# 1. Two Sum

## Problem Description

Given an array of integers `nums` and an integer `target`, return _indices of the two numbers such that they add up to `target`_.

You may assume that each input would have **exactly one solution**, and you may not use the _same_ element twice.

You can return the answer in any order.

### Example 1:

**Input:** `nums = [2,7,11,15]`, `target = 9`  
**Output:** `[0,1]`  
**Explanation:** Because `nums[0] + nums[1] == 9`, we return `[0, 1]`.

### Example 2:

**Input:** `nums = [3,2,4]`, `target = 6`  
**Output:** `[1,2]`

### Example 3:

**Input:** `nums = [3,3]`, `target = 6`  
**Output:** `[0,1]`

### Constraints:

- `2 <= nums.length <= 10^4`
- `-10^9 <= nums[i] <= 10^9`
- `-10^9 <= target <= 10^9`
- **Only one valid answer exists.**

## Solution (Golang)

### Approach: One-Pass Hash Map

We can use a hash map to store the elements we have already seen. For each element, we check if its complement (target - current element) exists in the map.

```go
func twoSum(nums []int, target int) []int {
    prevMap := make(map[int]int) // value : index

    for i, n := range nums {
        diff := target - n
        if j, ok := prevMap[diff]; ok {
            return []int{j, i}
        }
        prevMap[n] = i
    }
    return nil
}
```

## Explanation

1. **Hash Map initialization**: We create a map `prevMap` where the keys are the values from the array and the values are their indices.
2. **Iteration**: We loop through the array `nums` using both index `i` and value `n`.
3. **Difference Calculation**: For each number, we calculate the required `diff` (complement) to reach the `target` (`diff = target - n`).
4. **Lookup**: We check if this `diff` already exists in our `prevMap`.
   - If it exists, we have found our two numbers. We return their indices: `[prevMap[diff], i]`.
   - If it doesn't exist, we add the current number `n` and its index `i` to the `prevMap` and continue.
5. This approach works in a single pass because for any pair `(a, b)` that sums to `target`, when we reach the second number (say `b`), the first number (`a`) will already be in the map.

## Complexity

- **Time Complexity**: $O(n)$, where $n$ is the number of elements in the array. We traverse the list containing $n$ elements only once. Each lookup in the table costs only $O(1)$ time.
- **Space Complexity**: $O(n)$, the extra space required depends on the number of items stored in the hash map, which stores at most $n$ elements.
