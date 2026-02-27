# 304. Range Sum Query 2D - Immutable

## Problem Description

Given a 2D matrix `matrix`, handle multiple queries of the following type:

- Calculate the sum of the elements of `matrix` inside the rectangle defined by its upper left corner `(row1, col1)` and lower right corner `(row2, col2)`.

Implement the `NumMatrix` class:

- `NumMatrix(int[][] matrix)` Initializes the object with the integer matrix `matrix`.
- `int sumRegion(int row1, int col1, int row2, int col2)` Returns the sum of the elements of `matrix` inside the rectangle defined by its upper left corner `(row1, col1)` and lower right corner `(row2, col2)`.

You must design an algorithm where `sumRegion` works on $O(1)$ time complexity.

### Example 1:

**Input:**
`["NumMatrix", "sumRegion", "sumRegion", "sumRegion"]`
`[[[[3, 0, 1, 4, 2], [5, 6, 3, 2, 1], [1, 2, 0, 1, 5], [4, 1, 0, 1, 7], [1, 0, 3, 0, 5]]], [2, 1, 4, 3], [1, 1, 2, 2], [1, 2, 2, 4]]`

**Output:**
`[null, 8, 11, 12]`

## Solution (Golang)

### Approach: 2D Prefix Sum

We can precompute a 2D prefix sum matrix where `sums[i][j]` stores the sum of all elements in the rectangle from `(0,0)` to `(i-1, j-1)`.

```go
type NumMatrix struct {
    sums [][]int
}

func Constructor(matrix [][]int) NumMatrix {
    if len(matrix) == 0 || len(matrix[0]) == 0 {
        return NumMatrix{}
    }
    rows, cols := len(matrix), len(matrix[0])
    sums := make([][]int, rows+1)
    for i := range sums {
        sums[i] = make([]int, cols+1)
    }

    for r := 0; r < rows; r++ {
        for c := 0; c < cols; c++ {
            sums[r+1][c+1] = matrix[r][c] + sums[r][c+1] + sums[r+1][c] - sums[r][c]
        }
    }
    return NumMatrix{sums: sums}
}

func (this *NumMatrix) SumRegion(row1 int, col1 int, row2 int, col2 int) int {
    return this.sums[row2+1][col2+1] - this.sums[row1][col2+1] - this.sums[row2+1][col1] + this.sums[row1][col1]
}
```

## Explanation

1.  **Prefix Sum Definition**: `sums[r+1][c+1]` represents the sum of the sub-matrix with top-left `(0,0)` and bottom-right `(r,c)`.
2.  **Building the Matrix**: To calculate `sums[r+1][c+1]`, we take the value at `matrix[r][c]`, add the prefix sum above it (`sums[r][c+1]`), add the prefix sum to its left (`sums[r+1][c]`), and subtract the overlapping diagonal prefix sum (`sums[r][c]`) that was added twice.
3.  **Region Query**: The sum of any rectangle `(row1, col1)` to `(row2, col2)` can be calculated using the inclusion-exclusion principle:
    -   Start with the total sum from `(0,0)` to `(row2, col2)`.
    -   Subtract the rectangle above: `(0,0)` to `(row1-1, col2)`.
    -   Subtract the rectangle to the left: `(0,0)` to `(row2, col1-1)`.
    -   Add back the rectangle subtracted twice: `(0,0)` to `(row1-1, col1-1)`.

## Complexity

-   **Time Complexity**:
    -   `Constructor`: $O(M \times N)$ where $M$ is the number of rows and $N$ is the number of columns.
    -   `SumRegion`: $O(1)$ per query.
-   **Space Complexity**: $O(M \times N)$ to store the prefix sum matrix.
