#!/usr/bin/python3

import typing

from dotmap import DotMap
from deckhouse import hook, utils

config = """
configVersion: v1
kubernetesCustomResourceConversion:
  - name: v1beta1_to_v1
    crdName: crontabs.stable.example.com
    conversions:
    - fromVersion: stable.example.com/v1beta1
      toVersion: stable.example.com/v1
  - name: v1_to_v1beta1
    crdName: crontabs.stable.example.com
    conversions:
    - fromVersion: stable.example.com/v1
      toVersion: stable.example.com/v1beta1
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
