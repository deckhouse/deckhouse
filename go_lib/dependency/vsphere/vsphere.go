// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vsphere

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"reflect"
	"regexp"
	"strings"

	"github.com/spaolacci/murmur3"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/pbm"
	pbmTypes "github.com/vmware/govmomi/pbm/types"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

//go:generate minimock -i Client -o vsphere_mock.go

type Client interface {
	GetZonesDatastores() (*Output, error)
	ListPolicies() ([]StoragePolicy, error)
	RefreshClient() error
}

type client struct {
	client     *govmomi.Client
	restClient *rest.Client

	config *ProviderClusterConfiguration
}

const (
	datastoreTypeDatastore        = "Datastore"
	datastoreTypeDatastoreCluster = "DatastoreCluster"

	slugSeparator = "-"
)

type ProviderClusterConfiguration struct {
	Provider          Provider `json:"provider"`
	Region            string   `json:"region"`
	RegionTagCategory string   `json:"regionTagCategory"`
	ZoneTagCategory   string   `json:"zoneTagCategory"`
}

type Provider struct {
	Server   string `json:"server"`
	Username string `json:"username"`
	Password string `json:"password"`
	Insecure bool   `json:"insecure"`
}

type ZonedDataStore struct {
	Zones         []string `json:"zones"`
	InventoryPath string   `json:"path"`
	Name          string   `json:"name"`
	DatastoreType string   `json:"datastoreType"`
	DatastoreURL  string   `json:"datastoreURL"`
}

type Output struct {
	Datacenter      string           `json:"datacenter"`
	Zones           []string         `json:"zones"`
	ZonedDataStores []ZonedDataStore `json:"datastores"`
}

type StoragePolicy struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

var (
	dnsLabelRegex   = regexp.MustCompile(`^(?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9]*[a-zA-Z0-9])$`)
	dnsLabelMaxSize = 150
)

func NewClient(config *ProviderClusterConfiguration) (Client, error) {
	c, err := createVsphereClient(config)
	if err != nil {
		return nil, err
	}

	r := &client{
		config:     config,
		client:     c.client,
		restClient: c.restClient,
	}

	return r, nil
}

func (v *client) GetZonesDatastores() (*Output, error) {
	var (
		zoneTagCategoryName = v.config.ZoneTagCategory
	)

	dc, err := v.getDCByRegion(context.TODO())
	if err != nil {
		return nil, err
	}

	zonedDataStores, err := v.getDataStoresInDC(context.TODO(), dc)
	if err != nil {
		return nil, err
	}
	if len(zonedDataStores) == 0 {
		panic("no zonedDataStores returned")
	}

	zones, err := v.getZonesInDC(context.TODO(), dc, zoneTagCategoryName)
	if err != nil {
		return nil, err
	}

	output := Output{
		Datacenter:      dc.Name(),
		Zones:           zones,
		ZonedDataStores: zonedDataStores,
	}

	return &output, nil
}

// PolicyIDByName finds a SPBM storage policy by name and returns its ID.
func (v *client) ListPolicies() ([]StoragePolicy, error) {
	pc, err := pbm.NewClient(context.TODO(), v.client.Client)
	if err != nil {
		return nil, err
	}

	ids, err := pc.QueryProfile(
		context.TODO(),
		pbmTypes.PbmProfileResourceType{
			ResourceType: string(pbmTypes.PbmProfileResourceTypeEnumSTORAGE),
		},
		string(pbmTypes.PbmProfileCategoryEnumREQUIREMENT),
	)
	if err != nil {
		return nil, err
	}

	// RetrieveContent returns error if ids are empty.
	if len(ids) == 0 {
		return nil, nil
	}

	profiles, err := pc.RetrieveContent(context.TODO(), ids)
	if err != nil {
		return nil, err
	}

	result := make([]StoragePolicy, 0, len(profiles))
	for _, profile := range profiles {
		base := profile.GetPbmProfile()
		result = append(result, StoragePolicy{
			Name: base.Name,
			ID:   base.ProfileId.UniqueId,
		})
	}

	return result, nil
}

func (v *client) RefreshClient() error {
	c, err := createVsphereClient(v.config)
	if err != nil {
		return err
	}

	v.client = c.client
	v.restClient = c.restClient

	return nil
}

func createVsphereClient(config *ProviderClusterConfiguration) (client, error) {
	var (
		host     = config.Provider.Server
		username = config.Provider.Username
		password = config.Provider.Password
		insecure = config.Provider.Insecure
	)

	parsedURL, err := url.Parse(fmt.Sprintf("https://%s:%s@%s/sdk", url.PathEscape(strings.TrimSpace(username)), url.PathEscape(strings.TrimSpace(password)), url.PathEscape(strings.TrimSpace(host))))
	if err != nil {
		return client{}, err
	}

	vcClient, err := govmomi.NewClient(context.TODO(), parsedURL, insecure)
	if err != nil {
		return client{}, err
	}

	if !vcClient.IsVC() {
		return client{}, errors.New("not connected to vCenter")
	}

	restClient := rest.NewClient(vcClient.Client)
	user := url.UserPassword(username, password)
	if err := restClient.Login(context.TODO(), user); err != nil {
		return client{}, err
	}

	return client{
		client:     vcClient,
		restClient: restClient,
	}, nil
}

func (v *client) getDCByRegion(ctx context.Context) (*object.Datacenter, error) {
	var datacenter *object.Datacenter

	tagsClient := tags.NewManager(v.restClient)

	regionTag, err := tagsClient.GetTagForCategory(ctx, v.config.Region, v.config.RegionTagCategory)
	if err != nil {
		return nil, err
	}

	attachedObjects, _ := tagsClient.ListAttachedObjects(ctx, regionTag.ID)

	var dcRefs []mo.Reference
	for _, ref := range attachedObjects {
		if ref.Reference().Type == "Datacenter" {
			dcRefs = append(dcRefs, ref)
		}
	}
	if len(dcRefs) != 1 {
		return nil, fmt.Errorf("only one DC should match \"region\" tag, insted matched: %v", dcRefs)
	}

	finder := find.NewFinder(v.client.Client)

	dcRef, err := finder.ObjectReference(ctx, dcRefs[0].Reference())
	if err != nil {
		return nil, err
	}

	datacenter = dcRef.(*object.Datacenter)

	return datacenter, nil
}

func (v *client) getZonesInDC(ctx context.Context, datacenter *object.Datacenter, zoneTagCategoryName string) ([]string, error) {
	finder := find.NewFinder(v.client.Client, true)

	clusters, err := finder.ClusterComputeResourceList(ctx, path.Join(datacenter.InventoryPath, "..."))
	if err != nil {
		return nil, err
	}
	clusterReferences := make([]mo.Reference, len(clusters))
	for i := range clusters {
		clusterReferences[i] = clusters[i]
	}

	tagsClient := tags.NewManager(v.restClient)

	zoneTagCategory, err := tagsClient.GetCategory(ctx, zoneTagCategoryName)
	if err != nil {
		return nil, err
	}

	tagsInCategory, _ := tagsClient.ListTagsForCategory(ctx, zoneTagCategory.ID)

	tagsInCategoryMap := make(map[string]struct{})
	for _, tagID := range tagsInCategory {
		tag, err := tagsClient.GetTag(ctx, tagID)
		if err != nil {
			return nil, err
		}
		tagsInCategoryMap[tag.Name] = struct{}{}
	}

	clustersWithTags, err := tagsClient.GetAttachedTagsOnObjects(ctx, clusterReferences)
	if err != nil {
		return nil, err
	}

	var matchingZonesMap = make(map[string]struct{})
	for _, clusterTags := range clustersWithTags {
		for _, clusterTag := range clusterTags.Tags {
			if _, ok := tagsInCategoryMap[clusterTag.Name]; ok {
				matchingZonesMap[clusterTag.Name] = struct{}{}
			}
		}
	}

	matchingZones := make([]string, 0, len(matchingZonesMap))
	for zone := range matchingZonesMap {
		matchingZones = append(matchingZones, zone)
	}

	if len(matchingZones) == 0 {
		return nil, errors.New("no matching zones found")
	}

	return matchingZones, nil
}

func (v *client) getDataStoresInDC(ctx context.Context, datacenter *object.Datacenter) ([]ZonedDataStore, error) {
	finder := find.NewFinder(v.client.Client, true)

	datastores, dsNotFoundErr := finder.DatastoreList(ctx, path.Join(datacenter.InventoryPath, "..."))

	datastoreClusters, dscNotFoundErr := finder.DatastoreClusterList(ctx, path.Join(datacenter.InventoryPath, "..."))

	if dsNotFoundErr != nil && dscNotFoundErr != nil {
		return nil, fmt.Errorf("not a single Datastore or DatastoreCluster found in the cluster:\n%s\n%s", dsNotFoundErr, dscNotFoundErr)
	}

	datastoreReferences := make([]mo.Reference, 0, len(datastores))
	for _, ds := range datastores {
		datastoreReferences = append(datastoreReferences, ds)
	}

	for _, dsc := range datastoreClusters {
		datastoreReferences = append(datastoreReferences, dsc)
	}

	var datastoreMo []mo.Datastore
	pc := property.DefaultCollector(v.client.Client)
	props := []string{"info", "summary"}

	if len(datastores) > 0 {
		datastoreRefs := make([]types.ManagedObjectReference, 0, len(datastores))
		for _, ds := range datastores {
			datastoreRefs = append(datastoreRefs, ds.Reference())
		}
		if err := pc.Retrieve(ctx, datastoreRefs, props, &datastoreMo); err != nil {
			return nil, fmt.Errorf("can't retrieve properties of datastores:\n%s", err)
		}
	}

	datastoreMoByRef := make(map[types.ManagedObjectReference]mo.Datastore, len(datastoreMo))
	for _, o := range datastoreMo {
		datastoreMoByRef[o.Reference()] = o
	}

	tagsClient := tags.NewManager(v.restClient)

	zoneTagCategory, err := tagsClient.GetCategory(ctx, v.config.ZoneTagCategory)
	if err != nil {
		return nil, err
	}

	datastoresWithTags, err := tagsClient.GetAttachedTagsOnObjects(ctx, datastoreReferences)
	if err != nil {
		return nil, err
	}

	zds := make([]ZonedDataStore, 0)
	for _, attachedTags := range datastoresWithTags {
		var dsZones []string
		for _, tag := range attachedTags.Tags {
			if tag.CategoryID == zoneTagCategory.ID {
				dsZones = append(dsZones, tag.Name)
			}
		}
		if len(dsZones) == 0 {
			continue
		}

		dsObject, err := finder.ObjectReference(ctx, attachedTags.ObjectID.Reference())
		if err != nil {
			return nil, err
		}

		var (
			datastoreType string
			inventoryPath string
			datastoreURL  string
		)
		switch obj := dsObject.(type) {
		case *object.Datastore:
			datastoreType = datastoreTypeDatastore
			inventoryPath = obj.InventoryPath
			ds := datastoreMoByRef[obj.Reference()]
			datastoreURL = ds.Summary.Url
		case *object.StoragePod:
			datastoreType = datastoreTypeDatastoreCluster
			inventoryPath = obj.InventoryPath
		default:
			return nil, fmt.Errorf("'%s' is not a Datastore nor a DatastoreCluster", reflect.TypeOf(dsObject))
		}

		zds = append(zds, ZonedDataStore{
			Zones:         dsZones,
			InventoryPath: inventoryPath,
			Name:          slugKubernetesName(strings.Join(strings.Split(inventoryPath, "/")[3:], "-")),
			DatastoreType: datastoreType,
			DatastoreURL:  datastoreURL,
		})
	}

	return zds, nil
}

func slugKubernetesName(data string) string {
	if !shouldNotBeSlugged(data, dnsLabelRegex, dnsLabelMaxSize) {
		return slug(data, dnsLabelMaxSize)
	}

	return data
}

func shouldNotBeSlugged(data string, regexp *regexp.Regexp, maxSize int) bool {
	return len(data) == 0 || regexp.Match([]byte(data)) && len(data) < maxSize
}

func slug(data string, maxSize int) string {
	sluggedData := slugify(data)
	murmurHash := murmurHash(data)

	var slugParts []string
	if sluggedData != "" {
		croppedSluggedData := cropSluggedData(sluggedData, murmurHash, maxSize)
		if strings.HasPrefix(croppedSluggedData, "-") {
			slugParts = append(slugParts, croppedSluggedData[:len(croppedSluggedData)-1])
		} else {
			slugParts = append(slugParts, croppedSluggedData)
		}
	}
	slugParts = append(slugParts, murmurHash)

	consistentUniqSlug := strings.Join(slugParts, slugSeparator)

	return consistentUniqSlug
}

func cropSluggedData(data string, hash string, maxSize int) string {
	var index int
	maxLength := maxSize - len(hash) - len(slugSeparator)
	if len(data) > maxLength {
		index = maxLength
	} else {
		index = len(data)
	}

	return data[:index]
}

func slugify(data string) string {
	var result []rune

	var isCursorDash bool
	var isPreviousDash bool
	var isStartedDash, isDoubledDash bool

	isResultEmpty := true
	for _, r := range data {
		cursor := algorithm(string(r))
		if cursor == "" {
			continue
		}

		isCursorDash = cursor == "-"
		isStartedDash = isCursorDash && isResultEmpty
		isDoubledDash = isCursorDash && !isResultEmpty && isPreviousDash

		if isStartedDash || isDoubledDash {
			continue
		}

		result = append(result, []rune(cursor)...)
		isPreviousDash = isCursorDash
		isResultEmpty = false
	}

	isEndedDash := !isResultEmpty && isCursorDash
	if isEndedDash {
		return string(result[:len(result)-1])
	}
	return string(result)
}

func algorithm(data string) string {
	var result string
	for ind := range data {
		char, ok := mapping[string([]rune(data)[ind])]
		if ok {
			result += char
		}
	}

	return result
}

func murmurHash(args ...string) string {
	h32 := murmur3.New32()
	h32.Write([]byte(prepareHashArgs(args...)))
	sum := h32.Sum32()
	return fmt.Sprintf("%x", sum)
}

func prepareHashArgs(args ...string) string {
	return strings.Join(args, ":::")
}
