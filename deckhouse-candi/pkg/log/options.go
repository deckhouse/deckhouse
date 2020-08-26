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
				Attributes: []color.Attribute{color.FgBlue, color.Bold},
			},
		},
	}
}

func boldStyle() *logboek.Style {
	return &logboek.Style{Attributes: []color.Attribute{color.FgHiWhite, color.Bold}}
}

func BoldOptions() logboek.LogProcessOptions {
	return logboek.LogProcessOptions{LevelLogProcessOptions: logboek.LevelLogProcessOptions{Style: boldStyle()}}
}

// TODO: cook loogboek
func BoldStartOptions() logboek.LogProcessStartOptions {
	return logboek.LogProcessStartOptions{LevelLogProcessStartOptions: logboek.LevelLogProcessStartOptions{Style: boldStyle()}}
}

func BoldEndOptions() logboek.LogProcessEndOptions {
	return logboek.LogProcessEndOptions{LevelLogProcessEndOptions: logboek.LevelLogProcessEndOptions{Style: boldStyle()}}
}

func BoldFailOptions() logboek.LogProcessFailOptions {
	return logboek.LogProcessFailOptions{LevelLogProcessFailOptions: logboek.LevelLogProcessFailOptions{LevelLogProcessEndOptions: logboek.LevelLogProcessEndOptions{Style: boldStyle()}}}
}

func terraformStyle() *logboek.Style {
	return &logboek.Style{
		Attributes: []color.Attribute{color.FgGreen, color.Bold},
	}
}

func TerraformOptions() logboek.LogProcessOptions {
	return logboek.LogProcessOptions{LevelLogProcessOptions: logboek.LevelLogProcessOptions{
		Style: terraformStyle(),
	}}
}

func ConvergeOptions() logboek.LogProcessOptions {
	return logboek.LogProcessOptions{
		LevelLogProcessOptions: logboek.LevelLogProcessOptions{
			Style: &logboek.Style{
				Attributes: []color.Attribute{color.FgHiCyan, color.Bold},
			},
		},
	}
}

func TerraformStartOptions() logboek.LogProcessStartOptions {
	return logboek.LogProcessStartOptions{LevelLogProcessStartOptions: logboek.LevelLogProcessStartOptions{Style: terraformStyle()}}
}

func TerraformTitle(message string) {
	logboek.LogProcessStart(message, TerraformStartOptions())
}
