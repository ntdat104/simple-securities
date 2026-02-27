func subarraySum(nums []int, k int) int {
    count := 0
    sum := 0
    prefixSums := make(map[int]int)
    prefixSums[0] = 1 // Base case: a sum of 0 has occurred once
    
    for _, num := range nums {
        sum += num
        if val, ok := prefixSums[sum-k]; ok {
            count += val
        }
        prefixSums[sum]++
    }
    
    return count
}
```

## Explanation

1.  **Prefix Sum**: We keep track of the cumulative sum of elements as we iterate through the array.
2.  **The Formula**: At any index `i`, we have `current_sum`. We want to find if there's an index `j < i` such that `current_sum - prefix_sum[j] = k`. This is equivalent to checking if `prefix_sum[j] = current_sum - k`.
3.  **Hash Map**: We store each `prefix_sum` we encounter in a map, where the key is the sum and the value is the number of times it has occurred.
4.  **Counting**: For each number, we calculate the `current_sum`, then check the map for `current_sum - k`. If it exists, we add its frequency to our `count`.
5.  **Initialization**: We initialize the map with `prefixSums[0] = 1` to handle the case where a subarray starting from index 0 sums exactly to `k`.

## Complexity

-   **Time Complexity**: $O(n)$, where $n$ is the length of the array. we traverse the array once.
-   **Space Complexity**: $O(n)$ in the worst case to store the prefix sums in the map.