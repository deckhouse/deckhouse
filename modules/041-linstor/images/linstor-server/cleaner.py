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
        # In pod hardcoded. TODO: we can use linstor.Config.get_section('global') instead
        self.__conn = linstor.Linstor("linstor+ssl://linstor.d8-linstor.svc:3371", timeout=5)
        self.__conn.keyfile = '/etc/linstor/client/tls.key'
        self.__conn.cafile = '/etc/linstor/client/ca.crt'
        self.__conn.certfile =  '/etc/linstor/client/tls.crt'

    def get_resource_list(self, retries=5):
        logger = logging.getLogger()
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
   
def process_res_files():
    logger = logging.getLogger()
    res_files = get_res_files()
    logging.debug(f"get {len(res_files)} *.res files in {BASE_RES_PATH}")

    lin_con = LinstorConnection()
    rest_reponse = lin_con.get_resource_list()
    if rest_reponse.__class__ is list and rest_reponse[0].__class__ is linstor.responses.ResourceResponse:
        rest_list = [r.name for r in rest_reponse[0].resources]
        logging.debug(f"recived {len(rest_list)} from linstor rest api")
    else:
        rest_list = []
        logging.debug("resource list from rest api is empty")

    res_to_remove = [res for res in res_files if res.stem not in rest_list]
    logging.info(f"founded {len(res_to_remove)} resources to remove")
    for f in res_to_remove:
        try:
            f.unlink()
        except:
            logging.debug(f"coudnot remove {f.stem} file")
            continue



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
