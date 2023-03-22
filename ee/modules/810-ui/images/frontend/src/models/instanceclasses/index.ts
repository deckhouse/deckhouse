import AwsInstanceClass from "./AwsInstanceClass";
import AzureInstanceClass from "./AzureInstanceClass";
import GcpInstanceClass from "./GcpInstanceClass";
import OpenstackInstanceClass from "./OpenstackInstanceClass";
import VsphereInstanceClass from "./VsphereInstanceClass";
import YandexInstanceClass from "./YandexInstanceClass";

const Register = {
  aws: AwsInstanceClass,
  azure: AzureInstanceClass,
  openstack: OpenstackInstanceClass,
  gcp: GcpInstanceClass,
  vsphere: VsphereInstanceClass,
  yandex: YandexInstanceClass
} as const;

export default Register;

// TODO: somehow construct this from Register?
export type InstanceClassesTypes = AwsInstanceClass | AzureInstanceClass | GcpInstanceClass | OpenstackInstanceClass | VsphereInstanceClass | YandexInstanceClass;
