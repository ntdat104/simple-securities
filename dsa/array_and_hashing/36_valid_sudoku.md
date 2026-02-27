# 36. Valid Sudoku

## Problem Description

Determine if a `9 x 9` Sudoku board is valid. Only the filled cells need to be validated according to the following rules:

1. Each row must contain the digits `1-9` without repetition.
2. Each column must contain the digits `1-9` without repetition.
3. Each of the nine `3 x 3` sub-boxes of the grid must contain the digits `1-9` without repetition.

**Note:**
- A Sudoku board (partially filled) could be valid but is not necessarily solvable.
- Only the filled cells need to be validated according to the mentioned rules.

### Constraints:

- `board.length == 9`
- `board[i].length == 9`
- `board[i][j]` is a digit `1-9` or `'.'`.

## Solution (Golang)

### Approach: Hash Sets tracking Rows, Columns, and Boxes

```go
func isValidSudoku(board [][]byte) bool {
    var rows, cols, boxes [9][9]bool

    for r := 0; r < 9; r++ {
        for c := 0; c < 9; c++ {
            if board[r][c] == '.' {
                continue
            }

            // Convert '1'-'9' to index 0-8
            num := board[r][c] - '1'
            boxIdx := (r / 3) * 3 + (c / 3)

            if rows[r][num] || cols[c][num] || boxes[boxIdx][num] {
                return false
            }

            rows[r][num] = true
            cols[c][num] = true
            boxes[boxIdx][num] = true
        }
    }

    return true
}
```

## Explanation

1.  **Data Structures**: We use three 2D arrays (effectively arrays of hash sets) of size `9x9` to track which numbers have appeared in each row, each column, and each of the nine `3x3` boxes.
2.  **Iterating the Board**: We traverse every cell `(r, c)` of the $9 \times 9$ grid.
3.  **Skipping Empty Cells**: If a cell contains `'.'`, we skip it.
4.  **Index Calculation**:
    - `num`: We map the character digits '1'-'9' to indices 0-8.
    - `boxIdx`: To determine which of the nine `3x3` boxes a cell belongs to, we use the formula `(row / 3) * 3 + (col / 3)`.
5.  **Validation Check**: We check if the current `num` has already been recorded in the current row, column, or box. If so, the board is invalid.
6.  **Recording**: If not already present, we mark the number as "seen" in the corresponding row, column, and box.

## Complexity

-   **Time Complexity**: $O(1)$. Although it looks like $O(N^2)$ where $N=9$, the board size is fixed at 81 cells, so the number of operations is constant.
-   **Space Complexity**: $O(1)$. The auxiliary space used for the `rows`, `cols`, and `boxes` arrays is fixed (totaling $3 \times 9 \times 9$ bits/booleans).
