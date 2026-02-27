# 128. Longest Consecutive Sequence

## Problem Description

Given an unsorted array of integers `nums`, return the length of the longest consecutive elements sequence.

You must write an algorithm that runs in $O(n)$ time.

### Example 1:

**Input:** `nums = [100,4,200,1,3,2]`  
**Output:** `4`  
**Explanation:** The longest consecutive elements sequence is `[1, 2, 3, 4]`. Therefore its length is 4.

### Example 2:

**Input:** `nums = [0,3,7,2,5,8,4,6,0,1]`  
**Output:** `9`

### Constraints:

- `0 <= nums.length <= 10^5`
- `-10^9 <= nums[i] <= 10^9`

## Solution (Golang)

### Approach: Hash Set

We use a hash set for $O(1)$ lookups. To achieve $O(n)$ time, we only start counting a sequence from its smallest element (i.e., when `num - 1` is not in the set).

```go
func longestConsecutive(nums []int) int {
    numSet := make(map[int]bool)
    for _, num := range nums {
        numSet[num] = true
    }

    longest := 0
    for num := range numSet {
        // Only start check if 'num' is the beginning of a sequence
        if !numSet[num-1] {
            currentNum := num
            currentStreak := 1

            for numSet[currentNum+1] {
                currentNum++
                currentStreak++
            }

            if currentStreak > longest {
                longest = currentStreak
            }
        }
    }
    return longest
}
```

## Explanation

1. **Hash Set Storage**: We first insert all numbers into a map (set) to allow for $O(1)$ existence checks.
2. **Finding Sequence Starts**: We iterate through the set. A number `num` is the start of a sequence only if `num - 1` does not exist in the set.
3. **Counting Streak**: If it is a start, we use a `while` loop to check for the presence of `num + 1`, `num + 2`, etc., increasing the current streak counter.
4. **Efficiency**: Although there is a nested loop, each number is visited exactly twice (once in the outer loop and once in the inner loop across the entire execution), resulting in $O(n)$ complexity.

## Complexity

- **Time Complexity**: $O(n)$, where $n$ is the number of elements in the array.
- **Space Complexity**: $O(n)$ for storing the elements in the hash map.
