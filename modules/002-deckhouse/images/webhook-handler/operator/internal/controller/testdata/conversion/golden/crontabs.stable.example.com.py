#!/usr/bin/python3

import typing

from dotmap import DotMap
from deckhouse import hook, utils

config = """
configVersion: v1
kubernetesCustomResourceConversion:
  - name: v1alpha1_to_v1alpha2
    crdName: crontabs.stable.example.com
    - fromVersion: stable.example.com/v1alpha1
      toVersion: stable.example.com/v1alpha2
  - name: v1alpha2_to_v1alpha1
    crdName: crontabs.stable.example.com
    - fromVersion: stable.example.com/v1alpha2
      toVersion: stable.example.com/v1alpha1
"""

class Conversion(utils.BaseConversionHook):
    def __init__(self, ctx: hook.Context):
        super().__init__(ctx)
    def v1beta1_to_v1(self, o: dict) -> typing.Tuple[None, dict]:
        obj = DotMap(o)
    
        obj.apiVersion = "stable.example.com/v1"
        
        obj.spec.host=obj.spec.hostPort
        obj.spec.port=obj.spec.hostPort
        del obj.spec.hostPort
    
        return None, obj.toDict()
    
    def v1_to_v1beta1(self, o: dict) -> typing.Tuple[None, dict]:
        obj = DotMap(o)
    
        obj.apiVersion = "stable.example.com/v1beta1"
    
        hostPort = obj.spec.host+":"+obj.spec.port
        obj.spec.hostPort=hostPort
        del obj.spec.host
        del obj.spec.port
    
        return None, obj.toDict()

def main(ctx: hook.Context):
    Conversion(ctx).run()


if __name__ == "__main__":
    hook.run(main, config=config)
