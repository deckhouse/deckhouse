package calculated

import (
	"upmeter/pkg/checks"
	"upmeter/pkg/util"
)

// worsen merges episode pessimistically
func worsen(to, from *checks.DowntimeEpisode, stepSeconds int64) {
	var (
		total   = util.Min(to.Total(), from.Total(), stepSeconds)
		fail    = limitedMax(total, to.FailSeconds, from.FailSeconds)
		unknown = limitedMax(total-fail, to.UnknownSeconds, from.UnknownSeconds)
		nodata  = limitedMax(total-fail-unknown, to.NoDataSeconds, from.NoDataSeconds)
		success = total - fail - unknown - nodata
	)

	to.FailSeconds = fail
	to.UnknownSeconds = unknown
	to.NoDataSeconds = nodata
	to.SuccessSeconds = success
}

func limitedMax(limit int64, values ...int64) int64 {
	max := util.Max(values...)
	return util.Min(limit, max)
}
