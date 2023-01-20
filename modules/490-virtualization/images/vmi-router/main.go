/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"vmi-router/controllers"

	"github.com/boltdb/bolt"
	"github.com/vishvananda/netlink"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	virtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme = runtime.NewScheme()
	log    = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(virtv1.AddToScheme(scheme))
}

type cidrFlag []string

func (f *cidrFlag) String() string { return "" }
func (f *cidrFlag) Set(s string) error {
	*f = append(*f, s)
	return nil
}

func main() {
	var cidrs cidrFlag
	var hostIfaceName string
	var dbFile string
	var dryRun bool
	var routeLocal bool
	var metricsAddr string
	var probeAddr string
	flag.Var(&cidrs, "cidr", "CIDRs enabled to route (multiple flags allowed)")
	flag.StringVar(&dbFile, "db", "routes.db", "Path to database of local routes.")
	flag.BoolVar(&dryRun, "dry-run", false, "Don't perform any changes on the node.")
	flag.BoolVar(&routeLocal, "route-local", false, "Route all CIDRs via local interface (for tunneling mode).")
	flag.StringVar(&hostIfaceName, "host-iface", "cilium_host", "Name of local CNI interface.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	var parsedCIDRs []*net.IPNet
	for _, cidr := range cidrs {
		_, parsedCIDR, err := net.ParseCIDR(cidr)
		if err != nil || parsedCIDR == nil {
			fmt.Println(err, "failed to parse CIDR")
			os.Exit(1)
		}
		parsedCIDRs = append(parsedCIDRs, parsedCIDR)
	}

	log.Info(fmt.Sprintf("managed CIDRs: %+v", cidrs))

	db, err := initDB(dbFile)
	if err != nil {
		log.Error(err, "failed to init database")
		os.Exit(1)
	}
	defer db.Close()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
	})
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	clientSet, err := kubecli.GetKubevirtClientFromRESTConfig(mgr.GetConfig())
	if err != nil {
		log.Error(err, "unable to create clientset")
		os.Exit(1)
	}
	controller := controllers.VMIRouterController{
		NodeName:   os.Getenv("NODE_NAME"),
		DB:         db,
		RESTClient: clientSet.RestClient(),
		Client:     mgr.GetClient(),
		RouteLocal: routeLocal,
		CIDRs:      parsedCIDRs,
		RouteAdd:   netlink.RouteAdd,
		RouteDel:   netlink.RouteDel,
	}
	if dryRun {
		controller.RouteAdd = func(*netlink.Route) error { return nil }
		controller.RouteDel = func(*netlink.Route) error { return nil }
	} else {
		if controller.NodeName == "" {
			log.Error(fmt.Errorf(""), "Required NODE_NAME env variable is not specified!")
			os.Exit(1)
		}
		log.Info("my node name: " + controller.NodeName)
		hostIface, err := netlink.LinkByName(hostIfaceName)
		if err != nil {
			log.Error(err, "failed to get interface")
			os.Exit(1)
		}
		controller.HostIfaceIndex = hostIface.Attrs().Index
	}

	if err := mgr.Add(controller); err != nil {
		log.Error(err, "unable to add vmi router controller to manager")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func initDB(dbpath string) (*bolt.DB, error) {
	// Open the my.db data file in your current directory.
	// It will be created if it doesn't exist.
	db, err := bolt.Open(dbpath, 0600, nil)
	if err != nil {
		return db, fmt.Errorf("open database: %s", err)
	}

	db.Update(func(tx *bolt.Tx) error {
		for _, bucketName := range []string{controllers.VMIRoutesBucket, controllers.CIDRRoutesBucket} {
			_, err := tx.CreateBucket([]byte(bucketName))
			if err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}
		}
		return nil
	})
	return db, nil
}
