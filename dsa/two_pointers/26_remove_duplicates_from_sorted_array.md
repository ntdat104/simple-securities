# 26. Remove Duplicates from Sorted Array

## Problem Description

Given an integer array `nums` sorted in **non-decreasing order**, remove the duplicates **[in-place](https://en.wikipedia.org/wiki/In-place_algorithm)** such that each unique element appears only **once**. The **relative order** of the elements should be kept the same. Then return the number of unique elements in `nums`.

Consider the number of unique elements of `nums` to be `k`, to get accepted, you need to do the following things:

1.  Change the array `nums` such that the first `k` elements of `nums` contain the unique elements in the order they were present in `nums` initially. The remaining elements of `nums` are not important as well as the size of `nums`.
2.  Return `k`.

### Example 1:

**Input:** `nums = [1,1,2]`  
**Output:** `2, nums = [1,2,_]`  
**Explanation:** Your function should return `k = 2`, with the first two elements of `nums` being 1 and 2 respectively.

### Example 2:

**Input:** `nums = [0,0,1,1,1,2,2,3,3,4]`
**Output:** `5, nums = [0,1,2,3,4,_,_,_,_,_]`  
**Explanation:** Your function should return `k = 5`, with the first five elements of `nums` being 0, 1, 2, 3, and 4 respectively.

### Constraints:

- `1 <= nums.length <= 3 * 10^4`
- `-100 <= nums[i] <= 100`
- `nums` is sorted in **non-decreasing** order.

## Solution (Golang)

### Approach: Two Pointers (Slow and Fast)

Since the array is already sorted, all duplicates will be adjacent. We can use a "slow" pointer to track the position of the last unique element found and a "fast" pointer to iterate through the array.

```go
func removeDuplicates(nums []int) int {
    if len(nums) == 0 {
        return 0
    }

    k := 1 // Position of the next unique element
    for i := 1; i < len(nums); i++ {
        if nums[i] != nums[i-1] {
            nums[k] = nums[i]
            k++
        }
    }
    return k
}
```

## Explanation

1.  **Edge Case**: If the array is empty, return 0. (Though constraints say length >= 1).
2.  **Pointers**:
    - `i` (Fast pointer): Iterates through the array starting from the second element.
    - `k` (Slow pointer): Keeps track of where the next unique element should be placed.
3.  **Logic**: Because the array is sorted, an element is unique if it is different from its predecessor (`nums[i] != nums[i-1]`).
4.  **Update**: When a new unique element is found, we move it to the index `k` and increment `k`.
5.  **Result**: `k` will be the total count of unique elements, and they will occupy the first `k` positions in the array.

## Complexity

- **Time Complexity**: $O(n)$, where $n$ is the length of the array. We visit each element exactly once.
- **Space Complexity**: $O(1)$, as we modify the array in-place and use only one additional variable `k`.
