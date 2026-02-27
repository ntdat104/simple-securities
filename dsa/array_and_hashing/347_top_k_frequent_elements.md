# 347. Top K Frequent Elements

## Problem Description

Given an integer array `nums` and an integer `k`, return the `k` most frequent elements. You may return the answer in **any order**.

### Example 1:

**Input:** `nums = [1,1,1,2,2,3]`, `k = 2`  
**Output:** `[1,2]`

### Example 2:

**Input:** `nums = [1]`, `k = 1`  
**Output:** `[1]`

### Constraints:

- `1 <= nums.length <= 10^5`
- `-10^4 <= nums[i] <= 10^4`
- `k` is in the range `[1, the number of unique elements in the array]`.
- It is **guaranteed** that the answer is **unique**.

## Solution (Golang)

### Approach: Bucket Sort

```go
func topKFrequent(nums []int, k int) []int {
    countMap := make(map[int]int)
    for _, num := range nums {
        countMap[num]++
    }

    // Creating buckets where index represents the frequency
    buckets := make([][]int, len(nums)+1)
    for num, count := range countMap {
        buckets[count] = append(buckets[count], num)
    }

    res := make([]int, 0, k)
    // Iterate from the highest frequency bucket to the lowest
    for i := len(buckets) - 1; i >= 0 && len(res) < k; i-- {
        if len(buckets[i]) > 0 {
            res = append(res, buckets[i]...)
        }
    }
    
    // In case we collected more than k (though problem guarantees uniqueness)
    if len(res) > k {
        return res[:k]
    }
    return res
}
```

## Explanation

1.  **Count Frequencies**: We use a map `countMap` to store the frequency of each element in the input array.
2.  **Bucket Sort**: Instead of sorting the map by values (which would take $O(n \log n)$), we use a bucket sort approach. We create an array of slices called `buckets` where the index represents the frequency.
3.  **Collect Results**: We iterate through the `buckets` array from the end (highest frequency) to the beginning. We collect the elements until we reach `k` elements.

## Complexity

-   **Time Complexity**: $O(n)$, where $n$ is the length of the input array. We traverse the array to count frequencies, then iterate through the map to fill buckets, and finally traverse buckets.
-   **Space Complexity**: $O(n)$ to store the frequency map and the buckets.
