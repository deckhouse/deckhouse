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
import sched, time
import logging
from typing import List
from pathlib import Path

BASE_RES_PATH = "/var/lib/linstor.d/"
RES_FILES_MASK = "*.res"

logging.basicConfig(format = "%(asctime)s - %(name)s - %(levelname)s - %(message)s", level = logging.INFO)


def singleton(class_):
    instances = {}

    def get_instance(*args, **kwargs):
        if class_ not in instances:
            instances[class_] = class_(*args, **kwargs)
        return instances[class_]

    return get_instance

@singleton
class LinstorConnection:
    def __init__(self, log_level) -> None:
        # In pod hardcoded. TODO: we can use linstor.Config.get_section('global') instead
        self.__conn = linstor.Linstor("linstor+ssl://linstor.d8-linstor.svc:3371", timeout=5)
        self.__conn.keyfile = '/etc/linstor/client/tls.key'
        self.__conn.cafile = '/etc/linstor/client/ca.crt'
        self.__conn.certfile =  '/etc/linstor/client/tls.crt'
        self.__log_level = log_level

    def get_resource_list(self, retries=5):
        logger = logging.getLogger()
        logger.setLevel(self.__log_level)
        try:
            self.__conn.connect()
        except Exception:
            logger.error("error while connect linstor rest service linstor.d8-linstor.svc:3371")
            return None
        
        for i in range(retries):
            try:
                result = self.__conn.resource_list()
            except Exception:
                logger.debug(f"try retrive list resource from Linstor {i} time")
                continue
            else:
                break
        else:
            logger.error("couldnot request resource list from linstor")
            result = None
        self.__conn.disconnect()
        return result

def get_res_files() -> List[str]:
    files_iter = Path(BASE_RES_PATH).glob("RES_FILES_MASK")
    return [f.name for f in files_iter]
   
def process_res_files(log_level):
    logger = logging.getLogger()
    logger.setLevel(log_level)
    res_files = get_res_files()
    logger.debug(f"get {len(res_files)} *.res files in {BASE_RES_PATH}")

    lin_con = LinstorConnection(log_level)
    rest_reponse = lin_con.get_resource_list()
    if rest_reponse.__class__ is list and rest_reponse[0].__class__ is linstor.responses.ResourceResponse:
        rest_list = [r.name for r in rest_reponse[0].resources]
        logger.debug(f"recived {len(rest_list)} from linstor rest api")
    else:
        rest_list = []
        logger.debug("resource list from rest api is empty")

    res_to_remove = [res for res in res_files if res.stem not in rest_list]
    logger.info(f"founded {len(res_to_remove)} resources to remove")
    for f in res_to_remove:
        try:
            f.unlink()
        except:
            logger.debug(f"coudnot remove {f.stem} file")
            continue



def main(interval: int, debug: bool):
    log_level = logging.INFO
    if debug:
        log_level = logging.DEBUG
    scheduler = sched.scheduler(time.time, time.sleep)

    def run_periodicaly():
        logger = logging.getLogger()
        logger.debug("start new itteration")
        process_res_files(log_level)
        scheduler.enter(interval, 1, run_periodicaly)

    scheduler.enter(interval, 1, run_periodicaly)
    scheduler.run()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--interval", type=int, help="Interval in seconds", default=20, required=False)
    parser.add_argument("--debug", action="store_true", help="Enable debug logging", required=False)
    args = parser.parse_args()

    main(args.interval, args.debug)
