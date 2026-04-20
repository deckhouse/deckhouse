#!/usr/bin/python3

import typing

from dotmap import DotMap
from deckhouse import hook, utils

config = """
configVersion: v1
kubernetesCustomResourceConversion:
  - name: v1alpha1_to_v1
    crdName: example.deckhouse.io
    conversions:
    - fromVersion: deckhouse.io/v1alpha1
      toVersion: deckhouse.io/v1
  - name: v1_to_v1alpha1
    crdName: example.deckhouse.io
    conversions:
    - fromVersion: deckhouse.io/v1
      toVersion: deckhouse.io/v1alpha1
"""

class Conversion(utils.BaseConversionHook):
    def __init__(self, ctx: hook.Context):
        super().__init__(ctx)
    def v1alpha1_to_v1(self, o: dict) -> typing.Tuple[None, dict]:
        obj = DotMap(o)
    
        obj.apiVersion = "deckhouse.io/v1"
        
        obj.spec.host=obj.spec.hostPort
        obj.spec.port=obj.spec.hostPort
        del obj.spec.hostPort
    
        return None, obj.toDict()
    
    def v1_to_v1alpha1(self, o: dict) -> typing.Tuple[None, dict]:
        obj = DotMap(o)
    
        obj.apiVersion = "deckhouse.io/v1alpha1"
        if not obj.spec.host:
          return None, obj.toDict()
    
        hostPort = obj.spec.host+":"+obj.spec.port
        del obj.spec
        if hostPort:
          obj.spec.hostPort=hostPort
    
        return None, obj.toDict()

def main(ctx: hook.Context):
    Conversion(ctx).run()


if __name__ == "__main__":
    hook.run(main, config=config)
