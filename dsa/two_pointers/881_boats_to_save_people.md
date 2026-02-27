# 881. Boats to Save People

## Problem Description

You are given an array `people` where `people[i]` is the weight of the `i-th` person, and an **infinite number of boats** where each boat can carry a maximum weight of `limit`. Each boat carries at most two people at the same time, provided the sum of the weight of those people is at most `limit`.

Return the minimum number of boats to carry every given person.

### Example 1:

**Input:** `people = [1,2]`, `limit = 3`  
**Output:** `1`  
**Explanation:** 1 boat (1, 2)

### Example 2:

**Input:** `people = [3,2,2,1]`, `limit = 3`  
**Output:** `3`  
**Explanation:** 3 boats (1, 2), (2) and (3)

### Example 3:

**Input:** `people = [3,5,3,4]`, `limit = 5`  
**Output:** `4`  
**Explanation:** 4 boats (3), (3), (4), (5)

### Constraints:

- `1 <= people.length <= 5 * 10^4`
- `1 <= people[i] <= limit <= 3 * 10^4`

## Solution (Golang)

### Approach: Greedy + Two Pointers

To minimize the number of boats, we should try to pair the heaviest person with the lightest person. If they can fit together, we do so; otherwise, the heaviest person must take a boat alone.

```go
import "sort"

func numRescueBoats(people []int, limit int) int {
    sort.Ints(people)
    
    left, right := 0, len(people)-1
    boats := 0
    
    for left <= right {
        // If the lightest and heaviest can fit in one boat
        if people[left] + people[right] <= limit {
            left++
        }
        // In any case, the heaviest person must be on this boat
        right--
        boats++
    }
    
    return boats
}
```

## Explanation

1.  **Sorting**: We first sort the `people` array. This allows us to easily identify the lightest and heaviest individuals.
2.  **Two Pointers**: 
    -   `left` points to the lightest person.
    -   `right` points to the heaviest person.
3.  **Greedy Logic**: 
    -   We always put the heaviest person (`people[right]`) on a boat. 
    -   We then check if the lightest person (`people[left]`) can also fit in that same boat (`people[left] + people[right] <= limit`).
    -   If they can, we increment `left` to "remove" the lightest person from the list.
    -   We decrement `right` to "remove" the heaviest person.
    -   Increment the `boats` count.
4.  **Result**: The process continues until all people are assigned to a boat.

## Complexity

-   **Time Complexity**: $O(n \log n)$, where $n$ is the number of people. Sorting takes $O(n \log n)$ time, and the two-pointer traversal takes $O(n)$ time.
-   **Space Complexity**: $O(\log n)$ or $O(n)$ depending on the sorting algorithm implementation (Go's `sort.Ints` typically uses $O(\log n)$ auxiliary space for recursion).