# 42. Trapping Rain Water

## Problem Description

Given `n` non-negative integers representing an elevation map where the width of each bar is `1`, compute how much water it can trap after raining.

### Example 1:

![Example 1 Image](https://assets.leetcode.com/uploads/2018/10/22/rainwatertrap.png)
**Input:** `height = [0,1,0,2,1,0,1,3,2,1,2,1]`  
**Output:** `6`  
**Explanation:** The above elevation map (black section) is represented by array [0,1,0,2,1,0,1,3,2,1,2,1]. In this case, 6 units of rain water (blue section) are being trapped.

### Example 2:

**Input:** `height = [4,2,0,3,2,5]`  
**Output:** `9`

### Constraints:

- `n == height.length`
- `1 <= n <= 2 * 10^4`
- `0 <= height[i] <= 10^5`

## Solution (Golang)

### Approach: Two Pointers

This approach uses two pointers, `left` and `right`, to scan the elevation map from both ends. We keep track of the maximum height seen so far from the left (`maxLeft`) and from the right (`maxRight`).

```go
func trap(height []int) int {
    if len(height) == 0 {
        return 0
    }

    left, right := 0, len(height)-1
    maxLeft, maxRight := height[left], height[right]
    totalWater := 0

    for left < right {
        if maxLeft < maxRight {
            left++
            if height[left] < maxLeft {
                totalWater += maxLeft - height[left]
            } else {
                maxLeft = height[left]
            }
        } else {
            right--
            if height[right] < maxRight {
                totalWater += maxRight - height[right]
            } else {
                maxRight = height[right]
            }
        }
    }

    return totalWater
}
```

## Explanation

1.  **Iterative Process**: We move from both ends of the array towards the center using `left` and `right` pointers.
2.  **Maintaining Max Heights**: At any position, the amount of water trapped depends on the minimum of the maximum height to its left and the maximum height to its right.
3.  **The Logic**:
    - If `maxLeft < maxRight`, it means any water trapped at the `left` pointer position is limited by `maxLeft` (because we know there is something at least as high as `maxRight` on the right side).
    - If the current `height[left]` is less than `maxLeft`, the difference is the water trapped. Otherwise, we update `maxLeft`.
    - We apply the same logic symmetrically for the `right` pointer when `maxRight <= maxLeft`.
4.  **Why this works**: By moving the pointer that points to the smaller maximum height, we ensure that the "bottleneck" for trapping water is accurately represented by that smaller maximum.

## Complexity

-   **Time Complexity**: $O(n)$, where $n$ is the length of the array. We traverse the array once.
-   **Space Complexity**: $O(1)$, as we only use a few variables for pointers and maximum heights.
