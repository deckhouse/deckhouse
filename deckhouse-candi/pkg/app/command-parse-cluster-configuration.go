package app

import (
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/pkg/config"
)

func DefineCommandParseClusterConfiguration(kpApp *kingpin.Application, parentCmd *kingpin.CmdClause) *kingpin.CmdClause {
	var parseCmd *kingpin.CmdClause
	if parentCmd == nil {
		parseCmd = kpApp.Command("parse-cluster-configuration", "Parse configuration for bootstrap and konverge.")
	} else {
		parseCmd = parentCmd.Command("parse-cluster-configuration", "Parse configuration for bootstrap and konverge.")
	}

	var ParseInputFile string
	parseCmd.Flag("file", "input file name with yaml documents").
		Short('f').
		StringVar(&ParseInputFile)
	var ParseOutput string
	parseCmd.Flag("output", "output format json or yaml").
		Short('o').
		StringVar(&ParseOutput)
	var ParseIncludeBootstrap bool
	parseCmd.Flag("include-bootstrap", "include bootstrap field in output").
		Short('b').
		BoolVar(&ParseIncludeBootstrap)
	parseCmd.Action(func(c *kingpin.ParseContext) error {
		var err error
		var metaConfig *config.MetaConfig
		// TODO should be fixed in kingpin repo or shell-operator and others should migrate to github.com/alecthomas/kingpin.
		// https://github.com/flant/kingpin/pull/1
		// replace gopkg.in/alecthomas/kingpin.v2 => github.com/flant/kingpin is not working
		// if ParseInputFile == "-" {
		if ParseInputFile == "" {
			data, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read configs from stdin: %v", err)
			}
			metaConfig, err = config.ParseConfigFromData(string(data))
			if err != nil {
				return err
			}
		} else {
			metaConfig, err = config.ParseConfig(ParseInputFile)
			if err != nil {
				return err
			}
		}

		output, err := metaConfig.MarshalConfig(ParseIncludeBootstrap)
		if err != nil {
			return err
		}

		fmt.Println(string(output))
		return nil
	})

	return parseCmd
}
