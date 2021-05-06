package helpers

import (
	"errors"

	sh_app "github.com/flant/shell-operator/pkg/app"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers/aws"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers/d8crypto"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers/fnv"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers/openstack"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers/unit"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers/vsphere"
	dhctlapp "github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands"
)

func DefineHelperCommands(kpApp *kingpin.Application) {
	helpersCommand := sh_app.CommandWithDefaultUsageTemplate(kpApp, "helper", "Deckhouse helpers.")
	{
		fnvCommand := helpersCommand.Command("fnv", "Section for command related tp fnv encoding and decoding.")
		fnvEncodeCommand := fnvCommand.Command("encode", "Encode input in FNV styled string.")
		fnvEncodeInput := fnvEncodeCommand.Arg("input", "String to encode").Required().String()
		fnvEncodeCommand.Action(func(c *kingpin.ParseContext) error {
			return fnv.Encode(*fnvEncodeInput)
		})
	}

	{
		unitCommand := helpersCommand.Command("unit", "Unit related methods.")
		unitConvertCommand := unitCommand.Command("convert", "Convert units.")
		unitConvertMode := unitConvertCommand.Flag("mode", "Mode of unit converter").PlaceHolder("duration | kube-resource-unit").Enum("duration", "kube-resource-unit")
		unitConvertOutput := unitConvertCommand.Flag("output", "Output of unit converter").PlaceHolder("value | milli").Default("value").Enum("value", "milli")
		unitConvertCommand.Action(func(c *kingpin.ParseContext) error {
			return unit.Convert(*unitConvertMode, *unitConvertOutput)
		})
	}

	{
		awsCommand := helpersCommand.Command("aws", "AWS helpers.")
		awsMapZoneToSubnetsCommand := awsCommand.Command("map-zone-to-subnets", "Map zones to subnets.")
		awsMapZoneToSubnetsCommand.Action(func(c *kingpin.ParseContext) error {
			return aws.MapZoneToSubnets()
		})
	}

	{
		openstackCommand := helpersCommand.Command("openstack", "OpenStack helpers.")
		openstackGetVolumeTypes := openstackCommand.Command("get-volume-types", "Get volume types.")
		openstackGetVolumeTypes.Action(func(c *kingpin.ParseContext) error {
			return openstack.GetVolumeTypes()
		})
		supportsOnlineDiskResize := openstackCommand.Command("supports-online-disk-resize", "Check whether block-storage API support online resize.")
		supportsOnlineDiskResize.Action(func(c *kingpin.ParseContext) error {
			return openstack.SupportsOnlineDiskResize()
		})
	}

	{
		vsphereCommand := helpersCommand.Command("vsphere", "VSphere helpers.")
		vsphereGetZonesDatastores := vsphereCommand.Command("get-zones-datastores", "Get zones datastores.")
		vsphereGetZonesDatastores.Action(func(c *kingpin.ParseContext) error {
			return vsphere.GetZonesDatastores()
		})
	}

	{
		etcdCommand := helpersCommand.Command("etcd", "etcd helpers.")
		etcdCommand.Action(func(c *kingpin.ParseContext) error {
			return errors.New("helper etcd move-service is deprecated")
		})
	}

	{
		cryptoCommand := helpersCommand.Command("crypto", "RSA crypto helpers.")

		cryptoGenKeypairCommand := cryptoCommand.Command("gen-keypair", "Generate ED25519 keypair.")
		cryptoGenKeypairCommand.Action(func(c *kingpin.ParseContext) error {
			return d8crypto.GenKeypair()
		})

		cryptoGenJWTCommand := cryptoCommand.Command("gen-jwt", "Generate JWT token.")
		privateKeyPath := cryptoGenJWTCommand.Flag("private-key-path", "Path to private RSA key in PEM format.").Required().ExistingFile()
		claims := cryptoGenJWTCommand.Flag("claim", "Claims for token (ex --claim iss=deckhouse --claim sub=akakiy).").Required().StringMap()
		ttl := cryptoGenJWTCommand.Flag("ttl", "TTL duration (ex. 10s).").Required().Duration()
		cryptoGenJWTCommand.Action(func(c *kingpin.ParseContext) error {
			return d8crypto.GenJWT(*privateKeyPath, *claims, *ttl)
		})
	}

	// dhctl parser for ClusterConfiguration and <Provider-name>ClusterConfiguration secrets
	dhctlapp.DefineCommandParseClusterConfiguration(kpApp, helpersCommand)
	dhctlapp.DefineCommandParseCloudDiscoveryData(kpApp, helpersCommand)
}
