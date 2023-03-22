import InstanceClassAWSParams from "./AWSParams.vue";
import InstanceClassAzureParams from "./AzureParams.vue";
import InstanceClassGcpParams from "./GcpParams.vue";
import InstanceClassOpenstackParams from "./OpenstackParams.vue";
import InstanceClassVsphereParams from "./VsphereParams.vue";
import InstanceClassYandexParams from "./YandexParams.vue";

export default {
  aws: InstanceClassAWSParams,
  azure: InstanceClassAzureParams,
  gcp: InstanceClassGcpParams,
  openstack: InstanceClassOpenstackParams,
  vsphere: InstanceClassVsphereParams,
  yandex: InstanceClassYandexParams
};
