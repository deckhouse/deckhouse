package log

import (
	"github.com/fatih/color"
	"github.com/flant/logboek"
)

func MainProcessOptions() logboek.LogProcessOptions {
	return logboek.LogProcessOptions{
		LevelLogProcessOptions: logboek.LevelLogProcessOptions{
			Style: &logboek.Style{
				Attributes: []color.Attribute{color.FgYellow, color.Bold},
			},
		},
	}
}

func TaskOptions() logboek.LogProcessOptions {
	return logboek.LogProcessOptions{
		LevelLogProcessOptions: logboek.LevelLogProcessOptions{
			Style: &logboek.Style{
				Attributes: []color.Attribute{color.FgHiBlue, color.Bold},
			},
		},
	}
}

func BoldOptions() logboek.LogProcessOptions {
	return logboek.LogProcessOptions{
		LevelLogProcessOptions: logboek.LevelLogProcessOptions{
			Style: &logboek.Style{
				Attributes: []color.Attribute{color.FgHiWhite, color.Bold},
			},
		},
	}
}
