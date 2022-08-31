package update

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNextAllowedWindow(t *testing.T) {
	t.Run("min time is inside the window", func(t *testing.T) {
		ws := Windows{
			{
				From: "16:00",
				To:   "18:00",
				Days: []string{"wed"},
			},
		}
		// wedndesday 16:35
		min := time.Date(2021, 10, 13, 16, 35, 00, 0, time.UTC)

		res := ws.NextAllowedTime(min)
		assert.Equal(t, min, res)
	})

	t.Run("min time is before the window", func(t *testing.T) {
		ws := Windows{
			{
				From: "16:00",
				To:   "18:00",
				Days: []string{"wed"},
			},
		}
		// tuesday 16:35
		min := time.Date(2021, 10, 12, 16, 35, 00, 0, time.UTC)

		res := ws.NextAllowedTime(min)
		// beginning of the window: wednesday 16:00
		assert.Equal(t, time.Date(2021, 10, 13, 16, 00, 00, 0, time.UTC), res)
		assert.Equal(t, time.Wednesday, res.Weekday())
	})

	t.Run("min time between two windows", func(t *testing.T) {
		ws := Windows{
			{
				From: "16:00",
				To:   "18:00",
				Days: []string{"wed"},
			},

			{
				From: "20:00",
				To:   "22:00",
				Days: []string{"sat"},
			},
		}
		// wednesday 19:35
		min := time.Date(2021, 10, 13, 19, 35, 00, 0, time.UTC)

		res := ws.NextAllowedTime(min)
		// beginning of the window: saturday 20:00
		assert.Equal(t, time.Date(2021, 10, 16, 20, 00, 00, 0, time.UTC), res)
		assert.Equal(t, time.Saturday, res.Weekday())
	})

	t.Run("min time after single window", func(t *testing.T) {
		ws := Windows{
			{
				From: "16:00",
				To:   "18:00",
				Days: []string{"wed"},
			},
		}
		// wednesday 18:01
		min := time.Date(2021, 10, 13, 18, 01, 00, 0, time.UTC)

		res := ws.NextAllowedTime(min)
		// move to the one week: wednesday 16:00
		assert.Equal(t, time.Date(2021, 10, 20, 16, 00, 00, 0, time.UTC), res)
		assert.Equal(t, time.Wednesday, res.Weekday())
	})

	t.Run("min time after all windows", func(t *testing.T) {
		ws := Windows{
			{
				From: "20:00",
				To:   "22:00",
				Days: []string{"sat"},
			},
			{
				From: "16:00",
				To:   "18:00",
				Days: []string{"wed"},
			},
		}
		// sunday 19:35
		min := time.Date(2021, 10, 17, 19, 35, 00, 0, time.UTC)

		res := ws.NextAllowedTime(min)
		// beginning of the window: wednesday 16:00
		assert.Equal(t, time.Date(2021, 10, 20, 16, 00, 00, 0, time.UTC), res)
		assert.Equal(t, time.Wednesday, res.Weekday())
	})
}
