# 167. Two Sum II - Input Array Is Sorted

## Problem Description

Given a **1-indexed** array of integers `numbers` that is already **sorted in non-decreasing order**, find two numbers such that they add up to a specific `target` number. Let these two numbers be `numbers[index1]` and `numbers[index2]` where `1 <= index1 < index2 <= numbers.length`.

Return *the indices of the two numbers, `index1` and `index2`, **added by one** as an integer array `[index1, index2]` of length 2.*

The tests are generated such that there is **exactly one solution**. You **may not** use the same element twice.

Your solution must use only constant extra space.

### Example 1:

**Input:** `numbers = [2,7,11,15], target = 9`  
**Output:** `[1,2]`  
**Explanation:** The sum of 2 and 7 is 9. Therefore, `index1 = 1, index2 = 2`. We return `[1, 2]`.

### Example 2:

**Input:** `numbers = [2,3,4], target = 6`  
**Output:** `[1,3]`  
**Explanation:** The sum of 2 and 4 is 6. Therefore `index1 = 1, index2 = 3`. We return `[1, 3]`.

### Example 3:

**Input:** `numbers = [-1,0], target = -1`  
**Output:** `[1,2]`  
**Explanation:** The sum of -1 and 0 is -1. Therefore `index1 = 1, index2 = 2`. We return `[1, 2]`.

### Constraints:

- `2 <= numbers.length <= 3 * 10^4`
- `-1000 <= numbers[i] <= 1000`
- `numbers` is sorted in **non-decreasing order**.
- `-1000 <= target <= 1000`
- The tests are generated such that there is **exactly one solution**.

## Solution (Golang)

### Approach: Two Pointers

```go
func twoSum(numbers []int, target int) []int {
    left, right := 0, len(numbers)-1
    
    for left < right {
        sum := numbers[left] + numbers[right]
        
        if sum == target {
            return []int{left + 1, right + 1}
        }
        
        if sum < target {
            left++
        } else {
            right--
        }
    }
    
    return nil
}
```

## Explanation

1.  **Initialize Pointers**: We place `left` at the beginning (index 0) and `right` at the end (last index) of the sorted array.
2.  **Iterative Comparison**: 
    -   Calculate the `sum` of values at the two pointers.
    -   **If `sum == target`**: We've found the solution. Since the problem uses 1-based indexing, we return `[left + 1, right + 1]`.
    -   **If `sum < target`**: We need a larger sum. Since the array is sorted, we increment `left` to increase the total.
    -   **If `sum > target`**: We need a smaller sum. We decrement `right` to decrease the total.
3.  **Guarantee**: The problem guarantees exactly one solution, so the loop will always find a result before the pointers cross.

## Complexity

-   **Time Complexity**: $O(n)$, where $n$ is the number of elements in the array. In the worst case, we look at each element once.
-   **Space Complexity**: $O(1)$, as we only use two integer pointers regardless of the input size.
