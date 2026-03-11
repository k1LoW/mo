package main

import "fmt"

func main() {
	nums := []int{1, 2, 3, 4, 5}
	for i, n := range nums {
		if n%2 == 0 {
			fmt.Printf("nums[%d] = %d (even)\n", i, n)
		}
	}
}
