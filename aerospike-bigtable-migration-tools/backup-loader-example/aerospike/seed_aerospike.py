# Copyright 2025 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import json
import random
import aerospike
import logging

write_policy = {
    "key": aerospike.POLICY_KEY_SEND  # Ensure the user key is sent to the server
}

config = {
    "client_config": {"hosts": [("127.0.0.1", 3000)]},
    "data": {
        "test": {
            "carnivores": ["dog", "cat", "lion", "bear"],
            "herbivores": ["rabbit", "horse", "cow"],
        },
    },
}


def jsonify(input_dict):
    def convert_bytes(obj):
        if isinstance(obj, bytes):
            return obj.decode("utf-8")
        elif isinstance(obj, dict):
            return {key: convert_bytes(value) for key, value in obj.items()}
        elif isinstance(obj, list) or isinstance(obj, tuple):
            return [convert_bytes(item) for item in obj]
        else:
            return obj

    return convert_bytes(input_dict)


def get_key(index, prefix, max_index):
    """
    creates a zero-padded key with specified prefix
    """
    num_zeros = len(str(int(max_index)))
    return prefix + "-" + str(int(index)).rjust(num_zeros, "0")


def create_record(objects, nested_objects):
    record = {
        "integer": random.randint(0, 10000),
        "double": random.random(),
        "string": f"str-{random.random():.6f}-hello",
        "bool": random.random() > 0.5,
        "bytes": f"bytes-{random.random():.6f}-goodbye",
    }
    if objects:
        record["list_empty"] = []
        record["list_scalars"] = [1, 2, 3]
        record["list_mixed"] = [1, 2.2, "3", True, "test".encode()]
        record["map_empty"] = {}
        record["map_scalars"] = {1: "a", 2: "b", 3: "c"}
        record["map_mixed"] = {1: "a", "a": 1, True: "test".encode()}

    if nested_objects:
        record["list_nested"] = [[], [1], [1, [], [1]]]
        record["map_nested"] = {1: {}, 2: {1: "a"}, 3: {1: {1: "b"}, 2: {}}}
        record["mixed_nested"] = {
            1: [],
            2: {1: [], 2: {3: "a"}},
            3: [{1: "a"}, {2: "b"}],
        }

    return record


def seed(client, data, records_per_key):
    for namespace, ns_data in data.items():
        logging.info(f"SEEDING {namespace}")
        for set_name, set_data in ns_data.items():
            logging.info(f"\t SEEDING {set_name}")
            for key_prefix in set_data:
                for i in range(records_per_key):
                    key = get_key(i, key_prefix, records_per_key)
                    record = create_record(False, False)
                    log_entry = json.dumps(
                        {namespace: {set_name: {key: jsonify(record)}}}
                    )
                    logging.debug(log_entry)
                    client.put((namespace, set_name, key), record, policy=write_policy)
    logging.info("DONE SEEDING")


# Setup logging to both file and console
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(message)s",
    handlers=[logging.FileHandler("seed.log", mode="w"), logging.StreamHandler()],
)

client = aerospike.client(config["client_config"]).connect()
seed(client, config["data"], records_per_key=1)
client.close()
