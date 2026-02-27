func maxProfit(prices []int) int {
	totalProfit := 0
	for i := 1; i < len(prices); i++ {
		if prices[i] > prices[i-1] {
			totalProfit += prices[i] - prices[i-1]
		}
	}
	return totalProfit
}
```

## Explanation

1.  **Iterate through prices**: We start from the second day (index `1`).
2.  **Greedy Profit**: If the price on the current day is higher than the price on the previous day, we "buy" on the previous day and "sell" on the current day.
3.  **Accumulation**: By adding up all positive daily gains, we capture every upward movement in the stock price. This mathematically results in the maximum profit possible across any number of transactions.
4.  **No Downside**: If the price drops, we simply don't trade on that day (the `if` condition is not met), ensuring we never subtract from our profit.

## Complexity

-   **Time Complexity**: $O(n)$, where $n$ is the number of days (length of `prices`). We traverse the array exactly once.
-   **Space Complexity**: $O(1)$, as we only use a single variable `totalProfit` to keep track of the result.