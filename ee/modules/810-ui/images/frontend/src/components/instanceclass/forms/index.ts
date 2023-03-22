import InstanceClassAWSForm from "./AWSForm.vue";
import InstanceClassAzureForm from "./AzureForm.vue";
import InstanceClassGcpForm from "./GcpForm.vue";
import InstanceClassOpenstackForm from "./OpenstackForm.vue";
import InstanceClassVsphereForm from "./VsphereForm.vue";
import InstanceClassYandexForm from "./YandexForm.vue";

export default {
  aws: InstanceClassAWSForm,
  azure: InstanceClassAzureForm,
  gsp: InstanceClassGcpForm,
  openstack: InstanceClassOpenstackForm,
  vsphere: InstanceClassVsphereForm,
  yandex: InstanceClassYandexForm
};
