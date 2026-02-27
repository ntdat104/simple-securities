# 88. Merge Sorted Array

## Problem Description

You are given two integer arrays `nums1` and `nums2`, sorted in non-decreasing order, and two integers `m` and `n`, representing the number of elements in `nums1` and `nums2` respectively.

**Merge** `nums1` and `nums2` into a single array sorted in non-decreasing order.

The final sorted array should not be returned by the function, but instead be stored inside the array `nums1`. To accommodate this, `nums1` has a length of `m + n`, where the first `m` elements denote the elements that should be merged, and the last `n` elements are set to `0` and should be ignored. `nums2` has a length of `n`.

### Example 1:

**Input:** `nums1 = [1,2,3,0,0,0]`, `m = 3`, `nums2 = [2,5,6]`, `n = 3`  
**Output:** `[1,2,2,3,5,6]`  
**Explanation:** The arrays we are merging are `[1,2,3]` and `[2,5,6]`. The result of the merge is `[1,2,2,3,5,6]` with the underlined elements coming from `nums1`.

### Example 2:

**Input:** `nums1 = [1]`, `m = 1`, `nums2 = []`, `n = 0`  
**Output:** `[1]`  
**Explanation:** The arrays we are merging are `[1]` and `[]`. The result of the merge is `[1]`.

### Example 3:

**Input:** `nums1 = [0]`, `m = 0`, `nums2 = [1]`, `n = 1`  
**Output:** `[1]`  
**Explanation:** The arrays we are merging are `[]` and `[1]`. The result of the merge is `[1]`. Note that because `m = 0`, there are no elements in `nums1`. The `0` is only there to ensure the merge result can fit in `nums1`.

### Constraints:

- `nums1.length == m + n`
- `nums2.length == n`
- `0 <= m, n <= 200`
- `1 <= m + n <= 200`
- `-10^9 <= nums1[i], nums2[j] <= 10^9`

## Solution (Golang)

### Approach: Three Pointers (From End to Front)

Since `nums1` has enough space at the end, we can start comparing elements from the back of both arrays and place the largest element at the very end of `nums1`. This prevents us from overwriting elements in `nums1` that we still need to compare.

```go
func merge(nums1 []int, m int, nums2 []int, n int) {
    p1 := m - 1      // End of valid nums1 elements
    p2 := n - 1      // End of nums2 elements
    p := m + n - 1   // Destination index in nums1

    for p2 >= 0 {
        if p1 >= 0 && nums1[p1] > nums2[p2] {
            nums1[p] = nums1[p1]
            p1--
        } else {
            nums1[p] = nums2[p2]
            p2--
        }
        p--
    }
}
```

## Explanation

1.  **Pointers**: We initialize three pointers:
    -   `p1`: points to the last "real" element in `nums1`.
    -   `p2`: points to the last element in `nums2`.
    -   `p`: points to the very last index of the combined `nums1`.
2.  **Comparison Loop**: We compare `nums1[p1]` and `nums2[p2]`. We place the larger value at `nums1[p]` and decrement the corresponding pointer (`p1` or `p2`) and the target pointer `p`.
3.  **Completion Logic**:
    -   If `p2` reaches `-1` first, all elements of `nums2` are merged. The remaining elements of `nums1` are already in their correct sorted positions.
    -   If `p1` reaches `-1` first, we simply copy the remaining elements from `nums2` into `nums1`.
4.  **Why from the back?**: Moving from front to back would require shifting elements in `nums1` every time we insert something from `nums2`. Moving from back to front is $O(1)$ space and $O(n+m)$ time.

## Complexity

-   **Time Complexity**: $O(m + n)$, where $m$ and $n$ are the number of elements in `nums1` and `nums2`.
-   **Space Complexity**: $O(1)$, as we are modifying `nums1` in-place.
