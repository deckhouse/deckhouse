# Copyright 2023 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import sys
import linstor
import argparse
import pathlib
import sched, time
import logging

def singleton(class_):
    instances = {}

    def get_instance(*args, **kwargs):
        if class_ not in instances:
            instances[class_] = class_(*args, **kwargs)
        return instances[class_]

    return get_instance

@singleton
class LinstorConnection:
    def __init__(self) -> None:
        # In pod hardcoded.
        self.__conn = linstor.Linstor("linstor+ssl://linstor.d8-linstor.svc:3371")
        self.__conn.keyfile = '/etc/linstor/client/tls.key'
        self.__conn.cafile = '/etc/linstor/client/ca.crt'
        self.__conn.certfile =  '/etc/linstor/client/tls.crt'

    def get_resource_list(self, retries=5):
        logger = logging.getLogger()
        self.__conn.connect()
        for i in range(retries):
            try:
                result = self.__conn.resource_list()
            except Exception:
                logger.debug(f"try retrive list resource from Linstor {i} time")
                continue
            else:
                break
        else:
            logger.error("couldnor request resource list from linstor")
            result = None
        self.__conn.disconnect()
        return result


def process_res_files():
    pass

def main(interval: int, debug: bool):
    log_level = logging.INFO
    if debug:
        log_level = logging.DEBUG
    logging.basicConfig(stream=sys.stdout, level=log_level)

    scheduler = sched.scheduler(time.time, time.sleep)
    scheduler.enter(interval, 1, process_res_files)
    scheduler.run()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--interval", type=int, help="Interval in seconds", default=20, required=False)
    parser.add_argument("--debug", type=bool, action="store_true", help="Enable debug logging", required=False)
    args = parser.parse_args()

    main(args.interval, args.debug)
