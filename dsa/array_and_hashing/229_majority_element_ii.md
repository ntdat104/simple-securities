# 229. Majority Element II

## Problem Description

Given an integer array of size `n`, find all elements that appear more than `⌊ n/3 ⌋` times.

### Example 1:

**Input:** `nums = [3,2,3]`  
**Output:** `[3]`

### Example 2:

**Input:** `nums = [1]`  
**Output:** `[1]`

### Example 3:

**Input:** `nums = [1,2]`  
**Output:** `[1,2]`

### Constraints:

- `1 <= nums.length <= 5 * 10^4`
- `-10^9 <= nums[i] <= 10^9`

## Solution (Golang)

### Approach: Boyer-Moore Voting Algorithm (Boyer-Moore Variation)

To find elements that appear more than `n/k` times, there can be at most `k-1` such elements. For `n/3`, there can be at most `2` such elements. We can maintain two candidates and two counters.

```go
func majorityElement(nums []int) []int {
    if len(nums) == 0 {
        return []int{}
    }

    // Step 1: Find potential candidates
    cand1, cand2 := 0, 1 // Initialize with different values
    count1, count2 := 0, 0

    for _, num := range nums {
        if num == cand1 {
            count1++
        } else if num == cand2 {
            count2++
        } else if count1 == 0 {
            cand1 = num
            count1 = 1
        } else if count2 == 0 {
            cand2 = num
            count2 = 1
        } else {
            count1--
            count2--
        }
    }

    // Step 2: Verify candidates
    count1, count2 = 0, 0
    for _, num := range nums {
        if num == cand1 {
            count1++
        } else if num == cand2 {
            count2++
        }
    }

    var result []int
    n := len(nums)
    if count1 > n/3 {
        result = append(result, cand1)
    }
    if count2 > n/3 {
        result = append(result, cand2)
    }

    return result
}
```

## Explanation

1.  **Candidates**: Since we are looking for elements that appear more than `1/3` of the time, there can be at most two such elements. We maintain two potential candidates (`cand1`, `cand2`) and their respective counters (`count1`, `count2`).
2.  **Voting Loop**:
    - If the current number matches a candidate, increment its counter.
    - If a counter is 0, the current number becomes the new candidate for that slot.
    - If it matches neither and both counters are non-zero, "deduct" a vote from both candidates.
3.  **Verification**: The Boyer-Moore algorithm only guarantees that *if* there are majority elements, they will be among the candidates. It doesn't guarantee the candidates *are* majority elements. Thus, we must perform a second pass to count their actual occurrences.

## Complexity

-   **Time Complexity**: $O(n)$, where $n$ is the length of the array. We make two passes over the input.
-   **Space Complexity**: $O(1)$, as we only use a fixed number of variables regardless of the input size.
