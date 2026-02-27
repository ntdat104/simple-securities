# 912. Sort an Array

## Problem Description

Given an array of integers `nums`, sort the array in ascending order and return it.

You must solve the problem without using any built-in library functions in $O(n \log n)$ time complexity and with the smallest space complexity possible.

### Example 1:

**Input:** `nums = [5,2,3,1]`  
**Output:** `[1,2,3,5]`

### Example 2:

**Input:** `nums = [5,1,1,2,0,0]`  
**Output:** `[0,0,1,1,2,5]`

### Constraints:

- `1 <= nums.length <= 5 * 10^4`
- `-5 * 10^4 <= nums[i] <= 5 * 10^4`

## Solution (Golang)

### Approach: Merge Sort

Merge Sort is a stable, divide-and-conquer algorithm that guarantees $O(n \log n)$ time complexity.

```go
func sortArray(nums []int) []int {
    if len(nums) <= 1 {
        return nums
    }

    mid := len(nums) / 2
    left := sortArray(nums[:mid])
    right := sortArray(nums[mid:])

    return merge(left, right)
}

func merge(left, right []int) []int {
    result := make([]int, 0, len(left)+len(right))
    i, j := 0, 0

    for i < len(left) && j < len(right) {
        if left[i] < right[j] {
            result = append(result, left[i])
            i++
        } else {
            result = append(result, right[j])
            j++
        }
    }

    result = append(result, left[i:]...)
    result = append(result, right[j:]...)
    return result
}
```

## Explanation

1.  **Divide**: We recursively split the array into two halves until each sub-array contains only one element (which is inherently sorted).
2.  **Conquer**: The `merge` function takes two sorted arrays and combines them into one sorted array.
3.  **Merge Process**:
    - We use two pointers (`i` and `j`) to track our position in the `left` and `right` arrays.
    - We compare the elements at these pointers and append the smaller one to the `result`.
    - Once one array is exhausted, we append the remaining elements of the other array.

## Complexity

-   **Time Complexity**: $O(n \log n)$. The array is divided $\log n$ times, and each level of merging takes $O(n)$ time.
-   **Space Complexity**: $O(n)$. Merge sort requires extra space to store the merged arrays during the process.
