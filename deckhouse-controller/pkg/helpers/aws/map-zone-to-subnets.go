package aws

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	"flant/deckhouse-controller/pkg/helpers/utils"
)

type ZonesToSubnetIdMap map[string]string

func MapZoneToSubnets() error {
	sess := session.Must(session.NewSession())
	ec2Svc := ec2.New(sess)

	clusterID, err := utils.GetEnvOrDie("CLUSTER_ID")
	if err != nil {
		return err
	}

	availZones, err := ec2Svc.DescribeAvailabilityZones(nil)
	if err != nil || availZones == nil {
		return fmt.Errorf("list of availability zones is empty, or an error was returned: %v", err)
	}

	var zonesToSubnetMap = make(ZonesToSubnetIdMap)
	for _, az := range availZones.AvailabilityZones {
		subnets, err := ec2Svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("availability-zone-id"),
					Values: []*string{az.ZoneId},
				},
				{
					Name:   aws.String("tag:kubernetes.io/cluster/" + clusterID),
					Values: []*string{aws.String("*")},
				},
			}})
		if err != nil || subnets.Subnets == nil {
			return fmt.Errorf("list of availability zones is empty, or an error was returned: %v", err)
		}

		zonesToSubnetMap[*az.ZoneName] = *subnets.Subnets[0].SubnetId
	}

	marshalledMapping, err := json.Marshal(zonesToSubnetMap)
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write(marshalledMapping)
	if err != nil {
		return err
	}

	return nil
}
