package util

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func Test_DoWithTimer(t *testing.T) {
	// Test is manual because it is time based
	t.SkipNow()

	DoWithTimer(2*time.Second, func() {
		fmt.Printf("job start\n")
		time.Sleep(time.Second)
		fmt.Printf("job end\n")
	},
		func() {
			fmt.Printf("job is timed out\n")
		})

	DoWithTimer(time.Second, func() {
		fmt.Printf("job start\n")
		time.Sleep(2 * time.Second)
		fmt.Printf("job end\n")

	},
		func() {
			fmt.Printf("job is timed out\n")
		})
}

func Test_SequentialDoWithTimer(t *testing.T) {
	// Test is manual because it is time based
	t.SkipNow()

	items := []string{
		"1-1odin",
		"2-2dvav",
		"3-triri",
		"4-fourr",
		"5-fivev"}

	fmt.Printf("Success, with timeouts\n")
	SequentialDoWithTimer(
		context.Background(),
		time.Second,
		items,
		func(ctx context.Context, idx int, item string) int {
			prefix := strings.Repeat("  ", idx)
			fmt.Printf(prefix+"%s Start\n", item)

			t := time.NewTicker(500 * time.Millisecond)
			defer func() {
				//fmt.Printf(prefix+"%s job stop timer\n", item)
				t.Stop()
				//fmt.Printf(prefix+"%s job stop\n", item)
			}()
			for {
				select {
				case <-ctx.Done():
					fmt.Printf(prefix+"%s job canceled\n", item)
					return 0
				case <-t.C:
					if item == "4-fourr" {
						fmt.Printf(prefix+"%s job SUCCESS\n", item)
						return 1
					} else {
						fmt.Printf(prefix+"%s job\n", item)
					}
				}
			}
		},
		func(idx int, item string) {
			prefix := strings.Repeat("  ", idx)
			fmt.Printf(prefix+"%s Timed out\n", item)
		},
	)

	// 0 result, no timeouts
	fmt.Printf("\nAll fail, no timeouts\n")
	SequentialDoWithTimer(
		context.Background(),
		time.Second,
		items,
		func(ctx context.Context, idx int, item string) int {
			prefix := strings.Repeat("  ", idx)
			fmt.Printf(prefix+"%s Start\n", item)

			t := time.NewTimer(500 * time.Millisecond)
			defer func() {
				//fmt.Printf(prefix+"%s job stop timer\n", item)
				t.Stop()
				//fmt.Printf(prefix+"%s job stop\n", item)
			}()
			for {
				select {
				case <-ctx.Done():
					fmt.Printf(prefix+"%s job canceled\n", item)
					return 0
				case <-t.C:
					fmt.Printf(prefix+"%s job FAIL\n", item)
					return 0
				}
			}
		},
		func(idx int, item string) {
			prefix := strings.Repeat("  ", idx)
			fmt.Printf(prefix+"%s Timed out\n", item)
		},
	)

	fmt.Printf("\nSuccess, no timeouts\n")
	SequentialDoWithTimer(
		context.Background(),
		time.Second,
		items,
		func(ctx context.Context, idx int, item string) int {
			prefix := strings.Repeat("  ", idx)
			fmt.Printf(prefix+"%s Start\n", item)

			t := time.NewTimer(500 * time.Millisecond)
			defer func() {
				//fmt.Printf(prefix+"%s job stop timer\n", item)
				t.Stop()
				//fmt.Printf(prefix+"%s job stop\n", item)
			}()
			for {
				select {
				case <-ctx.Done():
					fmt.Printf(prefix+"%s job canceled\n", item)
					return 0
				case <-t.C:
					if item == "4-fourr" {
						fmt.Printf(prefix+"%s job SUCCESS\n", item)
						return 1
					} else {
						fmt.Printf(prefix+"%s job FAIL\n", item)
						return 0
					}
				}
			}
		},
		func(idx int, item string) {
			prefix := strings.Repeat("  ", idx)
			fmt.Printf(prefix+"%s Timed out\n", item)
		},
	)

}
