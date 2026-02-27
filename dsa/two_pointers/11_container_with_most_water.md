# 11. Container With Most Water

## Problem Description

You are given an integer array `height` of length `n`. There are `n` vertical lines drawn such that the two endpoints of the `i-th` line are `(i, 0)` and `(i, height[i])`.

Find two lines that together with the x-axis form a container, such that the container contains the most water.

Return *the maximum amount of water a container can store*.

**Notice** that you may not slant the container.

### Example 1:

**Input:** `height = [1,8,6,2,5,4,8,3,7]`  
**Output:** `49`  
**Explanation:** The above vertical lines are represented by array `[1,8,6,2,5,4,8,3,7]`. In this case, the max area of water the container can contain is 49.

### Example 2:

**Input:** `height = [1,1]`  
**Output:** `1`

### Constraints:

- `n == height.length`
- `2 <= n <= 10^5`
- `0 <= height[i] <= 10^4`

## Solution (Golang)

### Approach: Two Pointers

We use two pointers, one at the beginning and one at the end of the array. We calculate the area and then move the pointer that points to the shorter line, as this is the only way we might find a larger area.

```go
func maxArea(height []int) int {
    left, right := 0, len(height)-1
    maxWater := 0
    
    for left < right {
        w := right - left
        h := min(height[left], height[right])
        area := w * h
        
        if area > maxWater {
            maxWater = area
        }
        
        if height[left] < height[right] {
            left++
        } else {
            right--
        }
    }
    return maxWater
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
```

## Explanation

1.  **Initialize Pointers**: Place `left` at index `0` and `right` at the last index.
2.  **Calculate Area**: The area is determined by the distance between pointers (`width`) multiplied by the height of the shorter line (`min(height[left], height[right])`).
3.  **Update Maximum**: Keep track of the largest area seen so far.
4.  **Move Shorter Side**: 
    - The width will always decrease as we move the pointers closer.
    - To potentially increase the area, we need to find a taller height.
    - If we move the pointer pointing to the taller side, the height of the container will still be limited by the shorter side, and the width will decrease, leading to a smaller area.
    - Therefore, we must move the pointer that currently has the **smaller** height.
5.  **Termination**: The loop ends when the pointers meet.

## Complexity

-   **Time Complexity**: $O(n)$, where $n$ is the length of the array. We traverse the array once.
-   **Space Complexity**: $O(1)$, as we only use a few variables.
