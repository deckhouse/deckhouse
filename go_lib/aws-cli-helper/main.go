package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type ZonesToSubnetIdMap map[string]string

func main() {
	sess := session.Must(session.NewSession())
	ec2Svc := ec2.New(sess)

	clusterID := getEnvOrDie("CLUSTER_ID")

	availZones, err := ec2Svc.DescribeAvailabilityZones(nil)
	if err != nil || availZones == nil {
		panic(fmt.Sprintf("List of availability zones is empty or an error was returned: %v", err))
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
			panic(fmt.Sprintf("List of availability zones is empty or an error was returned: %v", err))
		}

		zonesToSubnetMap[*az.ZoneName] = *subnets.Subnets[0].SubnetId

	}

	marshalledMapping, err := json.Marshal(zonesToSubnetMap)
	if err != nil {
		panic(err)
	}

	_, err = os.Stdout.Write(marshalledMapping)
	if err != nil {
		panic(err)
	}
}

func getEnvOrDie(envName string) string {
	if value, ok := os.LookupEnv(envName); !ok {
		panic(fmt.Sprintf("env \"%s\" is not defined", envName))
	} else {
		return value
	}
}
