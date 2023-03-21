import AwsInstanceClass from "./AwsInstanceClass";
import OpenstackInstanceClass from "./OpenstackInstanceClass";

const Register = {
  aws: AwsInstanceClass,
  openstack: OpenstackInstanceClass,
} as const;

export default Register;

// TODO: somehow construct this from Register?
export type InstanceClassesTypes = AwsInstanceClass | OpenstackInstanceClass;
