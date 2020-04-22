package helpers

import (
	"gopkg.in/alecthomas/kingpin.v2"

	candiapp "flant/deckhouse-candi/pkg/app"
	sh_app "github.com/flant/shell-operator/pkg/app"

	"flant/deckhouse-controller/pkg/helpers/aws"
	"flant/deckhouse-controller/pkg/helpers/fnv"
	"flant/deckhouse-controller/pkg/helpers/helm"
	"flant/deckhouse-controller/pkg/helpers/openstack"
	"flant/deckhouse-controller/pkg/helpers/unit"
	"flant/deckhouse-controller/pkg/helpers/vsphere"
)

func DefineHelperCommands(kpApp *kingpin.Application) {
	helpersCommand := sh_app.CommandWithDefaultUsageTemplate(kpApp, "helper", "Deckhouse helpers.")

	fnvCommand := helpersCommand.Command("fnv", "Section for command related tp fnv encoding and decoding.")
	fnvEncodeCommand := fnvCommand.Command("encode", "Encode input in FNV styled string.")
	fnvEncodeInput := fnvEncodeCommand.Arg("input", "String to encode").Required().String()
	fnvEncodeCommand.Action(func(c *kingpin.ParseContext) error {
		return fnv.Encode(*fnvEncodeInput)
	})

	unitCommand := helpersCommand.Command("unit", "Unit related methods.")
	unitConvertCommand := unitCommand.Command("convert", "Convert units.")
	unitConvertMode := unitConvertCommand.Flag("mode", "Mode of unit converter").Enum("duration", "kube-resource-unit")
	unitConvertCommand.Action(func(c *kingpin.ParseContext) error {
		return unit.Convert(*unitConvertMode)
	})

	awsCommand := helpersCommand.Command("aws", "AWS helpers.")
	awsMapZoneToSubnetsCommand := awsCommand.Command("map-zone-to-subnets", "Map zones to subnets.")
	awsMapZoneToSubnetsCommand.Action(func(c *kingpin.ParseContext) error {
		return aws.MapZoneToSubnets()
	})

	openstackCommand := helpersCommand.Command("openstack", "OpenStack helpers.")
	openstackGetVolumeTypes := openstackCommand.Command("get-volume-types", "Get volume types.")
	openstackGetVolumeTypes.Action(func(c *kingpin.ParseContext) error {
		return openstack.GetVolumeTypes()
	})

	vsphereCommand := helpersCommand.Command("vsphere", "VSphere helpers.")
	vsphereGetZonesDatastores := vsphereCommand.Command("get-zones-datastores", "Get zones datastores.")
	vsphereGetZonesDatastores.Action(func(c *kingpin.ParseContext) error {
		return vsphere.GetZonesDatastores()
	})

	helmCommand := helpersCommand.Command("helm", "Helm helpers.")
	helmReleaseRenameCommand := helmCommand.Command("set-release-name", "Update release name in stored structure.")
	helmReleaseRenameInput := helmReleaseRenameCommand.Arg("input", "String").Required().String()
	helmReleaseRenameCommand.Action(func(c *kingpin.ParseContext) error {
		return helm.ReleaseRename(*helmReleaseRenameInput)
	})

	// deckhouse-candi parser for ClusterConfiguration and <Provider-name>ClusterConfiguration secrets
	candiapp.DefineCommandParseClusterConfiguration(kpApp, helpersCommand)
}
