package vsphere

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/spaolacci/murmur3"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/mo"

	"flant/deckhouse-controller/pkg/helpers/utils"
)

const (
	datastoreTypeDatastore        = "Datastore"
	datastoreTypeDatastoreCluster = "DatastoreCluster"

	slugSeparator = "-"
)

type vsphereClient struct {
	client     *govmomi.Client
	restClient *rest.Client

	host     string
	username string
	password string
	insecure bool
}

type ZonedDataStore struct {
	Zones         []string `json:"zones"`
	InventoryPath string   `json:"path"`
	Name          string   `json:"name"`
	DatastoreType string   `json:"datastoreType"`
}

type Output struct {
	Datacenter      string           `json:"datacenter"`
	Zones           []string         `json:"zones"`
	ZonedDataStores []ZonedDataStore `json:"datastores"`
}

var (
	dnsLabelRegex   = regexp.MustCompile(`^(?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9])$`)
	dnsLabelMaxSize = 150
)

func GetZonesDatastores() error {
	c, err := createVsphereClient()
	if err != nil {
		return err
	}

	regionTagName, err := utils.GetEnvOrDie("VSPHERE_REGION_TAG_NAME")
	if err != nil {
		return err
	}
	regionTagCategoryName, err := utils.GetEnvOrDie("VSPHERE_REGION_TAG_CATEGORY_NAME")
	if err != nil {
		return err
	}
	zoneTagCategoryName, err := utils.GetEnvOrDie("VSPHERE_ZONE_TAG_CATEGORY_NAME")
	if err != nil {
		return err
	}

	dc, err := getDCByRegion(context.TODO(), c, regionTagName, regionTagCategoryName)
	if err != nil {
		return err
	}

	zonedDataStores, err := getDataStoresInDC(context.TODO(), c, dc, regionTagName, zoneTagCategoryName)
	if err != nil {
		return err
	}
	if len(zonedDataStores) == 0 {
		panic("no zonedDataStores returned")
	}

	zones, err := getZonesInDC(context.TODO(), c, dc, zoneTagCategoryName)
	if err != nil {
		return err
	}

	marshalledZonedDataStores, err := json.Marshal(Output{
		Datacenter:      dc.Name(),
		Zones:           zones,
		ZonedDataStores: zonedDataStores,
	})
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write(marshalledZonedDataStores)
	if err != nil {
		return err
	}
	return nil
}

func createVsphereClient() (vsphereClient, error) {
	host, err := utils.GetEnvOrDie("GOVC_URL")
	if err != nil {
		return vsphereClient{}, err
	}
	username, err := utils.GetEnvOrDie("GOVC_USERNAME")
	if err != nil {
		return vsphereClient{}, err
	}
	password, err := utils.GetEnvOrDie("GOVC_PASSWORD")
	if err != nil {
		return vsphereClient{}, err
	}
	insecureRaw, err := utils.GetEnvOrDie("GOVC_INSECURE")
	if err != nil {
		return vsphereClient{}, err
	}
	insecure, err := strconv.ParseBool(insecureRaw)
	if err != nil {
		return vsphereClient{}, fmt.Errorf("\"GOVC_INSECURE\" is not bool: %v", insecure)
	}

	parsedURL, err := url.Parse(fmt.Sprintf("https://%s:%s@%s/sdk", url.PathEscape(strings.TrimSpace(username)), url.PathEscape(strings.TrimSpace(password)), url.PathEscape(strings.TrimSpace(host))))
	if err != nil {
		return vsphereClient{}, err
	}

	vcClient, err := govmomi.NewClient(context.TODO(), parsedURL, insecure)
	if err != nil {
		return vsphereClient{}, err
	}

	if !vcClient.IsVC() {
		return vsphereClient{}, errors.New("not connected to vCenter")
	}

	restClient := rest.NewClient(vcClient.Client)
	user := url.UserPassword(username, password)
	if err := restClient.Login(context.TODO(), user); err != nil {
		return vsphereClient{}, err
	}

	return vsphereClient{
		client:     vcClient,
		restClient: restClient,
		host:       host,
		username:   username,
		password:   password,
		insecure:   insecure,
	}, nil
}

func getDCByRegion(ctx context.Context, client vsphereClient, regionTagName, regionTagCategoryName string) (*object.Datacenter, error) {
	var datacenter *object.Datacenter

	tagsClient := tags.NewManager(client.restClient)

	regionTag, err := tagsClient.GetTagForCategory(ctx, regionTagName, regionTagCategoryName)
	if err != nil {
		return nil, err
	}

	attachedObjects, err := tagsClient.ListAttachedObjects(ctx, regionTag.ID)

	var dcRefs []mo.Reference
	for _, ref := range attachedObjects {
		if ref.Reference().Type == "Datacenter" {
			dcRefs = append(dcRefs, ref)
		}
	}
	if len(dcRefs) != 1 {
		return nil, fmt.Errorf("only one DC should match \"region\" tag, insted matched: %v", dcRefs)
	}

	finder := find.NewFinder(client.client.Client)

	dcRef, err := finder.ObjectReference(ctx, dcRefs[0].Reference())
	if err != nil {
		return nil, err
	}

	datacenter = dcRef.(*object.Datacenter)

	return datacenter, nil
}

func getZonesInDC(ctx context.Context, client vsphereClient, datacenter *object.Datacenter, zoneTagCategoryName string) ([]string, error) {
	finder := find.NewFinder(client.client.Client, true)

	clusters, err := finder.ClusterComputeResourceList(ctx, path.Join(datacenter.InventoryPath, "..."))
	if err != nil {
		return nil, err
	}
	clusterReferences := make([]mo.Reference, len(clusters))
	for i := range clusters {
		clusterReferences[i] = clusters[i]
	}

	tagsClient := tags.NewManager(client.restClient)

	zoneTagCategory, err := tagsClient.GetCategory(ctx, zoneTagCategoryName)
	if err != nil {
		return nil, err
	}

	tagsInCategory, err := tagsClient.ListTagsForCategory(ctx, zoneTagCategory.ID)

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

	var matchingZones []string
	for zone := range matchingZonesMap {
		matchingZones = append(matchingZones, zone)
	}

	if len(matchingZones) == 0 {
		return nil, errors.New("no matching zones found")
	}

	return matchingZones, nil
}

func getDataStoresInDC(ctx context.Context, client vsphereClient, datacenter *object.Datacenter, regionTagName, zoneTagCategoryName string) ([]ZonedDataStore, error) {
	finder := find.NewFinder(client.client.Client, true)

	datastores, dsNotFoundErr := finder.DatastoreList(ctx, path.Join(datacenter.InventoryPath, "..."))

	datastoreClusters, dscNotFoundErr := finder.DatastoreClusterList(ctx, path.Join(datacenter.InventoryPath, "..."))

	if dsNotFoundErr != nil && dscNotFoundErr != nil {
		return nil, fmt.Errorf("not a single Datastore or DatastoreCluster found in the cluster:\n%s\n%s", dsNotFoundErr, dscNotFoundErr)
	}

	var datastoreReferences []mo.Reference
	for _, ds := range datastores {
		datastoreReferences = append(datastoreReferences, ds)
	}

	for _, dsc := range datastoreClusters {
		datastoreReferences = append(datastoreReferences, dsc)
	}

	tagsClient := tags.NewManager(client.restClient)

	zoneTagCategory, err := tagsClient.GetCategory(ctx, zoneTagCategoryName)
	if err != nil {
		return nil, err
	}

	datastoresWithTags, err := tagsClient.GetAttachedTagsOnObjects(ctx, datastoreReferences)
	if err != nil {
		return nil, err
	}

	var zds []ZonedDataStore
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
		)
		switch dsObject.(type) {
		case *object.Datastore:
			datastoreType = datastoreTypeDatastore
			inventoryPath = dsObject.(*object.Datastore).InventoryPath
		case *object.StoragePod:
			datastoreType = datastoreTypeDatastoreCluster
			inventoryPath = dsObject.(*object.StoragePod).InventoryPath
		default:
			return nil, fmt.Errorf("\"%s\" is not a Datastore nor a DatastoreCluster", reflect.TypeOf(dsObject))
		}

		zds = append(zds, ZonedDataStore{
			Zones:         dsZones,
			InventoryPath: inventoryPath,
			Name:          slugKubernetesName(strings.Join(strings.Split(inventoryPath, "/")[3:], "-")),
			DatastoreType: datastoreType,
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
